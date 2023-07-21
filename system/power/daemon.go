// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"errors"
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
		err := d.manager.saveDsgConfig("PowerSavingModeAuto")
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = serverObj.ConnectChanged(d.manager, "PowerSavingModeEnabled", func(change *dbusutil.PropertyChanged) {
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
		d.manager.updatePowerSavingMode()
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
							if d.manager.dsgPower == nil {
								return
							}
							data, err := d.manager.dsgPower.Value(0, dsettingsMode)
							if err != nil {
								logger.Warning(err)
								return
							}
							logger.Infof(" ## time.AfterFunc 2 min manager.Mod : %s, data : %s", d.manager.Mode, data.Value().(string))
							d.manager.IsInBootTime = false
							//② 恢复流程也仅仅写内核性能模式文件，不设置后端属性
							err = d.manager.doSetMode(data.Value().(string))
							if err != nil {
								logger.Warning(err)
							}
							d.manager.loginManager.RemoveHandler(handlerId)
							err = serverObj.SetReadCallback(d.manager, "Mode", nil)
							if err != nil {
								logger.Warning(err)
							}
						})
					})
				}
			}
		})
		if err != nil {
			logger.Warning(err)
		}
		//③ 查看mode时, 恢复内核性能模式数据，不设置后端属性
		err = serverObj.SetReadCallback(d.manager, "Mode", func(read *dbusutil.PropertyRead) *dbus.Error {
			logger.Info("change to record mode")
			if highTimer != nil {
				highTimer.Stop()
			}
			defer func() {
				err := serverObj.SetReadCallback(d.manager, "Mode", nil)
				if err != nil {
					logger.Warning(err)
				}
			}()
			d.manager.IsInBootTime = false
			if d.manager.dsgPower == nil {
				return dbusutil.ToError(errors.New("dsgPower is nil"))
			}
			data, err := d.manager.dsgPower.Value(0, dsettingsMode)
			expectValue := data.Value().(string)
			if err != nil {
				logger.Warning(err)
				return dbusutil.ToError(errors.New("dsg of org.deepin.dde.daemon.power mode failed"))
			}
			logger.Infof(" SetReadCallback manager.Mode : %s, data : %s", d.manager.Mode, expectValue)
			return dbusutil.ToError(d.manager.doSetMode(expectValue))
		})
	}
	if err != nil {
		logger.Warning(err)
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
