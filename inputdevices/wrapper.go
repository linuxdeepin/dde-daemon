// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

// #cgo pkg-config: x11 xi
// #cgo CFLAGS: -W -Wall -fstack-protector-all -fPIC
// #cgo LDFLAGS: -lpthread
// #include "listen.h"
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/linuxdeepin/dde-api/dxinput"
	"github.com/linuxdeepin/dde-api/dxinput/common"
	dxutils "github.com/linuxdeepin/dde-api/dxinput/utils"
	gudev "github.com/linuxdeepin/go-gir/gudev-1.0"
)

type mouseInfo struct {
	*dxinput.Mouse
	devNode string
	phys    string
}

type touchpadInfo struct {
	*dxinput.Touchpad
	devNode string
	phys    string
}

type Mouses []*mouseInfo
type Touchpads []*touchpadInfo
type dxWacoms []*dxinput.Wacom

var (
	_devInfos    common.DeviceInfos
	_mouseInfos  Mouses
	_tpadInfos   Touchpads
	_wacomInfos  dxWacoms
	_gudevClient = gudev.NewClient([]string{"input"})
)

func startDeviceListener() {
	C.start_device_listener()
}

func endDeviceListener() {
	C.end_device_listener()
}

//export handleDeviceChanged
func handleDeviceChanged() {
	logger.Debug("Device changed")

	getDeviceInfos(true)

	// 鼠标依赖触摸板的数据，必须在触摸板之后获取
	_tpadInfos = Touchpads{}
	getTPadInfos(false)
	_mouseInfos = Mouses{}
	getMouseInfos(false)
	_wacomInfos = dxWacoms{}
	getWacomInfos(false)

	if _manager == nil {
		logger.Warning("_manager is nil")
		return
	}

	_manager.tpad.handleDeviceChanged()
	_manager.mouse.handleDeviceChanged()
	_manager.wacom.handleDeviceChanged()
	_manager.kbd.handleDeviceChanged()
}

func getDeviceInfos(force bool) common.DeviceInfos {
	if force || len(_devInfos) == 0 {
		_devInfos = dxutils.ListDevice()
	}

	return _devInfos
}

func getKeyboardNumber() int {
	var number = 0
	for _, info := range getDeviceInfos(false) {
		// TODO: Improve keyboard device detected by udev property 'ID_INPUT_KEYBOARD'
		if strings.Contains(strings.ToLower(info.Name), "keyboard") {
			number += 1
		}
	}
	return number
}

func getExtraInfo(id int32) (devNode string, phys string) {
	var devNodeBytes []byte
	var length int32
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	isWaylandSession := strings.Contains(sessionType, "wayland")
	if isWaylandSession {
		devNode = fmt.Sprint("/dev/input/event", id) // id是从kwayland获取的sysname
	} else {
		devNodeBytes, length = dxutils.GetProperty(id, "Device Node")
		if len(devNodeBytes) == 0 {
			logger.Warningf("could not get DeviceNode for %d", id)
			return
		}
		devNode = string(devNodeBytes[:length])
	}
	udevDev := _gudevClient.QueryByDeviceFile(devNode)
	if udevDev == nil {
		logger.Warning("failed to get device of", devNode)
		return
	}
	defer udevDev.Unref()

	phys = udevDev.GetSysfsAttr("phys")
	if phys == "" {
		parent := udevDev.GetParent()
		if parent == nil {
			logger.Warning("failed to get parent device of", devNode)
			return
		}
		phys = parent.GetSysfsAttr("phys")

		parent.Unref()
	}

	return
}

func getTouchpadInfoByDxTouchpad(tmp *dxinput.Touchpad) *touchpadInfo {
	m := &touchpadInfo{
		Touchpad: tmp,
	}

	m.devNode, m.phys = getExtraInfo(tmp.Id)

	return m
}

func getMouseInfoByDxMouse(tmp *dxinput.Mouse) *mouseInfo {
	m := &mouseInfo{
		Mouse: tmp,
	}

	m.devNode, m.phys = getExtraInfo(tmp.Id)

	return m
}

func getMouseInfos(force bool) Mouses {
	if !force && len(_mouseInfos) != 0 {
		return _mouseInfos
	}

	_mouseInfos = Mouses{}
	for _, info := range getDeviceInfos(force) {
		if info.Type == common.DevTypeMouse {
			tmp, _ := dxinput.NewMouseFromDeviceInfo(info)
			mouse := getMouseInfoByDxMouse(tmp)

			// phys 用来标识物理设备，若俩设备的 phys 相同，说明是同一物理设备，
			// 若 phys 与某个触摸板的 phys 相同，说明是同一个设备（触摸板），忽略此鼠标设备
			found := false
			for _, touchpad := range _tpadInfos {
				logger.Warning(touchpad)
				if touchpad.phys == mouse.phys {
					found = true
					break
				}
			}

			if found {
				logger.Debug("mouse device ignored:", tmp.Name)
				continue
			}

			if mouse.phys != "" {
				_mouseInfos = append(_mouseInfos, mouse)
				logger.Debug("mouse device add:", mouse)
			}

		}
	}

	return _mouseInfos
}

func getTPadInfos(force bool) Touchpads {
	if !force && len(_tpadInfos) != 0 {
		return _tpadInfos
	}

	_tpadInfos = Touchpads{}
	for _, info := range getDeviceInfos(false) {
		if info.Type == common.DevTypeTouchpad {
			tmp, _ := dxinput.NewTouchpadFromDevInfo(info)

			_tpadInfos = append(_tpadInfos, getTouchpadInfoByDxTouchpad(tmp))
		}
	}

	return _tpadInfos
}

func getWacomInfos(force bool) dxWacoms {
	if !force && len(_wacomInfos) != 0 {
		return _wacomInfos
	}

	_wacomInfos = dxWacoms{}
	for _, info := range getDeviceInfos(false) {
		if info.Type == common.DevTypeWacom {
			tmp, _ := dxinput.NewWacomFromDevInfo(info)
			_wacomInfos = append(_wacomInfos, tmp)
		}
	}

	return _wacomInfos
}

func (infos Mouses) get(id int32) *dxinput.Mouse {
	for _, info := range infos {
		if info.Id == id {
			return info.Mouse
		}
	}
	return nil
}

func (infos Mouses) string() string {
	return toJSON(infos)
}

func (infos Touchpads) get(id int32) *dxinput.Touchpad {
	for _, info := range infos {
		if info.Id == id {
			return info.Touchpad
		}
	}
	return nil
}

func (infos Touchpads) string() string {
	return toJSON(infos)
}

func (infos dxWacoms) get(id int32) *dxinput.Wacom {
	for _, info := range infos {
		if info.Id == id {
			return info
		}
	}
	return nil
}

func (infos dxWacoms) string() string {
	return toJSON(infos)
}

func toJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
