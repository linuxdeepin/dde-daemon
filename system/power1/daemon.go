// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"errors"
	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/loader"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
	"sync"
)

var logger = log.NewLogger("daemon/system/power")

func init() {
	loader.Register(NewDaemon(logger))
}

type Daemon struct {
	*loader.ModuleBase
	managerMu sync.RWMutex
	manager   *Manager
}

func NewDaemon(logger *log.Logger) *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("power", daemon, logger)
	return daemon
}
func (d *Daemon) ShortIdleState() (bool, error) {
	d.managerMu.RLock()
	defer d.managerMu.RUnlock()

	manager := d.manager
	if manager == nil {
		return false, errors.New("power manager is not initialized")
	}
	return manager.getShortIdleState(), nil
}

func (d *Daemon) SetShortIdleState(state bool) error {
	d.managerMu.RLock()
	defer d.managerMu.RUnlock()

	manager := d.manager
	if manager == nil {
		return errors.New("power manager is not initialized")
	}
	return manager.setShortIdleState(state)
}


func (d *Daemon) GetDependencies() []string {
	return []string{}
}

func (d *Daemon) Start() (err error) {
	d.managerMu.Lock()
	defer d.managerMu.Unlock()

	service := loader.GetService()
	manager, err := newManager(service)
	if err != nil {
		return
	}
	d.manager = manager

	manager.batteriesMu.Lock()
	for _, bat := range manager.batteries {
		err := service.Export(bat.getObjPath(), bat)
		if err != nil {
			logger.Warning("failed to export battery:", err)
		}
	}
	manager.batteriesMu.Unlock()
	serverObj, err := service.NewServerObject(dbusPath, manager)
	if err != nil {
		return
	}
	err = serverObj.ConnectChanged(manager, "PowerSavingModeAuto", func(change *dbusutil.PropertyChanged) {
		manager.updatePowerMode(false) // PowerSavingModeAuto change
		err := manager.saveDsgConfig("PowerSavingModeAuto")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(manager, "PowerSavingModeEnabled", func(change *dbusutil.PropertyChanged) {
		enabled := change.Value.(bool)
		manager.PropsMu.Lock()
		manager.updatePowerSavingState(false)
		manager.PropsMu.Unlock()
		// 历史版本只有节能和平衡之间的切换
		if enabled {
			manager.doSetMode(ddePowerSave)
		} else {
			manager.doSetMode(ddeBalance)
		}
		err := manager.saveDsgConfig("PowerSavingModeEnabled")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	// 属性改变后的回调函数
	err = serverObj.ConnectChanged(manager, "PowerSavingModeAutoWhenBatteryLow", func(change *dbusutil.PropertyChanged) {
		manager.refreshBatteryDisplay()
		manager.updatePowerMode(false) // PowerSavingModeAutoWhenBatteryLow change
		err := manager.saveDsgConfig("PowerSavingModeAutoWhenBatteryLow")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(manager, "PowerSavingModeBrightnessDropPercent", func(change *dbusutil.PropertyChanged) {
		err := manager.saveDsgConfig("PowerSavingModeBrightnessDropPercent")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(manager, "PowerSavingModeAutoBatteryPercent", func(change *dbusutil.PropertyChanged) {
		manager.refreshBatteryDisplay()
		manager.updatePowerMode(false) // PowerSavingModeAutoBatteryPercent change
		err := manager.saveDsgConfig("PowerSavingModeAutoBatteryPercent")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}
	if manager.enablePerformanceInBoot() {
		var handlerId dbusutil.SignalHandlerId
		handlerId, err = manager.displayManager.ConnectSessionAdded(func(session dbus.ObjectPath) {
			// 登录前tlpMode都是performance，不设置电源模式，直到有第一个用户登录了才设置电源模式
			displaySessions, err := manager.displayManager.Sessions().Get(0)
			if err != nil {
				logger.Warning(err)
			}
			if len(displaySessions) == 1 {
				manager.updatePowerMode(true)
			}
			manager.displayManager.RemoveHandler(handlerId)
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
	d.managerMu.Lock()
	defer d.managerMu.Unlock()

	manager := d.manager
	if manager == nil {
		return nil
	}
	service := loader.GetService()

	manager.batteriesMu.Lock()
	for _, bat := range manager.batteries {
		err := service.StopExport(bat)
		if err != nil {
			logger.Warning(err)
		}
	}
	manager.batteriesMu.Unlock()

	err := service.StopExport(manager)
	if err != nil {
		logger.Warning(err)
	}

	manager.destroy()
	d.manager = nil
	return nil
}
