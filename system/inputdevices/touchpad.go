// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const(
    touchpadSwitchFile = "/proc/uos/touchpad_switch"
    touchpadDBusPath = "/com/deepin/system/InputDevices/Touchpad"
    touchpadDBusInterface = "com.deepin.system.InputDevices.Touchpad"
)

type Touchpad struct {
	service *dbusutil.Service
	Enable  bool
	IsExist bool
}

func newTouchpad(service *dbusutil.Service) *Touchpad {
	t := &Touchpad {
		service: service,
	}
	err := TouchpadExist(touchpadSwitchFile)
	if err != nil {
		logger.Warning(err)
		t.setPropIsExist(false)
		t.setPropEnable(false)
		return t
	}
	t.setPropIsExist(true)
	enable, err := TouchpadEnable(touchpadSwitchFile)
	if err != nil {
		logger.Warning(err)
	}
	t.setPropEnable(enable)
	return t
}

func (t *Touchpad) SetTouchpadEnable(enabled bool) *dbus.Error {
	err := t.setTouchpadEnable(enabled)
	return dbusutil.ToError(err)
}

func (t *Touchpad) setTouchpadEnable(enabled bool) error {
	if err := TouchpadExist(touchpadSwitchFile); err != nil {
		return err
	}
	current, err := TouchpadEnable(touchpadSwitchFile)
	if err != nil {
		logger.Warning(err)
		return err
	}
	if current == enabled {
		return nil
	}
	arg := "enable"
	if !enabled {
		arg = "disable"
	}
	err = ioutil.WriteFile(touchpadSwitchFile, []byte(arg), 0644)
	if err != nil{
		logger.Warning(err)
		return err
	}
	t.setPropEnable(enabled)
	return nil
}

func TouchpadEnable(filePath string) (bool, error) {
	err := TouchpadExist(filePath)
	if err != nil {
		logger.Warning(err)
		return false, err
	}
	content, err := ioutil.ReadFile(touchpadSwitchFile)
	if err != nil {
		logger.Warning(err)
		return false, err
	}
	return strings.Contains(string(content), "enable"), nil
}

func TouchpadExist(filePath string) error {
	if filePath != touchpadSwitchFile {
		return errors.New("filePath is inValid")
	}
	_, err := os.Stat(touchpadSwitchFile)
	if err != nil {
		logger.Warning(err)
		return err 
	}
	return nil
}

func (t *Touchpad) GetInterfaceName() string {
	return touchpadDBusInterface
}

func (t *Touchpad) export(path dbus.ObjectPath) error {
	err := t.service.Export(path, t)
	if err != nil {
		logger.Warning(err)
		return err
	}

	return nil
}

func (t *Touchpad) stopExport() error {
	return t.service.StopExport(t)
}