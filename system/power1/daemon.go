// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/loader"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
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
	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeAuto", func(change *dbusutil.PropertyChanged) {
		d.manager.updatePowerMode(false) // PowerSavingModeAuto change
		err := d.manager.saveDsgConfig("PowerSavingModeAuto")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeEnabled", func(change *dbusutil.PropertyChanged) {
		enabled := change.Value.(bool)
		d.manager.PropsMu.Lock()
		d.manager.updatePowerSavingState(false)
		d.manager.PropsMu.Unlock()
		// 历史版本只有节能和平衡之间的切换
		if enabled {
			d.manager.doSetMode(ddePowerSave)
		} else {
			d.manager.doSetMode(ddeBalance)
		}
		err := d.manager.saveDsgConfig("PowerSavingModeEnabled")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	// 属性改变后的回调函数
	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeAutoWhenBatteryLow", func(change *dbusutil.PropertyChanged) {
		d.manager.refreshBatteryDisplay()
		d.manager.updatePowerMode(false) // PowerSavingModeAutoWhenBatteryLow change
		err := d.manager.saveDsgConfig("PowerSavingModeAutoWhenBatteryLow")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeBrightnessDropPercent", func(change *dbusutil.PropertyChanged) {
		err := d.manager.saveDsgConfig("PowerSavingModeBrightnessDropPercent")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeAutoBatteryPercent", func(change *dbusutil.PropertyChanged) {
		d.manager.refreshBatteryDisplay()
		d.manager.updatePowerMode(false) // PowerSavingModeAutoBatteryPercent change
		err := d.manager.saveDsgConfig("PowerSavingModeAutoBatteryPercent")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}
	if d.manager.enablePerformanceInBoot() {
		var handlerId dbusutil.SignalHandlerId
		handlerId, err = d.manager.displayManager.ConnectSessionAdded(func(session dbus.ObjectPath) {
			// 登录前tlpMode都是performance，不设置电源模式，直到有第一个用户登录了才设置电源模式
			displaySessions, err := d.manager.displayManager.Sessions().Get(0)
			if err != nil {
				logger.Warning(err)
			}
			if len(displaySessions) == 1 {
				d.manager.updatePowerMode(true)
			}
			d.manager.displayManager.RemoveHandler(handlerId)
		})
		if err != nil {
			logger.Warning(err)
		}
	}
	err = serverObj.Export()
	if err != nil {
		logger.Warning(err)
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
