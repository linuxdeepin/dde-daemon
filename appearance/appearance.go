// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

// Manage desktop appearance
package appearance

import (
	"errors"
	"time"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"

	"github.com/linuxdeepin/dde-daemon/appearance/background"
	"github.com/linuxdeepin/dde-daemon/loader"
)

var (
	_m     *Manager
	logger = log.NewLogger("daemon/appearance")
)

type Module struct {
	*loader.ModuleBase
}

func init() {
	background.SetLogger(logger)
	loader.Register(NewModule(logger))
}

func NewModule(logger *log.Logger) *Module {
	var d = new(Module)
	d.ModuleBase = loader.NewModuleBase("appearance", d, logger)
	return d
}

func HandlePrepareForSleep(sleep bool) {
	if _m == nil {
		return
	}
	if sleep {
		return
	}
	cfg, err := doUnmarshalWallpaperSlideshow(_m.WallpaperSlideShow.Get())
	if err == nil {
		for monitorSpace := range cfg {
			if cfg[monitorSpace] == wsPolicyWakeup {
				_m.autoChangeBg(monitorSpace, time.Now())
			}
		}
	}
}

func (*Module) GetDependencies() []string {
	return []string{}
}

func (*Module) start() error {
	service := loader.GetService()

	_m = newManager(service)
	err := _m.init()
	if err != nil {
		logger.Warning(err)
		return err
	}

	err = service.Export(dbusPath, _m, _m.syncConfig)
	if err != nil {
		_m.destroy()
		return err
	}

	err = service.Export(backgroundDBusPath, _m.bgSyncConfig)
	if err != nil {
		return err
	}

	so := service.GetServerObject(_m)
	err = so.SetWriteCallback(_m, propQtActiveColor, func(write *dbusutil.PropertyWrite) *dbus.Error {
		value, ok := write.Value.(string)
		if !ok {
			return dbusutil.ToError(errors.New("type is not string"))
		}
		err = _m.setQtActiveColor(value)
		return dbusutil.ToError(err)
	})
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		_m.destroy()
		err = service.StopExport(_m)
		if err != nil {
			return err
		}
		return err
	}

	err = _m.syncConfig.Register()
	if err != nil {
		logger.Warning("failed to register for deepin sync", err)
	}

	err = _m.bgSyncConfig.Register()
	if err != nil {
		logger.Warning("failed to register for deepin sync", err)
	}

	go _m.listenCursorChanged()
	go _m.handleThemeChanged()
	_m.listenGSettingChanged()
	_m.initFont()
	return nil
}

func (m *Module) Start() error {
	if _m != nil {
		return nil
	}
	return m.start()
}

func (*Module) Stop() error {
	if _m == nil {
		return nil
	}

	_m.destroy()
	service := loader.GetService()
	err := service.StopExport(_m)
	if err != nil {
		return err
	}
	_m = nil
	return nil
}
