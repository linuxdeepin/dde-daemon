// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/dde-daemon/loader"
)

var logger = log.NewLogger("daemon/system/power")

func init() {
	loader.Register(NewDaemon(logger))
}

type Daemon struct {
	*loader.ModuleBase
	manager *Manager
}

func NewDaemon(logger *log.Logger) *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("power", daemon, logger)
	return daemon
}

func (d *Daemon) GetDependencies() []string {
	return []string{}
}

func (d *Daemon) Start() (err error) {
	service := loader.GetService()
	d.manager, err = newManager(service)
	if err != nil {
		return
	}

	d.manager.batteriesMu.Lock()
	for _, bat := range d.manager.batteries {
		err := service.Export(bat.getObjPath(), bat)
		if err != nil {
			logger.Warning("failed to export battery:", err)
		}
	}
	d.manager.batteriesMu.Unlock()

	serverObj, err := service.NewServerObject(dbusPath, d.manager)
	if err != nil {
		return
	}
	// 属性写入前触发的回调函数
	err = serverObj.SetWriteCallback(d.manager, "PowerSavingModeEnabled",
		d.manager.writePowerSavingModeEnabledCb)
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeAuto", func(change *dbusutil.PropertyChanged) {
		d.manager.updatePowerSavingMode()
		err := d.manager.saveConfig()
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeEnabled", func(change *dbusutil.PropertyChanged) {
		err := d.manager.saveConfig()
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	// 属性改变后的回调函数
	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeAutoWhenBatteryLow", func(change *dbusutil.PropertyChanged) {
		d.manager.updatePowerSavingMode()
		err := d.manager.saveConfig()
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeBrightnessDropPercent", func(change *dbusutil.PropertyChanged) {
		err := d.manager.saveConfig()
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeAutoBatteryPercent", func(change *dbusutil.PropertyChanged) {
		err := d.manager.saveConfig()
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.Export()
	if err != nil {
		return
	}

	err = service.RequestName(dbusServiceName)
	return
}

func (d *Daemon) Stop() error {
	if d.manager == nil {
		return nil
	}
	service := loader.GetService()

	d.manager.batteriesMu.Lock()
	for _, bat := range d.manager.batteries {
		err := service.StopExport(bat)
		if err != nil {
			logger.Warning(err)
		}
	}
	d.manager.batteriesMu.Unlock()

	err := service.StopExport(d.manager)
	if err != nil {
		logger.Warning(err)
	}

	d.manager.destroy()
	d.manager = nil
	return nil
}
