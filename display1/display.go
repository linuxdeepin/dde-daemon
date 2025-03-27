// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"errors"
	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

const (
	dbusServiceName = "org.deepin.dde.Display1"
	dbusInterface   = "org.deepin.dde.Display1"
	dbusPath        = "/org/deepin/dde/Display1"
)

var _dpy *Manager

var _greeterMode bool

func SetGreeterMode(val bool) {
	_greeterMode = val
}

func Start(service *dbusutil.Service) error {
	m := newManager(service)
	m.init()

	if !_greeterMode {
		// 正常 startdde
		err := service.Export(dbusPath, m)
		if err != nil {
			return err
		}
		so := service.GetServerObject(m)
		if so != nil {
			err = so.SetWriteCallback(m, "ColorTemperatureEnabled", func(write *dbusutil.PropertyWrite) *dbus.Error {
				value, ok := write.Value.(bool)
				if !ok {
					err = errors.New("Type is not bool")
					logger.Warning(err)
					return dbusutil.ToError(err)
				}
				ok = m.setPropColorTemperatureEnabled(value)
				if !ok {
					err = errors.New("Set ColorTemperatureEnabled failed!")
					logger.Warning(err)
					return dbusutil.ToError(err)
				}
				cfg := m.getSuitableUserMonitorModeConfig(m.DisplayMode)
				if cfg == nil {
					cfg = getDefaultUserMonitorModeConfig()
				}
				mode := ColorTemperatureModeNone
				if value {
					mode = m.colorTemperatureModeOn
				} else {
					m.colorTemperatureModeOn = m.ColorTemperatureMode
				}
				err = m.setColorTempMode(mode)
				return dbusutil.ToError(err)
			})
		}

		err = service.RequestName(dbusServiceName)
		if err != nil {
			return err
		}
	}
	_dpy = m
	return nil
}

func StartPart2() error {
	if _dpy == nil {
		return errors.New("_dpy is nil")
	}
	m := _dpy
	m.initSysDisplay()
	m.initTouchscreens()

	if !_greeterMode {
		controlRedshift("disable")
		m.applyColorTempConfig(m.DisplayMode)
	}

	return nil
}

func SetLogLevel(level log.Priority) {
	logger.SetLogLevel(level)
}
