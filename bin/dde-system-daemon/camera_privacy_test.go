// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// newCameraTestEnv builds an isolated sysfs tree for camera privacy tests.
func newCameraTestEnv(t *testing.T) (usbRoot, v4lDir string) {
	t.Helper()
	tmpDir := t.TempDir()
	usbRoot = filepath.Join(tmpDir, "usb")
	v4lDir = filepath.Join(tmpDir, "v4l")
	for _, d := range []string{usbRoot, v4lDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return usbRoot, v4lDir
}

// restoreSysfsPaths restores the global sysfs path vars after each test.
func restoreSysfsPaths(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		usbDevicesRoot = "/sys/bus/usb/devices"
		v4lClassPath = "/sys/class/video4linux"
		uvcDriverPath = "/sys/bus/usb/drivers/uvcvideo"
	})
}

// addUSBDevice creates a USB device and its interface entries as siblings
// directly under usbRoot, mirroring the real sysfs layout
// (/sys/bus/usb/devices/1-7 and /sys/bus/usb/devices/1-7:1.0). Each interface
// gets a bInterfaceClass; a "0e" (video) interface is bound to uvcvideo by
// symlink so interfaceHasDriver() reports true until an unbind removes it.
func addUSBDevice(t *testing.T, usbRoot, dev string, interfaces map[string]string) {
	t.Helper()
	devDir := filepath.Join(usbRoot, dev)
	if err := os.MkdirAll(devDir, 0755); err != nil {
		t.Fatal(err)
	}
	for iface, class := range interfaces {
		addUVCInterface(t, usbRoot, iface, class)
	}
}

// addUVCInterface creates a USB interface entry (e.g. "1-7:1.0") directly
// under usbRoot with the given interface class.
func addUVCInterface(t *testing.T, usbRoot, iface, class string) {
	t.Helper()
	ifaceDir := filepath.Join(usbRoot, iface)
	if err := os.MkdirAll(ifaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ifaceDir, "bInterfaceClass"), []byte(class+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if class == "0e" {
		// Bound to uvcvideo by default.
		if err := os.Symlink(filepath.Join("..", "drivers", "uvcvideo"),
			filepath.Join(ifaceDir, "driver")); err != nil {
			t.Fatal(err)
		}
	}
}

// unbindUVCInterface simulates a successful uvcvideo unbind by removing the
// driver symlink on the interface dir.
func unbindUVCInterface(t *testing.T, usbRoot, iface string) {
	t.Helper()
	if err := os.Remove(filepath.Join(usbRoot, iface, "driver")); err != nil {
		t.Fatal(err)
	}
}

func TestListUVCVideoInterfacesFiltersByVideoClass(t *testing.T) {
	usbRoot, _ := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot

	// Video interface 1-7:1.0 + audio 1-7:1.1.
	addUSBDevice(t, usbRoot, "1-7", map[string]string{"1-7:1.0": "0e", "1-7:1.1": "01"})
	// Video only 2-4:1.0.
	addUSBDevice(t, usbRoot, "2-4", map[string]string{"2-4:1.0": "0e"})
	// Audio only 3-9:1.0, must be excluded.
	addUSBDevice(t, usbRoot, "3-9", map[string]string{"3-9:1.0": "01"})
	// Device dir "usb1" (hub) and non-interface entries must be excluded.
	hubDir := filepath.Join(usbRoot, "usb1")
	if err := os.MkdirAll(hubDir, 0755); err != nil {
		t.Fatal(err)
	}
	// A video entry under a device dir but without ":" prefix must be ignored.
	dev4Dir := filepath.Join(usbRoot, "4-1")
	if err := os.MkdirAll(dev4Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dev4Dir, "bInterfaceClass"), []byte("0e\n"), 0644); err != nil {
		t.Fatal(err)
	}

	devs, err := listUVCVideoInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"1-7:1.0", "2-4:1.0"}
	if !reflect.DeepEqual(devs, expected) {
		t.Fatalf("expected %v, got %v", expected, devs)
	}
}

func TestSetUVCVideoUnboundTogglesVideoInterfaceOnly(t *testing.T) {
	usbRoot, _ := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	uvcDriverPath = filepath.Join(usbRoot, "drivers", "uvcvideo")
	if err := os.MkdirAll(uvcDriverPath, 0755); err != nil {
		t.Fatal(err)
	}

	addUSBDevice(t, usbRoot, "1-7", map[string]string{"1-7:1.0": "0e", "1-7:1.1": "01"})

	// Spy on writeDriverAttr, simulating the kernel driver side effects:
	// on "bind" the driver symlink is (re)created on the interface dir, so
	// the skip-already-bound guard sees the interface as bound afterwards.
	var calls []string
	writeDriverAttr = func(op, iface string) error {
		calls = append(calls, op+":"+iface)
		// Kernel side effects: "unbind" removes the driver symlink,
		// "bind" recreates it.
		link := filepath.Join(usbRoot, iface, "driver")
		switch op {
		case "unbind":
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
		case "bind":
			if err := os.Symlink(filepath.Join("..", "drivers", "uvcvideo"), link); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}

	applied, err := setUVCVideoUnbound(true)
	if err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Fatal("expected applied=true when a video interface exists")
	}
	if !reflect.DeepEqual(calls, []string{"unbind:1-7:1.0"}) {
		t.Fatalf("expected only video iface unbind, got %v", calls)
	}
	// The audio interface must never be touched by the camera switch.
	for _, c := range calls {
		if strings.Contains(c, "1-7:1.1") {
			t.Fatalf("audio interface must not be unbound/bound: %v", calls)
		}
	}

	calls = nil
	applied, err = setUVCVideoUnbound(false)
	if err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Fatal("expected applied=true when re-enabling")
	}
	if !reflect.DeepEqual(calls, []string{"bind:1-7:1.0"}) {
		t.Fatalf("expected only video iface bind, got %v", calls)
	}
}

func TestSetUVCVideoUnboundNoCameraReturnsNotApplied(t *testing.T) {
	usbRoot, _ := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot

	applied, err := setUVCVideoUnbound(true)
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("expected applied=false when no video interface exists")
	}
}

func TestSetCameraPrivacyFallsBackToUVCUnbindWhenNoV4L2(t *testing.T) {
	usbRoot, v4lDir := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	v4lClassPath = v4lDir

	// Empty video4linux dir: no V4L2 devices.
	addUSBDevice(t, usbRoot, "1-7", map[string]string{"1-7:1.0": "0e", "1-7:1.1": "01"})

	var calls []string
	writeDriverAttr = func(op, iface string) error {
		calls = append(calls, op+":"+iface)
		return nil
	}

	applied, err := setCameraPrivacy(true)
	if err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Fatal("expected applied=true via uvcvideo unbind fallback")
	}
	if !reflect.DeepEqual(calls, []string{"unbind:1-7:1.0"}) {
		t.Fatalf("expected video interface unbind, got %v", calls)
	}
}

func TestSetCameraPrivacyNoDevicesNotApplied(t *testing.T) {
	usbRoot, v4lDir := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	v4lClassPath = v4lDir

	applied, err := setCameraPrivacy(true)
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("expected applied=false when no camera devices exist")
	}
}

func TestV4L2ControlIoctlConstants(t *testing.T) {
	// These constants must match <linux/videodev2.h> so ioctl calls target the
	// right control; guard against silent UAPI drift.
	if v4l2CtrlIDPrivacy != 0x9a0910 {
		t.Fatalf("V4L2_CID_CAMERA_PRIVACY mismatch: got 0x%x", v4l2CtrlIDPrivacy)
	}
	if vidIocSCTRL != 0xc008561c {
		t.Fatalf("VIDIOC_S_CTRL mismatch: got 0x%x", vidIocSCTRL)
	}
	if vidIocQueryCtrl != 0xc0445624 {
		t.Fatalf("VIDIOC_QUERYCTRL mismatch: got 0x%x", vidIocQueryCtrl)
	}
}

func TestListVideoDevicesFiltersByUVCInterface(t *testing.T) {
	usbRoot, v4lDir := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	v4lClassPath = v4lDir

	// video0 -> UVC camera interface 1-7:1.0 (class 0e).
	ifaceDir := filepath.Join(usbRoot, "1-7:1.0")
	if err := os.MkdirAll(ifaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ifaceDir, "bInterfaceClass"), []byte("0e\n"), 0644); err != nil {
		t.Fatal(err)
	}
	video0Dir := filepath.Join(v4lDir, "video0")
	if err := os.MkdirAll(video0Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "usb", "1-7:1.0"),
		filepath.Join(video0Dir, "device")); err != nil {
		t.Fatal(err)
	}

	// video1 -> a non-video interface (class 01, e.g. audio), must be excluded.
	audioIfaceDir := filepath.Join(usbRoot, "1-7:1.1")
	if err := os.MkdirAll(audioIfaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(audioIfaceDir, "bInterfaceClass"), []byte("01\n"), 0644); err != nil {
		t.Fatal(err)
	}
	video1Dir := filepath.Join(v4lDir, "video1")
	if err := os.MkdirAll(video1Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "usb", "1-7:1.1"),
		filepath.Join(video1Dir, "device")); err != nil {
		t.Fatal(err)
	}

	// A "video" prefixed entry whose device symlink is not a USB interface.
	otherDir := filepath.Join(v4lDir, "video99")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "somewhere"), filepath.Join(otherDir, "device")); err != nil {
		t.Fatal(err)
	}

	devs, err := listVideoDevices()
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 1 || devs[0] != "/dev/video0" {
		t.Fatalf("expected only /dev/video0, got %v", devs)
	}
}

func TestListUVCVideoInterfacesNoVideoNotApplied(t *testing.T) {
	usbRoot, _ := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot

	// Only audio interfaces; no video -> nothing to switch.
	addUSBDevice(t, usbRoot, "3-9", map[string]string{"3-9:1.0": "01"})

	devs, err := listUVCVideoInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(devs) != 0 {
		t.Fatalf("expected no devices, got %v", devs)
	}
}

func TestCameraPrivacyReadsDriverBindingState(t *testing.T) {
	usbRoot, v4lDir := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	v4lClassPath = v4lDir

	// No V4L2 devices, one camera interface that is currently bound.
	addUSBDevice(t, usbRoot, "1-7", map[string]string{"1-7:1.0": "0e", "1-7:1.1": "01"})

	privacy, known := cameraPrivacy()
	if !known {
		t.Fatal("expected known=true when a camera device exists")
	}
	if privacy {
		t.Fatal("expected privacy=false while the video interface is bound")
	}

	// Unbind the video interface: privacy must now report on.
	unbindUVCInterface(t, usbRoot, "1-7:1.0")
	privacy, known = cameraPrivacy()
	if !known {
		t.Fatal("expected known=true after unbind")
	}
	if !privacy {
		t.Fatal("expected privacy=true while the video interface is unbound")
	}
}

func TestCameraPrivacyUnknownWithoutDevices(t *testing.T) {
	usbRoot, v4lDir := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	v4lClassPath = v4lDir

	if _, known := cameraPrivacy(); known {
		t.Fatal("expected known=false when no camera device exists")
	}
}

// TestSetCameraPrivacyReenableAfterDisable guards the regression where a
// deauthorized camera could never be switched back on. The re-enable path must
// still find and rebind the same video interface.
func TestSetCameraPrivacyReenableAfterDisable(t *testing.T) {
	usbRoot, v4lDir := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	v4lClassPath = v4lDir

	addUSBDevice(t, usbRoot, "1-7", map[string]string{"1-7:1.0": "0e", "1-7:1.1": "01"})

	var calls []string
	writeDriverAttr = func(op, iface string) error {
		calls = append(calls, op+":"+iface)
		// Kernel side effects: "unbind" removes the driver symlink,
		// "bind" recreates it.
		link := filepath.Join(usbRoot, iface, "driver")
		switch op {
		case "unbind":
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
		case "bind":
			if err := os.Symlink(filepath.Join("..", "drivers", "uvcvideo"), link); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}

	applied, err := setCameraPrivacy(true)
	if err != nil || !applied {
		t.Fatalf("privacy on: applied=%v err=%v", applied, err)
	}
	if !reflect.DeepEqual(calls, []string{"unbind:1-7:1.0"}) {
		t.Fatalf("expected unbind, got %v", calls)
	}

	// Re-enable must rebind the same video interface even though the driver
	// link is gone.
	calls = nil
	applied, err = setCameraPrivacy(false)
	if err != nil || !applied {
		t.Fatalf("privacy off: applied=%v err=%v", applied, err)
	}
	if !reflect.DeepEqual(calls, []string{"bind:1-7:1.0"}) {
		t.Fatalf("BUG: re-enable must bind video iface, got %v", calls)
	}

	// The audio interface must never be touched by the camera switch.
	for _, c := range calls {
		if c == "bind:1-7:1.1" || c == "unbind:1-7:1.1" {
			t.Fatalf("audio interface must not be touched: %v", calls)
		}
	}
}

// TestSetCameraPrivacyAudioOnlyUntouched ensures an audio-only USB device is
// never switched by the camera privacy path.
func TestSetCameraPrivacyAudioOnlyUntouched(t *testing.T) {
	usbRoot, v4lDir := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	v4lClassPath = v4lDir

	addUSBDevice(t, usbRoot, "3-9", map[string]string{"3-9:1.0": "01"})

	applied, err := setCameraPrivacy(true)
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("expected applied=false for audio-only device")
	}
}

// TestSetCameraPrivacyStateUpdatesMemory verifies the in-memory privacy flag
// is updated (persistence is skipped when no dconfig is wired in tests).
func TestSetCameraPrivacyStateUpdatesMemory(t *testing.T) {
	t.Cleanup(func() { cameraPrivacyOn = false })

	setCameraPrivacyState(true)
	cameraPrivacyMu.Lock()
	on := cameraPrivacyOn
	cameraPrivacyMu.Unlock()
	if !on {
		t.Fatal("expected cameraPrivacyOn=true after setCameraPrivacyState(true)")
	}

	setCameraPrivacyState(false)
	cameraPrivacyMu.Lock()
	on = cameraPrivacyOn
	cameraPrivacyMu.Unlock()
	if on {
		t.Fatal("expected cameraPrivacyOn=false after setCameraPrivacyState(false)")
	}
}

// TestReapplyCameraPrivacyWhenOn verifies that a hotplug re-apply disables the
// camera while privacy is on.
func TestReapplyCameraPrivacyWhenOn(t *testing.T) {
	usbRoot, v4lDir := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	v4lClassPath = v4lDir
	t.Cleanup(func() { cameraPrivacyOn = false })

	addUSBDevice(t, usbRoot, "1-7", map[string]string{"1-7:1.0": "0e", "1-7:1.1": "01"})

	var calls []string
	writeDriverAttr = func(op, iface string) error {
		calls = append(calls, op+":"+iface)
		return nil
	}

	cameraPrivacyMu.Lock()
	cameraPrivacyOn = true
	cameraPrivacyMu.Unlock()

	reapplyCameraPrivacy()
	if !reflect.DeepEqual(calls, []string{"unbind:1-7:1.0"}) {
		t.Fatalf("expected video iface unbind on re-apply, got %v", calls)
	}
}

// TestReapplyCameraPrivacyNoopWhenOff verifies nothing is switched while the
// privacy switch is off — a hotplug must not silently disable the camera.
func TestReapplyCameraPrivacyNoopWhenOff(t *testing.T) {
	usbRoot, v4lDir := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	v4lClassPath = v4lDir
	t.Cleanup(func() { cameraPrivacyOn = false })

	addUSBDevice(t, usbRoot, "1-7", map[string]string{"1-7:1.0": "0e"})

	var calls []string
	writeDriverAttr = func(op, iface string) error {
		calls = append(calls, op+":"+iface)
		return nil
	}

	cameraPrivacyMu.Lock()
	cameraPrivacyOn = false
	cameraPrivacyMu.Unlock()

	reapplyCameraPrivacy()
	if len(calls) != 0 {
		t.Fatalf("expected no driver ops while privacy off, got %v", calls)
	}
}

// TestShouldReapplyCamera verifies the uevent filter: video4linux add is the
// reliable signal, usb_device add is the fallback with a DEVPATH precheck,
// and everything else (usb_interface add — arrives before uvcvideo probe —
// and non-add actions) must not trigger a re-apply.
func TestShouldReapplyCamera(t *testing.T) {
	usbRoot, _ := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot
	sysfsRoot = filepath.Dir(usbRoot)
	t.Cleanup(func() { sysfsRoot = "/sys" })
	usbRel := strings.TrimPrefix(usbRoot, sysfsRoot)

	// DEVPATH prechecks read the device directory CHILDREN (real sysfs:
	// /sys/devices/.../1-4/1-4:1.0), unlike the flat usbDevicesRoot layout
	// the other fixtures mirror. Build the child layout directly.
	mkDev := func(dev string, ifaces map[string]string) {
		t.Helper()
		for iface, class := range ifaces {
			dir := filepath.Join(sysfsRoot, usbRel, dev, iface)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "bInterfaceClass"),
				[]byte(class+"\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mkDev("1-4", map[string]string{"1-4:1.0": "0e"}) // camera
	mkDev("1-5", map[string]string{"1-5:1.0": "08"}) // flash drive

	cameraDev := usbRel + "/1-4"
	usbStickDev := usbRel + "/1-5"

	cases := []struct {
		name                      string
		action, subsystem, devtyp string
		devpath                   string
		want                      bool
	}{
		{"v4l add always fires", "add", "video4linux", "video4linux", "/nonexistent", true},
		{"usb_device add with 0e iface fires", "add", "usb", "usb_device", cameraDev, true},
		{"usb_device add without 0e iface (flash drive) skipped", "add", "usb", "usb_device", usbStickDev, false},
		// Interfaces not yet created at event time: unknown must not become
		// "no", the fallback has to fire.
		{"usb_device add unreadable devpath fires", "add", "usb", "usb_device", usbRel + "/9-9", true},
		{"usb_interface add never fires", "add", "usb", "usb_interface", cameraDev, false},
		{"remove never fires", "remove", "video4linux", "video4linux", "/nonexistent", false},
		{"change never fires", "change", "usb", "usb_device", cameraDev, false},
		{"bind never fires", "bind", "usb", "usb_interface", cameraDev, false},
		{"other subsystems never fire", "add", "block", "disk", "/nonexistent", false},
	}
	for _, c := range cases {
		if got := shouldReapplyCamera(c.action, c.subsystem, c.devtyp, c.devpath); got != c.want {
			t.Errorf("%s: shouldReapplyCamera(%q, %q, %q, %q) = %v, want %v",
				c.name, c.action, c.subsystem, c.devtyp, c.devpath, got, c.want)
		}
	}
}

// TestUsbDeviceHasVideoInterfaceUnknownVsNo pins the distinction the
// fallback depends on: unreadable attribute means unknown (ok=false), a
// device whose interfaces are all non-video means a confident "no".
func TestUsbDeviceHasVideoInterfaceUnknownVsNo(t *testing.T) {
	sysfsRoot = t.TempDir()
	t.Cleanup(func() { sysfsRoot = "/sys" })
	usbRel := "/usb/1-5"

	found, ok := usbDeviceHasVideoInterface("/usb/9-9")
	if ok || found {
		t.Fatal("absent device dir must be unknown, not no")
	}

	// Child layout, as in real sysfs DEVPATH dirs.
	ifaceDir := filepath.Join(sysfsRoot, usbRel, "1-5:1.0")
	if err := os.MkdirAll(ifaceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ifaceDir, "bInterfaceClass"),
		[]byte("08\n"), 0644); err != nil {
		t.Fatal(err)
	}
	found, ok = usbDeviceHasVideoInterface(usbRel)
	if !ok {
		t.Fatal("readable non-video device must be a confident answer")
	}
	if found {
		t.Fatal("mass-storage-only device must not be reported as video")
	}
}

// TestSetUVCVideoUnboundSkipsAlreadyUnbound guards the idempotence fix: a
// re-apply over an already-disabled camera must not write unbind again (the
// kernel rejects it with -ENODEV).
func TestSetUVCVideoUnboundSkipsAlreadyUnbound(t *testing.T) {
	usbRoot, _ := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot

	addUSBDevice(t, usbRoot, "1-7", map[string]string{"1-7:1.0": "0e"})
	unbindUVCInterface(t, usbRoot, "1-7:1.0")

	var calls []string
	writeDriverAttr = func(op, iface string) error {
		calls = append(calls, op+":"+iface)
		return nil
	}

	applied, err := setUVCVideoUnbound(true)
	if err != nil {
		t.Fatal(err)
	}
	// applied stays true: an already-disabled camera must not be reported as
	// "nothing switchable", otherwise hotkey consumers would see the toggle
	// as a no-op failure.
	if !applied {
		t.Fatal("expected applied=true even when all interfaces already unbound")
	}
	if len(calls) != 0 {
		t.Fatalf("expected no unbind writes for already-unbound interfaces, got %v", calls)
	}

	// Re-enable must still bind the unbound interface.
	calls = nil
	writeDriverAttr = func(op, iface string) error {
		calls = append(calls, op+":"+iface)
		// Simulate the kernel binding the driver on "bind".
		if op == "bind" {
			if err := os.Symlink(filepath.Join("..", "drivers", "uvcvideo"),
				filepath.Join(usbRoot, iface, "driver")); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	}
	applied, err = setUVCVideoUnbound(false)
	if err != nil || !applied {
		t.Fatalf("privacy off: applied=%v err=%v", applied, err)
	}
	if !reflect.DeepEqual(calls, []string{"bind:1-7:1.0"}) {
		t.Fatalf("expected bind for previously unbound interface, got %v", calls)
	}

	// And a second disable after re-enable writes unbind again (interface
	// has a driver once more).
	calls = nil
	applied, err = setUVCVideoUnbound(true)
	if err != nil || !applied {
		t.Fatalf("privacy on again: applied=%v err=%v", applied, err)
	}
	if !reflect.DeepEqual(calls, []string{"unbind:1-7:1.0"}) {
		t.Fatalf("expected unbind after re-enable, got %v", calls)
	}
}

// TestSetUVCVideoUnboundSkipsAlreadyBound verifies re-enable does not write
// bind for interfaces that already have the driver — a hotplug re-apply with
// privacy off stays a no-op on the wire.
func TestSetUVCVideoUnboundSkipsAlreadyBound(t *testing.T) {
	usbRoot, _ := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot

	addUSBDevice(t, usbRoot, "1-7", map[string]string{"1-7:1.0": "0e"})

	var calls []string
	writeDriverAttr = func(op, iface string) error {
		calls = append(calls, op+":"+iface)
		return nil
	}

	applied, err := setUVCVideoUnbound(false)
	if err != nil {
		t.Fatal(err)
	}
	if !applied {
		t.Fatal("expected applied=true when a bound video interface exists")
	}
	if len(calls) != 0 {
		t.Fatalf("expected no bind writes for already-bound interfaces, got %v", calls)
	}
}

// TestSetUVCVideoUnboundENODEVTolerated verifies that a write failing with
// ENODEV (metadata interface racing the main interface unbind) does not
// abort the loop: remaining interfaces still get their write, and the
// overall result stays applied=true.
func TestSetUVCVideoUnboundENODEVTolerated(t *testing.T) {
	usbRoot, _ := newCameraTestEnv(t)
	restoreSysfsPaths(t)
	usbDevicesRoot = usbRoot

	addUSBDevice(t, usbRoot, "1-2", map[string]string{"1-2:1.0": "0e", "1-2:1.1": "0e"})

	writeDriverAttr = func(op, iface string) error {
		// First interface (main video) succeeds and removes its driver
		// symlink; the metadata interface write races and gets ENODEV.
		if iface == "1-2:1.0" {
			if err := os.Remove(filepath.Join(usbRoot, iface, "driver")); err != nil {
				t.Fatal(err)
			}
			return nil
		}
		return &os.PathError{Op: "write", Path: "/sys/bus/usb/drivers/uvcvideo/unbind",
			Err: unix.ENODEV}
	}

	applied, err := setUVCVideoUnbound(true)
	if err != nil {
		t.Fatalf("ENODEV on a metadata interface must not fail the call: %v", err)
	}
	if !applied {
		t.Fatal("expected applied=true despite ENODEV on metadata interface")
	}
}

// TestParseUevent verifies the raw netlink uevent parser against the kernel
// payload format: "ACTION@DEVPATH\0KEY=VALUE\0KEY=VALUE\0...".
func TestParseUevent(t *testing.T) {
	// A real video4linux add uevent shape.
	payload := []byte("add@/devices/pci0000:00/0000:00:14.0/usb1/1-4/video4linux/video0\x00" +
		"ACTION=add\x00" +
		"DEVPATH=/devices/pci0000:00/0000:00:14.0/usb1/1-4/video4linux/video0\x00" +
		"SUBSYSTEM=video4linux\x00" +
		"DEVNAME=/dev/video0\x00" +
		"DEVTYPE=video4linux\x00" +
		"MAJOR=81\x00")
	msg := parseUevent(payload)
	if msg.action != "add" || msg.subsystem != "video4linux" || msg.devtype != "video4linux" {
		t.Fatalf("unexpected parse: %+v", msg)
	}
	if msg.devpath != "/devices/pci0000:00/0000:00:14.0/usb1/1-4/video4linux/video0" {
		t.Fatalf("unexpected devpath: %q", msg.devpath)
	}
	if !shouldReapplyCamera(msg.action, msg.subsystem, msg.devtype, msg.devpath) {
		t.Fatal("expected video4linux add uevent to trigger re-apply")
	}

	// A usb_device add uevent (fallback path): DEVTYPE distinguishes it
	// from the usb_interface add which must not trigger.
	usbPayload := []byte("add@/devices/pci0000:00/0000:00:99.9/usb9/9-9\x00" +
		"ACTION=add\x00SUBSYSTEM=usb\x00DEVTYPE=usb_device\x00")
	msg = parseUevent(usbPayload)
	if msg.action != "add" || msg.subsystem != "usb" || msg.devtype != "usb_device" {
		t.Fatalf("unexpected parse: %+v", msg)
	}
	if msg.devpath != "/devices/pci0000:00/0000:00:99.9/usb9/9-9" {
		t.Fatalf("unexpected devpath: %q", msg.devpath)
	}
	// The devpath does not exist in this environment, so the precheck
	// result is "unknown" — which must not suppress the re-apply.
	if !shouldReapplyCamera(msg.action, msg.subsystem, msg.devtype, msg.devpath) {
		t.Fatal("expected usb_device add uevent with unknown sysfs to trigger re-apply")
	}

	ifacePayload := []byte("add@/devices/usb1/1-4/1-4:1.0\x00" +
		"ACTION=add\x00SUBSYSTEM=usb\x00DEVTYPE=usb_interface\x00")
	msg = parseUevent(ifacePayload)
	if shouldReapplyCamera(msg.action, msg.subsystem, msg.devtype, msg.devpath) {
		t.Fatal("usb_interface add must not trigger re-apply")
	}

	// Remove and change actions must not trigger.
	removePayload := []byte("remove@/devices/usb1/1-4/video4linux/video0\x00" +
		"ACTION=remove\x00SUBSYSTEM=video4linux\x00")
	msg = parseUevent(removePayload)
	if shouldReapplyCamera(msg.action, msg.subsystem, msg.devtype, msg.devpath) {
		t.Fatal("remove action must not trigger re-apply")
	}
}
