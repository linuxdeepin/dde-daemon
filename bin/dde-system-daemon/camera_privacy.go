// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	gudev "github.com/linuxdeepin/go-gir/gudev-1.0"
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
)

const usbIfaceClassVID = "0e"

// dconfig key persisting the camera privacy switch so it survives daemon
// restart and is reapplied on hotplug.
const dsKeyCameraPrivacyEnabled = "cameraPrivacyEnabled"

var (
	cameraDConfig   *dconfig.DConfig
	cameraPrivacyMu sync.Mutex
	cameraPrivacyOn bool
	cameraWatcher   *gudev.Client
)

// initCameraPrivacy loads the persisted privacy state, re-applies it to any
// camera currently present, and starts a udev watcher so that a camera
// plugged in later is disabled again while privacy is on. It must be called
// once after the system daemon's dconfig is ready.
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

	// Watch USB hotplug: when a camera shows up while privacy is on, unbind it.
	cameraWatcher = gudev.NewClient([]string{"usb"})
	if cameraWatcher == nil {
		logger.Warning("initCameraPrivacy: gudev client nil, hotplug watch disabled")
		return
	}
	cameraWatcher.Connect("uevent", handleCameraUEvent)
	logger.Info("initCameraPrivacy: watcher started, privacy on=", on)
}

// handleCameraUEvent re-applies the privacy state when a USB device is added.
// It is intentionally idempotent: setCameraPrivacy(true) unbinds every UVC
// video interface, so a freshly plugged camera is disabled while cameras
// already off stay off.
func handleCameraUEvent(client *gudev.Client, action string, device *gudev.Device) {
	defer device.Unref()
	if action != "add" {
		return
	}
	if device.GetDevtype() != "usb_device" {
		return
	}
	reapplyCameraPrivacy()
}

// reapplyCameraPrivacy re-disables the camera when the privacy switch is on.
// Called on hotplug and at startup. Idempotent: setCameraPrivacy(true)
// unbinds every UVC video interface, so already-disabled cameras stay off.
func reapplyCameraPrivacy() {
	cameraPrivacyMu.Lock()
	on := cameraPrivacyOn
	cameraPrivacyMu.Unlock()
	if !on {
		return
	}
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
		if err := writeDriverAttr(op, iface); err != nil {
			logger.Warningf("setUVCVideoUnbound: %s %s: %v", op, iface, err)
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

// SetCameraPrivacy is the exported D-Bus method: authorizes the caller,
// applies the tiered privacy switch and reports whether hardware switching
// happened so the caller can fall back to app-level handling.
func (d *Daemon) SetCameraPrivacy(sender dbus.Sender, state bool) (applied bool, busErr *dbus.Error) {
	if err := checkAuth("org.deepin.dde.daemon.set-camera-privacy", string(sender)); err != nil {
		logger.Warningf("SetCameraPrivacy authorization failed: %q", err.Error())
		return false, dbusutil.ToError(err)
	}

	logger.Infof("SetCameraPrivacy set state: %v", state)
	applied, err := setCameraPrivacy(state)
	if err != nil {
		logger.Warningf("SetCameraPrivacy failed: %v", err)
		return false, dbusutil.ToError(err)
	}
	// Persist the switch so it survives restart and is reapplied on hotplug.
	setCameraPrivacyState(state)
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
