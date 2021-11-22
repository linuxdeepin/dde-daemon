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
	"fmt"
	"os"
	"time"

	dbus "github.com/godbus/dbus"
	bluez "github.com/linuxdeepin/go-dbus-factory/org.bluez"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/proxy"
)

type adapter struct {
	core    bluez.HCI
	address string

	Path                dbus.ObjectPath
	Name                string
	Alias               string
	Powered             bool
	Discovering         bool
	Discoverable        bool
	DiscoverableTimeout uint32
	// discovering timer, when time is up, stop discovering until start button is clicked next time
	discoveringTimeout *time.Timer
	//Scan timeout flag
	discoveringTimeoutFlag              bool
	scanReadyToConnectDeviceTimeout     *time.Timer
	scanReadyToConnectDeviceTimeoutFlag bool
	waitDiscovery                       bool
	connectingCount                     int
}

var defaultDiscoveringTimeout = 1 * time.Minute
var defaultFindDeviceTimeout = 1 * time.Second

func newAdapter(systemSigLoop *dbusutil.SignalLoop, apath dbus.ObjectPath) (a *adapter) {
	a = &adapter{Path: apath}
	systemConn := systemSigLoop.Conn()
	a.core, _ = bluez.NewHCI(systemConn, apath)
	a.core.InitSignalExt(systemSigLoop, true)
	a.connectProperties()
	a.address, _ = a.core.Adapter().Address().Get(0)
	// 用于定时停止扫描
	a.discoveringTimeout = time.AfterFunc(defaultDiscoveringTimeout, func() {
		logger.Debug("discovery time out, stop discovering")
		//扫描结束后更新备份
		globalBluetooth.backupDeviceLock.Lock()
		globalBluetooth.backupDevices = make(map[dbus.ObjectPath][]*backupDevice)
		for adapterpath, devices := range globalBluetooth.devices {
			for _, device := range devices {
				globalBluetooth.backupDevices[adapterpath] = append(globalBluetooth.backupDevices[adapterpath], newBackupDevice(device))
			}
		}
		globalBluetooth.backupDeviceLock.Unlock()
		//Scan timeout
		a.discoveringTimeoutFlag = true
		if err := a.core.Adapter().StopDiscovery(0); err != nil {
			logger.Warningf("stop discovery failed, err:%v", err)
		}
		globalBluetooth.prepareToConnectedDevice = ""
	})
	//扫描1S钟，未扫描到该设备弹出通知
	a.scanReadyToConnectDeviceTimeout = time.AfterFunc(defaultFindDeviceTimeout, func() {
		a.scanReadyToConnectDeviceTimeoutFlag = false
		_, err := globalBluetooth.getDevice(globalBluetooth.prepareToConnectedDevice)
		if err != nil {
			backupdevice, err1 := globalBluetooth.getBackupDevice(globalBluetooth.prepareToConnectedDevice)
			if err1 != nil {
				logger.Debug("getBackupDevice Failed:", err1)
				return
			}
			notifyConnectFailedHostDown(backupdevice.Alias)
		}
		//清空备份
		globalBluetooth.backupDeviceLock.Lock()
		globalBluetooth.backupDevices = make(map[dbus.ObjectPath][]*backupDevice)
		globalBluetooth.backupDeviceLock.Unlock()
	})
	// stop timer at first
	a.discoveringTimeout.Stop()
	a.scanReadyToConnectDeviceTimeout.Stop()
	// fix alias
	alias, _ := a.core.Adapter().Alias().Get(0)
	if alias == "first-boot-hostname" {
		hostname, err := os.Hostname()
		if err == nil {
			if hostname != "first-boot-hostname" {
				// reset alias
				err = a.core.Adapter().Alias().Set(0, "")
				if err != nil {
					logger.Warning(err)
				}
			}
		} else {
			logger.Warning("failed to get hostname:", err)
		}
	}

	a.Alias, _ = a.core.Adapter().Alias().Get(0)
	a.Name, _ = a.core.Adapter().Name().Get(0)
	a.Powered, _ = a.core.Adapter().Powered().Get(0)
	a.Discovering, _ = a.core.Adapter().Discovering().Get(0)
	a.Discoverable, _ = a.core.Adapter().Discoverable().Get(0)
	a.DiscoverableTimeout, _ = a.core.Adapter().DiscoverableTimeout().Get(0)
	return
}

func (a *adapter) destroy() {
	a.core.RemoveHandler(proxy.RemoveAllHandlers)
}

func (a *adapter) String() string {
	return fmt.Sprintf("adapter %s [%s]", a.Alias, a.address)
}

func (a *adapter) notifyAdapterAdded() {
	logger.Info("AdapterAdded", a)
	err := globalBluetooth.service.Emit(globalBluetooth, "AdapterAdded", marshalJSON(a))
	if err != nil {
		logger.Warning(err)
	}
	globalBluetooth.updateState()
}

func (a *adapter) notifyAdapterRemoved() {
	logger.Info("AdapterRemoved", a)
	err := globalBluetooth.service.Emit(globalBluetooth, "AdapterRemoved", marshalJSON(a))
	if err != nil {
		logger.Warning(err)
	}
	globalBluetooth.updateState()
}

func (a *adapter) notifyPropertiesChanged() {
	err := globalBluetooth.service.Emit(globalBluetooth, "AdapterPropertiesChanged", marshalJSON(a))
	if err != nil {
		logger.Warning(err)
	}
	globalBluetooth.updateState()
}

func (a *adapter) connectProperties() {
	err := a.core.Adapter().Name().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}
		a.Name = value
		logger.Debugf("%s Name: %v", a, value)
		a.notifyPropertiesChanged()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = a.core.Adapter().Alias().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}
		a.Alias = value
		logger.Debugf("%s Alias: %v", a, value)
		a.notifyPropertiesChanged()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = a.core.Adapter().Powered().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		a.Powered = value
		logger.Debugf("%s Powered: %v", a, value)

		power := globalBluetooth.config.getAdapterConfigPowered(a.address)
		if power != a.Powered {
			err = a.core.Adapter().Powered().Set(0, power)
			if err != nil {
				logger.Warning("set Adapter Powered failed:", err)
			}
		}

		if a.Powered {
			err := a.core.Adapter().Discoverable().Set(0, globalBluetooth.config.Discoverable)
			if err != nil {
				logger.Warningf("failed to set discoverable for %s: %v", a, err)
			}
			go func() {
				time.Sleep(1 * time.Second)
				err = a.core.Adapter().StopDiscovery(0)
				if err != nil {
					logger.Warningf("failed to stop discovery for %s: %v", a, err)
				}
				// in case auto connect to device failed, only when signal power on is received, try to auto connect device
				globalBluetooth.tryConnectPairedDevices()
				a.waitDiscovery = true
				a.startDiscovery()
			}()
		} else {
			// if power off, stop discovering time out
			a.discoveringTimeout.Stop()
		}
		// Sleep for 1s and wait for bluez to set the attributes before sending the attribute change signal
		time.Sleep(1 * time.Second)
		a.notifyPropertiesChanged()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = a.core.Adapter().Discovering().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		a.Discovering = value
		logger.Debugf("%s Discovering: %v", a, value)
		//Scan timeout and send attribute change signal directly
		if a.discoveringTimeoutFlag {
			a.notifyPropertiesChanged()
		} else {
			if value != a.Powered {
				return
			}
			a.notifyPropertiesChanged()
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = a.core.Adapter().Discoverable().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		a.Discoverable = value
		logger.Debugf("%s Discoverable: %v", a, value)
		a.notifyPropertiesChanged()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = a.core.Adapter().DiscoverableTimeout().ConnectChanged(func(hasValue bool, value uint32) {
		if !hasValue {
			return
		}
		a.DiscoverableTimeout = value
		logger.Debugf("%s DiscoverableTimeout: %v", a, value)
		a.notifyPropertiesChanged()
	})
	if err != nil {
		logger.Warning(err)
	}
}
func (a *adapter) startDiscovery() {
	if a.connectingCount > 0 {
		logger.Info("some devices connecting, can not start discovery")
		return
	}

	a.discoveringTimeoutFlag = false
	logger.Debugf("start discovery")
	err := a.core.Adapter().StartDiscovery(0)
	if err != nil {
		logger.Warningf("failed to start discovery for %s: %v", a, err)
	} else {
		logger.Debug("reset timer for stop scan")
		a.waitDiscovery = false
		// start discovering success, reset discovering timer
		a.discoveringTimeout.Reset(defaultDiscoveringTimeout)
	}
}

func (a *adapter) addConnectingCount() {
	logger.Debug("add connecting count")
	a.connectingCount++
}

func (a *adapter) minusConnectingCount() {
	if a.connectingCount > 0 {
		logger.Debug("had device connecting, return")
		a.connectingCount--
	}

	if a.waitDiscovery && a.connectingCount == 0 {
		a.waitDiscovery = false
		a.startDiscovery()
	}
}
