/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package bluetooth

import (
	//"time"

	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
)

type daemon struct {
	*loader.ModuleBase
}

func newBluetoothDaemon(logger *log.Logger) *daemon {
	var d = new(daemon)
	d.ModuleBase = loader.NewModuleBase("bluetooth", d, logger)
	return d
}

func (*daemon) GetDependencies() []string {
	return []string{}
}

var globalBluetooth *Bluetooth
var globalAgent *agent

func HandlePrepareForSleep(sleep bool) {
	if globalBluetooth == nil {
		logger.Warning("Module 'bluetooth' has not start")
		return
	}
	if sleep {
		for _, aobj := range globalBluetooth.adapters {
			if aobj.Powered {
				if err := aobj.core.StopDiscovery(0); err != nil {
					logger.Warningf("failed to stop discovery, %s, %v", aobj, err)
					continue
				}
				// 'Powered' is true in config file, so reset it
				if err := aobj.core.Powered().Set(0, false); err != nil {
					logger.Warningf("failed to set %s powered off: %v", aobj, err)
					continue
				}
			}
	

		}
		logger.Debug("prepare to sleep")
		return
	}
	logger.Debug("Wakeup from sleep, will set adapter and try connect device")
	//time.Sleep(time.Second * 3)
	for _, aobj := range globalBluetooth.adapters {
		if !aobj.Powered {
			powered := globalBluetooth.config.getAdapterConfigPowered(aobj.address)
			if !powered {
				continue
			}

			// 'Powered' is true in config file, so reset it
			if err := aobj.core.Powered().Set(0, true); err != nil {
				logger.Warningf("failed to set %s powered: %v", aobj, err)
				continue
			}
		}

		if !aobj.Discovering {
			// sometimes the adapter stops discovering and 'Discovering' property becomes 'false'
			// so try to start again
			if err := aobj.core.StartDiscovery(0); err != nil {
				logger.Warningf("failed to start discovery, %s, %v", aobj, err)
			}
		}

		_ = aobj.core.Discoverable().Set(0, globalBluetooth.config.Discoverable)
	}
	//move reconnect devices into adapter.go when power signal on coming
	//globalBluetooth.tryConnectPairedDevices()
}

func (*daemon) Start() error {
	if globalBluetooth != nil {
		return nil
	}

	service := loader.GetService()
	globalBluetooth = newBluetooth(service)

	err := service.Export(dbusPath, globalBluetooth)
	if err != nil {
		logger.Warning("failed to export bluetooth:", err)
		globalBluetooth = nil
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	sysService, err := dbusutil.NewSystemService()
	if err != nil {
		return err
	}

	globalAgent = newAgent(sysService)
	globalAgent.b = globalBluetooth
	globalBluetooth.agent = globalAgent

	err = sysService.Export(agentDBusPath, globalAgent)
	if err != nil {
		logger.Warning("failed to export agent:", err)
		return err
	}

	err = initNotifications()
	if err != nil {
		return err
	}
	// initialize bluetooth after dbus interface installed
	go globalBluetooth.init()
	return nil
}

func (*daemon) Stop() error {
	if globalBluetooth == nil {
		return nil
	}

	service := loader.GetService()
	err := service.ReleaseName(dbusServiceName)
	if err != nil {
		logger.Warning(err)
	}

	globalBluetooth.destroy()
	globalBluetooth = nil
	return nil
}
