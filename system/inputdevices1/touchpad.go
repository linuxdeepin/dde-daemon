// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices1

import (
	"errors"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	// touchpad接口的导出不再依赖touchpadSwitchFile文件的存在，只有设置的时候才会读写该文件配置
	touchpadSwitchFile    = "/proc/uos/touchpad_switch"
	touchpadDBusPath      = "/org/deepin/dde/InputDevices1/Touchpad"
	touchpadDBusInterface = "org.deepin.dde.InputDevices1.Touchpad"
)

type Touchpad struct {
	service *dbusutil.Service
	Enable  bool
}

func newTouchpad(service *dbusutil.Service) *Touchpad {
	t := &Touchpad{
		service: service,
		Enable:  getDsgConf(),
	}
	return t
}

func (t *Touchpad) SetTouchpadEnable(enabled bool) *dbus.Error {
	err := t.setTouchpadEnable(enabled)
	return dbusutil.ToError(err)
}

func (t *Touchpad) setTouchpadEnable(enabled bool) error {
	t.setPropEnable(enabled)
	err := setDsgConf(enabled)
	if err != nil {
		logger.Warning(err)
		return err
	}

	if err = TouchpadExist(touchpadSwitchFile); err != nil {
		return nil
	}
	current, err := TouchpadEnable(touchpadSwitchFile)
	if err != nil {
		logger.Warning(" TouchpadEnable err : ", err)
		return err
	}
	if current == enabled {
		logger.Info("current touchPad state is same : ", enabled)
		return nil
	}
	arg := "enable"
	if !enabled {
		arg = "disable"
	}
	err = os.WriteFile(touchpadSwitchFile, []byte(arg), 0644)
	if err != nil {
		logger.Warning(" os.WriteFile err : ", err)
		return err
	}
	return nil
}

func setDsgConf(enable bool) error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	ds := configManager.NewConfigManager(sysBus)
	confPath, err := ds.AcquireManager(0, _dsettingsAppID, _dsettingsInputdevicesName, "")
	if err != nil {
		return err
	}
	dsManager, err := configManager.NewManager(sysBus, confPath)
	if err != nil {
		return err
	}
	err = dsManager.SetValue(0, _dsettingsTouchpadEnabledKey, dbus.MakeVariant(enable))
	if err != nil {
		return err
	}
	return nil
}

func getDsgConf() bool {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return false
	}
	ds := configManager.NewConfigManager(sysBus)
	confPath, err := ds.AcquireManager(0, _dsettingsAppID, _dsettingsInputdevicesName, "")
	if err != nil {
		return false
	}
	dsManager, err := configManager.NewManager(sysBus, confPath)
	if err != nil {
		return false
	}
	data, err := dsManager.Value(0, _dsettingsTouchpadEnabledKey)
	if err != nil {
		return false
	}
	return data.Value().(bool)
}

func TouchpadEnable(filePath string) (bool, error) {
	err := TouchpadExist(filePath)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(touchpadSwitchFile)
	if err != nil {
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
	return t.service.Export(path, t)
}

func (t *Touchpad) stopExport() error {
	return t.service.StopExport(t)
}
