/*
 * Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:     zsien <i@zsien.cn>
 *
 * Maintainer: zsien <i@zsien.cn>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package inputdevices

// #cgo pkg-config: libinput libudev
// #include <libinput.h>
// #include <libudev.h>
// #include "dde-libinput.h"
import "C"
import (
	"unsafe"

	"pkg.deepin.io/lib/log"
)

//export log_handler_go
func log_handler_go(priority C.enum_libinput_log_priority, cstr *C.char) {
	str := C.GoString(cstr)
	switch priority {
	case C.LIBINPUT_LOG_PRIORITY_DEBUG:
		logger.Debug(str)
	case C.LIBINPUT_LOG_PRIORITY_INFO:
		logger.Info(str)
	case C.LIBINPUT_LOG_PRIORITY_ERROR:
		logger.Error(str)
	}
}

//export handle_device_added
func handle_device_added(event *C.struct_libinput_event, userdata unsafe.Pointer) {
	dev := C.libinput_event_get_device(event)
	if C.libinput_device_has_capability(dev, C.LIBINPUT_DEVICE_CAP_TOUCH) == 0 {
		return
	}

	l := (*libinput)(userdata)
	l.i.newTouchscreen(newLibinputDevice(dev))
}

//export handle_device_removed
func handle_device_removed(event *C.struct_libinput_event, userdata unsafe.Pointer) {
	dev := C.libinput_event_get_device(event)
	if C.libinput_device_has_capability(dev, C.LIBINPUT_DEVICE_CAP_TOUCH) == 0 {
		return
	}

	l := (*libinput)(userdata)
	l.i.removeTouchscreen(newLibinputDevice(dev))
}

type libinput struct {
	data   *C.struct_data
	stopCh chan struct{}

	i *InputDevices
}

func newLibinput(i *InputDevices) *libinput {
	var priority C.enum_libinput_log_priority = C.LIBINPUT_LOG_PRIORITY_ERROR

	debugLevel := logger.GetLogLevel()
	switch debugLevel {
	case log.LevelInfo:
		priority = C.LIBINPUT_LOG_PRIORITY_INFO
	case log.LevelDebug:
		priority = C.LIBINPUT_LOG_PRIORITY_DEBUG
	}

	l := new(libinput)
	l.i = i
	l.data = C.new_libinput((*C.char)(unsafe.Pointer(l)), priority)

	return l
}

func (l *libinput) start() {
	l.stopCh = make(chan struct{}, 1)

	go func() {
		C.start(l.data)

		l.stopCh <- struct{}{}
	}()
}

func (l *libinput) stop() {
	C.stop(l.data)

	<-l.stopCh
}

type libinputDevice struct {
	dev *C.struct_libinput_device
}

func newLibinputDevice(dev *C.struct_libinput_device) *libinputDevice {
	return &libinputDevice{
		dev: dev,
	}
}

func (d *libinputDevice) GetName() string {
	return C.GoString(C.libinput_device_get_name(d.dev))
}

func (d *libinputDevice) GetOutputName() string {
	return C.GoString(C.libinput_device_get_output_name(d.dev))
}

func (d *libinputDevice) GetSize() (float64, float64) {
	var width C.double
	var height C.double
	C.libinput_device_get_size(d.dev, &width, &height)

	return float64(width), float64(height)
}

func (d *libinputDevice) GetPhys() string {
	udev := C.libinput_device_get_udev_device(d.dev)
	v := C.udev_device_get_sysattr_value(udev, C.CString("phys"))
	if v == nil {
		parent := C.udev_device_get_parent(udev)
		v = C.udev_device_get_sysattr_value(parent, C.CString("phys"))
	}
	C.udev_device_unref(udev)

	return C.GoString(v)
}

func (d *libinputDevice) GetDevNode() string {
	udev := C.libinput_device_get_udev_device(d.dev)
	v := C.GoString(C.udev_device_get_devnode(udev))
	C.udev_device_unref(udev)
	return v
}
func (d *libinputDevice) GetSysAttr(k string) string {
	udev := C.libinput_device_get_udev_device(d.dev)
	v := C.udev_device_get_sysattr_value(udev, C.CString(k))
	C.udev_device_unref(udev)
	return C.GoString(v)
}

func (d *libinputDevice) GetProperty(k string) string {
	udev := C.libinput_device_get_udev_device(d.dev)
	v := C.GoString(C.udev_device_get_property_value(udev, C.CString(k)))
	C.udev_device_unref(udev)
	return v
}
