// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	"github.com/linuxdeepin/dde-daemon/securityloader"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"golang.org/x/sys/unix"
)

// V4L2 ioctl constants derived from <linux/videodev2.h> /
// <linux/v4l2-controls.h>. Verified empirically against the kernel UAPI.
const (
	// V4L2_CID_CAMERA_PRIVACY = V4L2_CID_CAMERA_CLASS_BASE(0x9a0900) + 16
	v4l2CtrlIDPrivacy = 0x9a0910
	// VIDIOC_G_CTRL      _IOWR('V', 27, struct v4l2_control)
	vidIocGCTRL = 0xc008561b
	// VIDIOC_S_CTRL      _IOWR('V', 28, struct v4l2_control)
	vidIocSCTRL = 0xc008561c
	// VIDIOC_QUERYCTRL   _IOWR('V', 36, struct v4l2_queryctrl)
	vidIocQueryCtrl = 0xc0445624
)

// struct v4l2_control { __u32 id; __s32 value; }
type v4l2Ctrl struct {
	ID    uint32
	Value int32
}

// struct v4l2_queryctrl (fields needed for probing):
// { __u32 id; __u32 type; char name[32]; __s32 min/max/step/default_value;
//
//	__u32 flags; __u32 reserved[2]; }
type v4l2QueryCtrl struct {
	ID       uint32
	CtrlType uint32
	Name     [32]uint8
	Minimum  int32
	Maximum  int32
	Step     int32
	Default  int32
	Flags    uint32
	Reserved [2]uint32
}

// sysfs paths, overridable in tests.
var (
	usbDevicesRoot = "/sys/bus/usb/devices"
	v4lClassPath   = "/sys/class/video4linux"
	uvcDriverPath  = "/sys/bus/usb/drivers/uvcvideo"
	// sysfsRoot is the mount point DEVPATH-based uevent prechecks resolve
	// against (kernel DEVPATH values are relative to it). Overridable in tests.
	sysfsRoot = "/sys"
)

const usbIfaceClassVID = "0e"

// dconfig key persisting the camera privacy switch so it survives daemon
// restart and is reapplied on hotplug.
const dsKeyCameraPrivacyEnabled = "cameraPrivacyEnabled"

var (
	cameraDConfig   *dconfig.DConfig
	cameraPrivacyMu sync.Mutex
	cameraOpMu      sync.Mutex
	cameraPrivacyOn bool
)

// 说明:本模块是相机禁用快捷键的底层实现,负责识别摄像头并通过硬件
// 手段开关它。
//
// 识别信号来自内核 uevent(原生 netlink 监听):设备插入后,内核先广播
// usb_device add 事件(此时接口信息可能还没生成,只作兜底);uvcvideo
// 驱动完成 probe、注册 /dev/videoN 节点后,再广播 video4linux add 事件
// (此时设备一定可读,是可靠的识别信号)。
//
// 收到事件后按优先级尝试两种关闭方式:
//  1. 优先用摄像头自带的 V4L2_CID_CAMERA_PRIVACY 控制(部分摄像头
//     硬件支持的物理遮挡开关),通过 ioctl 设置;
//  2. 不支持该控制的,按 USB 接口类 0e(视频类)找出摄像头接口,写
//     uvcvideo 驱动的 unbind 关闭、bind 恢复。同设备的音频接口类不是
//     0e,不受影响,内置麦克风可正常使用。
//
// 采集卡、U 盘等设备没有 0e 接口,不会被误操作。切换硬件状态和保存
// 配置都在同一把锁内完成,热插拔触发的重新关闭会先重新读取开关状态,
// 不会把用户刚切换的状态覆盖回去。
//
// initCameraPrivacy loads the persisted switch state, re-applies it to any
// camera currently present, and starts a uevent watcher so that a camera
// plugged in later is switched off again while the switch is on. It must
// be called once after the system daemon's dconfig is ready.
func initCameraPrivacy() {
	dc, err := dconfig.NewDConfig(dsettingsSystemDaemonID, dsettingsSystemDaemonName, "")
	if err != nil {
		logger.Warning("initCameraPrivacy: new dconfig failed:", err)
		return
	}
	cameraDConfig = dc

	on, err := dc.GetValueBool(dsKeyCameraPrivacyEnabled)
	if err != nil {
		logger.Warning("initCameraPrivacy: read dconfig failed:", err)
	}
	cameraPrivacyMu.Lock()
	cameraPrivacyOn = on
	cameraPrivacyMu.Unlock()

	// Re-apply the persisted state to cameras present at startup.
	if on {
		if _, err := setCameraPrivacy(true); err != nil {
			logger.Warning("initCameraPrivacy: re-apply failed:", err)
		}
	}

	// Watch for camera hotplug with a raw netlink uevent monitor. gudev
	// cannot be used here: its "uevent" signal is dispatched by the GLib
	// main loop, which dde-system-daemon does not run (verified on a real
	// machine: the netlink socket fills up and is never read). Subscribing
	// to the kernel broadcast group directly keeps the same event source
	// without a GLib dependency. A video4linux "add" uevent is emitted by
	// video_register_device() after /dev/videoN is created, so both privacy
	// tiers see a fully populated device; the usb_device "add" fallback
	// covers cameras that bypass the video4linux event path.
	if err := startCameraUeventMonitor(); err != nil {
		logger.Warning("initCameraPrivacy: start uevent monitor failed:", err)
		return
	}
	logger.Info("initCameraPrivacy: uevent monitor started, privacy on=", on)
}

// startCameraUeventMonitor joins the kernel uevent broadcast group and reads
// it on a dedicated goroutine. The socket is non-blocking friendly via
// blocking reads: the goroutine lives for the process lifetime.
func startCameraUeventMonitor() error {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_KOBJECT_UEVENT)
	if err != nil {
		return fmt.Errorf("socket: %w", err)
	}
	addr := &unix.SockaddrNetlink{Family: unix.AF_NETLINK, Groups: 1}
	if err := unix.Bind(fd, addr); err != nil {
		unix.Close(fd)
		return fmt.Errorf("bind: %w", err)
	}
	go cameraUeventLoop(fd)
	return nil
}

// cameraUeventLoop reads raw uevent messages until the process exits. The
// kernel payload is "ACTION@DEVPATH\0KEY=VALUE\0KEY=VALUE\0..."; parse
// ACTION, SUBSYSTEM and DEVTYPE from it and re-apply privacy when a camera
// became usable.
func cameraUeventLoop(fd int) {
	defer unix.Close(fd)
	buf := make([]byte, 8192)
	for {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			logger.Warningf("cameraUeventLoop: recvfrom failed: %v", err)
			return
		}
		msg := parseUevent(buf[:n])
		if !shouldReapplyCamera(msg.action, msg.subsystem, msg.devtype, msg.devpath) {
			logger.Debugf("handleCameraUEvent: ignore uevent action=%q subsystem=%q devtype=%q",
				msg.action, msg.subsystem, msg.devtype)
			continue
		}
		logger.Infof("handleCameraUEvent: camera event action=%q subsystem=%q devtype=%q, re-apply privacy",
			msg.action, msg.subsystem, msg.devtype)
		reapplyCameraPrivacy()
	}
}

// ueventMsg holds the fields the filter needs from a raw uevent payload.
type ueventMsg struct {
	action    string
	subsystem string
	devtype   string
	devpath   string
}

// parseUevent extracts ACTION and DEVPATH (from the "ACTION@DEVPATH" header)
// and the SUBSYSTEM/DEVTYPE environment keys of a netlink uevent message.
// Keys are NUL-separated, per kobject_uevent_env().
func parseUevent(payload []byte) ueventMsg {
	var msg ueventMsg
	fields := strings.Split(string(payload), "\x00")
	if len(fields) == 0 {
		return msg
	}
	header := fields[0]
	if idx := strings.IndexByte(header, '@'); idx >= 0 {
		msg.action = header[:idx]
		msg.devpath = header[idx+1:]
	}
	for _, field := range fields[1:] {
		switch {
		case strings.HasPrefix(field, "SUBSYSTEM="):
			msg.subsystem = strings.TrimPrefix(field, "SUBSYSTEM=")
		case strings.HasPrefix(field, "DEVTYPE="):
			msg.devtype = strings.TrimPrefix(field, "DEVTYPE=")
		}
	}
	return msg
}

// usbDeviceHasVideoInterface reports whether the sysfs device directory at
// devpath (a usb_device DEVPATH, e.g. /devices/.../usb1/1-4) contains an
// interface with bInterfaceClass 0e (video). ok is false when the directory
// or the interface attributes cannot be read — the caller must treat that
// as "unknown", not "no", because at usb_device add time the interfaces may
// not be created yet.
func usbDeviceHasVideoInterface(devpath string) (found bool, ok bool) {
	base := filepath.Join(sysfsRoot, devpath)
	entries, err := os.ReadDir(base)
	if err != nil {
		return false, false
	}
	sawInterface := false
	for _, e := range entries {
		if !strings.Contains(e.Name(), ":") {
			continue
		}
		sawInterface = true
		data, err := os.ReadFile(filepath.Join(base, e.Name(), "bInterfaceClass"))
		if err != nil {
			return false, false
		}
		if strings.TrimSpace(string(data)) == usbIfaceClassVID {
			return true, true
		}
	}
	return false, sawInterface
}

// shouldReapplyCamera reports whether a uevent means a camera may have
// become usable. The reliable signal is a video4linux "add" uevent (emitted
// by video_register_device() after the /dev/videoN node exists). usb_device
// "add" is kept as a fallback: it arrives before the driver probe, so a
// DEVPATH precheck filters out non-camera devices (flash drives, mice)
// while keeping the reapply when the interfaces are not readable yet — the
// precheck must never turn "unknown" into "no", that would break the
// fallback for cameras whose interfaces are not populated at event time.
func shouldReapplyCamera(action, subsystem, devtype, devpath string) bool {
	if action != "add" {
		return false
	}
	if subsystem == "video4linux" {
		return true
	}
	if subsystem == "usb" && devtype == "usb_device" {
		found, ok := usbDeviceHasVideoInterface(devpath)
		return !ok || found
	}
	return false
}

// reapplyCameraPrivacy re-disables the camera when the privacy switch is on.
// Called on hotplug and at startup. Idempotent: setCameraPrivacy(true)
// unbinds every UVC video interface, so already-disabled cameras stay off.
// The flag is re-checked inside the operation mutex: a uevent may arrive
// while a concurrent SetCameraPrivacy(false) is waiting for the mutex, and
// applying without re-checking would re-enable the camera the caller just
// disabled.
func reapplyCameraPrivacy() {
	cameraOpMu.Lock()
	defer cameraOpMu.Unlock()
	cameraPrivacyMu.Lock()
	on := cameraPrivacyOn
	cameraPrivacyMu.Unlock()
	if !on {
		logger.Debug("reapplyCameraPrivacy: privacy off, skip")
		return
	}
	logger.Info("reapplyCameraPrivacy: re-applying privacy on")
	if _, err := setCameraPrivacy(true); err != nil {
		logger.Warning("reapplyCameraPrivacy: re-apply failed:", err)
	}
}

// setCameraPrivacyState persists and stores the privacy switch. Called by
// SetCameraPrivacy after applying the hardware state.
func setCameraPrivacyState(on bool) {
	cameraPrivacyMu.Lock()
	cameraPrivacyOn = on
	cameraPrivacyMu.Unlock()
	if cameraDConfig != nil {
		if err := cameraDConfig.SetValue(dsKeyCameraPrivacyEnabled, on); err != nil {
			logger.Warning("setCameraPrivacyState: persist failed:", err)
		}
	}
}

// setCameraPrivacyOp runs the hardware switch and the persist step under one
// operation mutex. Concurrent D-Bus calls and hotplug reapplications must not
// interleave bind/unbind writes or persist out of order: the mutex guarantees
// the hardware state and the persisted flag come from the last requested
// state, not from whichever caller finished last.
func setCameraPrivacyOp(state bool) (bool, error) {
	cameraOpMu.Lock()
	defer cameraOpMu.Unlock()
	applied, err := setCameraPrivacy(state)
	if err != nil {
		logger.Warningf("setCameraPrivacyOp: set to %v failed: %v", state, err)
		return applied, err
	}
	setCameraPrivacyState(state)
	logger.Infof("setCameraPrivacyOp: set to %v, applied=%v", state, applied)
	return applied, nil
}

// setCameraPrivacy applies the requested privacy state with a tiered strategy:
//  1. Standard V4L2 camera privacy control (V4L2_CID_CAMERA_PRIVACY) — the
//     kernel/hardware privacy capability, no device list needed.
//  2. uvcvideo driver unbind/bind at the video-interface level — removes
//     /dev/video* while privacy is on without touching other interfaces on
//     the same USB device, so a built-in mic (bound to snd-usb-audio) keeps
//     working.
func setCameraPrivacy(state bool) (bool, error) {
	applied := false

	devs, err := listVideoDevices()
	if err != nil {
		logger.Warningf("setCameraPrivacy: list video devices failed: %v", err)
	}
	if len(devs) == 0 {
		logger.Debug("setCameraPrivacy: no V4L2 devices found")
	}
	for _, dev := range devs {
		ok, err := setV4L2Privacy(dev, state)
		if err != nil {
			logger.Warningf("setCameraPrivacy: %s: %v", dev, err)
		}
		if ok {
			applied = true
		}
	}
	if applied {
		return true, nil
	}

	ok, err := setUVCVideoUnbound(state)
	if err != nil {
		logger.Warningf("setCameraPrivacy: uvcvideo unbind switch failed: %v", err)
		return false, err
	}
	return ok, nil
}

// listVideoDevices returns /dev/videoN paths backed by a UVC camera interface.
func listVideoDevices() ([]string, error) {
	entries, err := os.ReadDir(v4lClassPath)
	if err != nil {
		return nil, err
	}
	var devs []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "video") {
			continue
		}
		devLink, err := os.Readlink(filepath.Join(v4lClassPath, name, "device"))
		if err != nil {
			continue
		}
		if ifaceClassOfLink(devLink) != usbIfaceClassVID {
			continue
		}
		devs = append(devs, "/dev/"+name)
	}
	return devs, nil
}

// ifaceClassOfLink resolves a video4linux device symlink (e.g. "../../../1-7:1.0")
// to its USB interface bInterfaceClass, "" when undeterminable.
func ifaceClassOfLink(devLink string) string {
	base := filepath.Base(devLink)
	if !strings.Contains(base, ":") {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(usbDevicesRoot, base, "bInterfaceClass"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// setV4L2Privacy sets the camera privacy control on dev; ok is false when the
// device does not expose V4L2_CID_CAMERA_PRIVACY.
func setV4L2Privacy(dev string, state bool) (ok bool, err error) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer f.Close()

	if !v4l2HasControl(f.Fd(), v4l2CtrlIDPrivacy) {
		return false, nil
	}

	val := int32(0)
	if state {
		val = 1
	}
	if err := v4l2SetControl(f.Fd(), v4l2CtrlIDPrivacy, val); err != nil {
		return false, err
	}
	logger.Infof("setV4L2Privacy: privacy=%d on %s", val, dev)
	return true, nil
}

func v4l2HasControl(fd uintptr, ctrlID uint32) bool {
	qc := v4l2QueryCtrl{ID: ctrlID}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, vidIocQueryCtrl,
		uintptr(unsafe.Pointer(&qc)))
	return errno == 0
}

func v4l2SetControl(fd uintptr, ctrlID uint32, value int32) error {
	ctrl := v4l2Ctrl{ID: ctrlID, Value: value}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, vidIocSCTRL,
		uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return errno
	}
	return nil
}

// setUVCVideoUnbound disables (privacy=true) or re-enables (privacy=false) the
// camera by unbinding/binding the uvcvideo driver on every UVC video interface
// (bInterfaceClass 0e). Because the operation is limited to the video
// interface, a built-in mic on an audio interface of the same USB device stays
// bound to snd-usb-audio and keeps working. Returns applied=false when no UVC
// video interface is found.
func setUVCVideoUnbound(privacy bool) (bool, error) {
	ifaces, err := listUVCVideoInterfaces()
	if err != nil {
		return false, err
	}
	if len(ifaces) == 0 {
		return false, nil
	}

	for _, iface := range ifaces {
		var op string
		if privacy {
			op = "unbind"
		} else {
			op = "bind"
		}
		// Idempotence: an already-unbound interface must not be unbound
		// again (the kernel returns -ENODEV), and a bound one not re-bound.
		if op == "unbind" && !interfaceHasDriver(filepath.Join(usbDevicesRoot, iface)) {
			continue
		}
		if op == "bind" && interfaceHasDriver(filepath.Join(usbDevicesRoot, iface)) {
			continue
		}
		if err := writeDriverAttr(op, iface); err != nil {
			// ENODEV is expected when unbinding a metadata interface (the
			// second 0e-class interface) while the main video interface of
			// the same camera tears the driver probe down concurrently:
			// the driver symlink lags the kernel state for a moment. That
			// interface is already effectively unbound, so demote to Debug.
			if errors.Is(err, unix.ENODEV) || errors.Is(err, unix.ENOENT) {
				logger.Debugf("setUVCVideoUnbound: %s %s: %v (already gone)", op, iface, err)
			} else {
				logger.Warningf("setUVCVideoUnbound: %s %s: %v", op, iface, err)
			}
			continue
		}
		logger.Infof("setUVCVideoUnbound: %s %s", op, iface)
	}
	return true, nil
}

// listUVCVideoInterfaces returns USB interface IDs (e.g. "1-7:1.0") whose
// bInterfaceClass is 0e (video).
func listUVCVideoInterfaces() ([]string, error) {
	entries, err := os.ReadDir(usbDevicesRoot)
	if err != nil {
		return nil, err
	}
	var ifaces []string
	for _, e := range entries {
		name := e.Name()
		if !strings.Contains(name, ":") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(usbDevicesRoot, name, "bInterfaceClass"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == usbIfaceClassVID {
			ifaces = append(ifaces, name)
		}
	}
	sort.Strings(ifaces)
	return ifaces, nil
}

// writeDriverAttr writes iface (e.g. "1-7:1.0") to the uvcvideo driver's
// bind/unbind attribute.
var writeDriverAttr = func(op, iface string) error {
	return os.WriteFile(filepath.Join(uvcDriverPath, op), []byte(iface), 0644)
}

// cameraPrivacy reports the current privacy state using the same tier order as
// setCameraPrivacy: the V4L2 privacy control when available, otherwise the
// uvcvideo driver binding state. known is false when no camera device can be
// queried, letting the caller keep its own state.
func cameraPrivacy() (privacy bool, known bool) {
	devs, err := listVideoDevices()
	if err != nil {
		logger.Warningf("cameraPrivacy: list video devices failed: %v", err)
	}
	for _, dev := range devs {
		on, ok := v4l2Privacy(dev)
		if ok {
			return on, true
		}
	}

	ifaces, err := listUVCVideoInterfaces()
	if err != nil {
		logger.Warningf("cameraPrivacy: list uvc video interfaces failed: %v", err)
		return false, false
	}
	for _, iface := range ifaces {
		// When the uvcvideo driver is rebound the interface points at it and
		// /dev/video* is back; when unbound the driver link is gone. An
		// unbound video interface means privacy (camera off) is on.
		return !interfaceHasDriver(filepath.Join(usbDevicesRoot, iface)), true
	}
	return false, false
}

// v4l2Privacy reads the camera privacy control on dev; ok is false when the
// device does not expose V4L2_CID_CAMERA_PRIVACY.
func v4l2Privacy(dev string) (privacy bool, ok bool) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		return false, false
	}
	defer f.Close()

	if !v4l2HasControl(f.Fd(), v4l2CtrlIDPrivacy) {
		return false, false
	}
	val, err := v4l2GetControl(f.Fd(), v4l2CtrlIDPrivacy)
	if err != nil {
		logger.Warningf("v4l2Privacy: %s: %v", dev, err)
		return false, false
	}
	return val != 0, true
}

func v4l2GetControl(fd uintptr, ctrlID uint32) (int32, error) {
	ctrl := v4l2Ctrl{ID: ctrlID}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, vidIocGCTRL,
		uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return 0, errno
	}
	return ctrl.Value, nil
}

// interfaceHasDriver reports whether the USB interface still has a driver
// bound (its `driver` symlink resolves), i.e. it is not unbound.
func interfaceHasDriver(ifaceDir string) bool {
	_, err := os.Readlink(filepath.Join(ifaceDir, "driver"))
	return err == nil
}

// authorize checks the caller via the security loader registry with a polkit
// fallback for the given action id.
func (d *Daemon) authorize(sender dbus.Sender, actionID string) error {
	return securityloader.AuthorizeWithPolkit(
		d.allowCallers,
		securityloader.DaemonScope,
		sender,
		d.service.Conn(),
		actionID,
	)
}

// SetCameraPrivacy is the exported D-Bus method: authorizes the caller,
// applies the tiered privacy switch and reports whether hardware switching
// happened so the caller can fall back to app-level handling.
func (d *Daemon) SetCameraPrivacy(sender dbus.Sender, state bool) (applied bool, busErr *dbus.Error) {
	if err := d.authorize(sender, "org.deepin.dde.daemon.set-camera-privacy"); err != nil {
		logger.Warningf("SetCameraPrivacy authorization failed: %q", err.Error())
		return false, dbusutil.ToError(err)
	}

	logger.Infof("SetCameraPrivacy set state: %v", state)
	applied, err := setCameraPrivacyOp(state)
	if err != nil {
		logger.Warningf("SetCameraPrivacy failed: %v", err)
		return false, dbusutil.ToError(err)
	}
	return applied, nil
}

// GetCameraPrivacy is the exported D-Bus method reporting the current camera
// privacy state. known is false when no camera device can be queried, so the
// caller can fall back to its own tracking. Reading the state needs no
// privileges, so no authorization is required.
func (d *Daemon) GetCameraPrivacy() (privacy bool, known bool, busErr *dbus.Error) {
	privacy, known = cameraPrivacy()
	logger.Infof("GetCameraPrivacy privacy=%v known=%v", privacy, known)
	return privacy, known, nil
}
