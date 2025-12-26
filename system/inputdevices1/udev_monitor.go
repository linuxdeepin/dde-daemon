// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices1

// #cgo pkg-config: libudev
// #include <libudev.h>
// #include <stdlib.h>
// #include <string.h>
// #include <poll.h>
//
// // 永久阻塞等待 udev 事件（-1 表示永不超时）
// static struct udev_device* receive_device_blocking(struct udev_monitor *monitor) {
//     struct pollfd pfd;
//     pfd.fd = udev_monitor_get_fd(monitor);
//     pfd.events = POLLIN;
//
//     int ret = poll(&pfd, 1, -1);  // -1 = 永不超时
//     if (ret > 0) {
//         return udev_monitor_receive_device(monitor);
//     }
//     return NULL;  // 只有出错才会到这里
// }
import "C"
import (
	"strings"
	"unsafe"
)

// udevMonitor 使用 libudev 监听输入设备的插拔
type udevMonitor struct {
	udev     *C.struct_udev
	monitor  *C.struct_udev_monitor
	stopCh   chan struct{}
	callback func(devices []string)
}

// newUdevMonitor 创建 udev 监听器
func newUdevMonitor(callback func(devices []string)) *udevMonitor {
	// 创建 udev 上下文
	udev := C.udev_new()
	if udev == nil {
		logger.Warning("failed to create udev context")
		return nil
	}

	// 创建 udev 监听器
	monitor := C.udev_monitor_new_from_netlink(udev, C.CString("udev"))
	if monitor == nil {
		logger.Warning("failed to create udev monitor")
		C.udev_unref(udev)
		return nil
	}

	// 过滤：只监听 input 子系统
	subsystem := C.CString("input")
	C.udev_monitor_filter_add_match_subsystem_devtype(monitor, subsystem, nil)
	C.free(unsafe.Pointer(subsystem))

	// 启用监听器
	if C.udev_monitor_enable_receiving(monitor) < 0 {
		logger.Warning("failed to enable udev monitor")
		C.udev_monitor_unref(monitor)
		C.udev_unref(udev)
		return nil
	}

	m := &udevMonitor{
		udev:     udev,
		monitor:  monitor,
		stopCh:   make(chan struct{}),
		callback: callback,
	}

	// 启动监听 goroutine
	go m.handleEvents()

	logger.Info("udev monitor started (using libudev)")
	return m
}

// handleEvents 处理 udev 事件（永久阻塞式）
func (m *udevMonitor) handleEvents() {
	// 使用 channel 接收设备事件
	deviceCh := make(chan *C.struct_udev_device, 10)

	// 启动阻塞读取 goroutine
	go func() {
		for {
			// 永久阻塞等待事件，不占用 CPU
			device := C.receive_device_blocking(m.monitor)
			if device != nil {
				select {
				case deviceCh <- device:
				case <-m.stopCh:
					C.udev_device_unref(device)
					return
				}
			}
		}
	}()

	// 主循环处理事件或停止信号
	for {
		select {
		case <-m.stopCh:
			return
		case device := <-deviceCh:
			m.handleDevice(device)
			C.udev_device_unref(device)
		}
	}
}

// handleDevice 处理单个设备事件
func (m *udevMonitor) handleDevice(device *C.struct_udev_device) {
	// 获取事件类型
	action := C.GoString(C.udev_device_get_action(device))

	// 只关心 add 和 remove 事件
	if action != "add" && action != "remove" {
		return
	}

	// 获取设备节点
	devnode := C.GoString(C.udev_device_get_devnode(device))
	if devnode == "" || !strings.Contains(devnode, "/dev/input/event") {
		return
	}

	// 检查是否是触控板
	if !isTouchpadDeviceUdev(device) {
		return
	}

	logger.Debugf("udev: %s event for %s", action, devnode)

	// 重新扫描所有设备并回调
	if m.callback != nil {
		devices := m.enumerateDevices()
		m.callback(devices)
	}
}

// isTouchpadDeviceUdev 判断 udev 设备是否是触控板
func isTouchpadDeviceUdev(device *C.struct_udev_device) bool {
	// 方法1：检查 ID_INPUT_TOUCHPAD 属性（最准确）
	propKey := C.CString("ID_INPUT_TOUCHPAD")
	defer C.free(unsafe.Pointer(propKey))
	prop := C.udev_device_get_property_value(device, propKey)
	if prop != nil && C.GoString(prop) == "1" {
		return true
	}

	// 方法2：通过设备名称判断（降级方案）
	nameKey := C.CString("device/name")
	defer C.free(unsafe.Pointer(nameKey))
	name := C.udev_device_get_sysattr_value(device, nameKey)
	if name != nil {
		devName := C.GoString(name)
		nameLower := strings.ToLower(devName)
		return strings.Contains(nameLower, "touchpad") || isTPadPS2Mouse(nameLower)
	}

	return false
}

// isTPadPS2Mouse 判断是否是 PS/2 触控板
func isTPadPS2Mouse(name string) bool {
	return strings.Contains(name, "ps/2") && strings.Contains(name, "mouse") && !strings.Contains(name, "usb")
}

// enumerateDevices 枚举所有触控板设备
func (m *udevMonitor) enumerateDevices() []string {
	var devices []string

	// 创建枚举器
	enumerate := C.udev_enumerate_new(m.udev)
	if enumerate == nil {
		logger.Warning("failed to create udev enumerate")
		return devices
	}
	defer C.udev_enumerate_unref(enumerate)

	// 过滤：只枚举 input 子系统
	subsystem := C.CString("input")
	C.udev_enumerate_add_match_subsystem(enumerate, subsystem)
	C.free(unsafe.Pointer(subsystem))

	// 扫描设备
	C.udev_enumerate_scan_devices(enumerate)

	// 获取设备列表
	deviceMap := make(map[string]bool)
	entry := C.udev_enumerate_get_list_entry(enumerate)
	for entry != nil {
		// 获取设备路径
		syspath := C.udev_list_entry_get_name(entry)
		device := C.udev_device_new_from_syspath(m.udev, syspath)

		if device != nil {
			// 只处理 event 设备
			devnode := C.GoString(C.udev_device_get_devnode(device))
			if devnode != "" && strings.Contains(devnode, "/dev/input/event") {
				// 检查是否是触控板
				if isTouchpadDeviceUdev(device) {
					// 获取设备名称
					nameKey := C.CString("device/name")
					name := C.udev_device_get_sysattr_value(device, nameKey)
					C.free(unsafe.Pointer(nameKey))
					if name != nil {
						devName := strings.TrimSpace(C.GoString(name))
						if devName != "" && !deviceMap[devName] {
							deviceMap[devName] = true
							devices = append(devices, devName)
						}
					}
				}
			}
			C.udev_device_unref(device)
		}

		entry = C.udev_list_entry_get_next(entry)
	}

	return devices
}

// destroy 销毁监听器
func (m *udevMonitor) destroy() {
	if m.stopCh != nil {
		close(m.stopCh)
	}

	if m.monitor != nil {
		C.udev_monitor_unref(m.monitor)
		m.monitor = nil
	}

	if m.udev != nil {
		C.udev_unref(m.udev)
		m.udev = nil
	}

	logger.Info("udev monitor stopped")
}
