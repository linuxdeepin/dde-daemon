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

	// Spy on writeDriverAttr, simulating the kernel driver side effects.
	var calls []string
	writeDriverAttr = func(op, iface string) error {
		calls = append(calls, op+":"+iface)
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
