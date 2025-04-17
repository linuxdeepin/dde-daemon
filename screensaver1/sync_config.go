// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package screensaver

import (
	"encoding/json"
	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-gir/gio-2.0"
)

const (
	dScreenSaverPath        = "/org/deepin/dde/ScreenSaver1"
	dScreenSaverServiceName = "org.deepin.dde.ScreenSaver1"

	gsSchemaPower = "com.deepin.dde.power"

	keyCurrent                     = "currentScreenSaver"
	keyLockScreenAtAwake           = "lockScreenAtAwake"
	keybatteryScreenSaverTimeout   = "batteryScreenSaverTimeout"
	keyLinePowerScreenSaverTimeout = "linePowerScreenSaverTimeout"

	deepinScreensaverDBusServiceName   = "com.deepin.ScreenSaver"
	deepinScreensaverDBusPath          = "/com/deepin/ScreenSaver"
	deepinScreensaverDBusInterfaceName = deepinScreensaverDBusServiceName
	dbusPropertyName                   = "org.freedesktop.DBus.Properties"
)

type syncConfig struct {
}

func (sc *syncConfig) Get() (interface{}, error) {
	gs := gio.NewSettings(gsSchemaPower)
	defer gs.Unref()
	var v syncData
	v.Version = "1.0"
	bus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return nil, err
	}

	obj := bus.Object(deepinScreensaverDBusServiceName, deepinScreensaverDBusPath)
	err = obj.Call(dbusPropertyName+".Get", 0, deepinScreensaverDBusInterfaceName, keyCurrent).Store(&v.Current)
	if err != nil {
		logger.Warning(err)
	}
	err = obj.Call(dbusPropertyName+".Get", 0, deepinScreensaverDBusInterfaceName, keyLockScreenAtAwake).Store(&v.LockScreenAtAwake)
	if err != nil {
		logger.Warning(err)
	}
	err = obj.Call(dbusPropertyName+".Get", 0, deepinScreensaverDBusInterfaceName, keybatteryScreenSaverTimeout).Store(&v.BatteryDelay)
	if err != nil {
		logger.Warning(err)
	}
	err = obj.Call(dbusPropertyName+".Get", 0, deepinScreensaverDBusInterfaceName, keyLinePowerScreenSaverTimeout).Store(&v.LinePowerDelay)
	if err != nil {
		logger.Warning(err)
	}
	return &v, nil
}

func (sc *syncConfig) Set(data []byte) error {
	var v syncData
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}

	bus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return err
	}

	obj := bus.Object(deepinScreensaverDBusServiceName, deepinScreensaverDBusPath)
	setConfig := func(key string, val interface{}) {
		err := obj.Call(dbusPropertyName+".Set", 0, deepinScreensaverDBusInterfaceName, key, dbus.MakeVariant(val)).Err
		if err != nil {
			logger.Warningf("sync %v to %v failed, %v", key, val, err)
		}
	}
	setConfig(keyCurrent, v.Current)
	setConfig(keyLockScreenAtAwake, v.LockScreenAtAwake)
	setConfig(keybatteryScreenSaverTimeout, int32(v.BatteryDelay))
	setConfig(keyLinePowerScreenSaverTimeout, int32(v.LinePowerDelay))
	return nil
}

// version: 1.0
type syncData struct {
	Version           string `json:"version"`
	BatteryDelay      int    `json:"battery_delay"`
	LinePowerDelay    int    `json:"line_power_delay"`
	LockScreenAtAwake bool   `json:"lock_screen_at_awake"`
	Current           string `json:"current"`
}
