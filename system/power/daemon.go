// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"sync"
	"time"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-daemon/loader"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
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
	if d.manager.enablePerformanceInBoot() {
		var once sync.Once
		var handlerId dbusutil.SignalHandlerId
		var highTimer *time.Timer
		handlerId, err = d.manager.loginManager.ConnectSessionNew(func(sessionId string, sessionPath dbus.ObjectPath) {
			session, err := login1.NewSession(service.Conn(), sessionPath)
			if err == nil {
				name, err := session.Name().Get(0)
				if err == nil && name != "lightdm" {
					// 登录后两分钟内高性能,两分钟后修改回原有的mode
					once.Do(func() {
						highTimer = time.AfterFunc(time.Minute*2, func() {
							_ = d.manager.SetMode(d.manager.fakeMode)
							d.manager.loginManager.RemoveHandler(handlerId)
							_ = serverObj.SetReadCallback(d.manager, "Mode", nil)
						})
					})
				}
			}
		})
		if err != nil {
			logger.Warning(err)
		}
		// 查看mode时,将mode还原成原有的mode
		err = serverObj.SetReadCallback(d.manager, "Mode", func(read *dbusutil.PropertyRead) *dbus.Error {
			logger.Info("change to record mode")
			if highTimer != nil {
				highTimer.Stop()
			}
			defer func() {
				_ = serverObj.SetReadCallback(d.manager, "Mode", nil)
			}()
			return d.manager.SetMode(d.manager.fakeMode)
		})
	}
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
