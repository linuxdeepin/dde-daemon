package bluetooth

import (
	"fmt"
	"time"

	airplanemode "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.airplanemode"
	dbus "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

func (b *Bluetooth) ConnectDevice(dpath dbus.ObjectPath, apath dbus.ObjectPath) *dbus.Error {
	d, err := b.getDevice(dpath)
	b.prepareToConnectedLock.Lock()
	b.prepareToConnectedDevice = dpath
	b.prepareToConnectedLock.Unlock()
	if err != nil {
		logger.Debug("getDevice failed:", err)
		a, err := b.getAdapter(apath)
		if err != nil {
			logger.Debug("getAdapter failed:", err)
		}
		a.startDiscovery()
		a.scanReadyToConnectDeviceTimeoutFlag = true
		a.scanReadyToConnectDeviceTimeout.Reset(defaultFindDeviceTimeout)
	} else {
		go d.Connect()
	}
	return nil
}

func (b *Bluetooth) DisconnectDevice(dpath dbus.ObjectPath) *dbus.Error {
	d, err := b.getDevice(dpath)
	if err != nil {
		return dbusutil.ToError(err)
	}
	go d.Disconnect()
	return nil
}

func (b *Bluetooth) RemoveDevice(apath, dpath dbus.ObjectPath) *dbus.Error {
	a, err := b.getAdapter(apath)
	if err != nil {
		return dbusutil.ToError(err)
	}
	// find remove device from map
	removeDev, err := b.getDevice(dpath)
	if err != nil {
		logger.Warningf("failed to get device, err: %v", err)
		return dbusutil.ToError(err)
	}
	b.removeBackupDevice(dpath)
	// check if device connect state is connecting, if is, mark remove state as true
	deviceState := removeDev.getState()
	if deviceState == deviceStateConnecting {
		removeDev.markNeedRemove(true)
	} else {
		// connection finish, allow to remove device directly
		err = a.core.RemoveDevice(0, dpath)
		if err != nil {
			logger.Warningf("failed to remove device %q from adapter %q: %v",
				dpath, apath, err)
			return dbusutil.ToError(err)
		}
	}
	return nil
}

func (b *Bluetooth) SetDeviceAlias(dpath dbus.ObjectPath, alias string) *dbus.Error {
	d, err := b.getDevice(dpath)
	if err != nil {
		return dbusutil.ToError(err)
	}
	err = d.core.Alias().Set(0, alias)
	if err != nil {
		return dbusutil.ToError(err)
	}
	return nil
}

func (b *Bluetooth) SetDeviceTrusted(dpath dbus.ObjectPath, trusted bool) *dbus.Error {
	d, err := b.getDevice(dpath)
	if err != nil {
		return dbusutil.ToError(err)
	}
	err = d.core.Trusted().Set(0, trusted)
	if err != nil {
		return dbusutil.ToError(err)
	}
	return nil
}

// GetDevices return all device objects that marshaled by json.
func (b *Bluetooth) GetDevices(apath dbus.ObjectPath) (devicesJSON string, err *dbus.Error) {

	if adapter, ok := b.adapters[apath]; ok {
		logger.Debug("!!!!!!!!!!!!!!GetDevices", adapter.discoveringTimeoutFlag)
		b.devicesLock.Lock()
		if adapter.discoveringTimeoutFlag { //蓝牙设备被清除， 发送备份的蓝牙设备列表
			var result []*backupDevice
			devices := b.backupDevices[apath]
			result = append(result, devices...)
			devicesJSON = marshalJSON(result)

		} else {
			var result []*device

			b.updateconnectState()
			devices := b.devices[apath]
			result = append(result, devices...)
			devicesJSON = marshalJSON(result)
		}
		b.devicesLock.Unlock()
	}
	return
}

// GetAdapters return all adapter objects that marshaled by json.
func (b *Bluetooth) GetAdapters() (adaptersJSON string, err *dbus.Error) {
	adapters := make([]*adapter, 0, len(b.adapters))
	b.adaptersLock.Lock()
	for _, a := range b.adapters {
		adapters = append(adapters, a)
	}
	b.adaptersLock.Unlock()
	adaptersJSON = marshalJSON(adapters)
	return
}

func (b *Bluetooth) RequestDiscovery(apath dbus.ObjectPath) *dbus.Error {
	a, err := b.getAdapter(apath)
	if err != nil {
		return dbusutil.ToError(err)
	}

	if !a.Powered {

		err = fmt.Errorf("'%s' power off", a)
		return dbusutil.ToError(err)
	}

	discovering, err := a.core.Discovering().Get(0)
	if err != nil {
		return dbusutil.ToError(err)
	}

	if discovering {
		// if adapter is discovering now, just return
		return nil
	}

	a.startDiscovery()

	return nil
}

// 检查蓝牙设备是否被飞行模式禁用
func checkAndEnableBluetoothDevice() {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return
	}

	air := airplanemode.NewAirplaneMode(sysBus)
	if air == nil {
		return
	}
	enabled, err := air.BluetoothEnabled().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	// 如果蓝牙设备被禁用，则开启蓝牙设备
	if !enabled {
		err = air.EnableBluetooth(0, true)
		if err != nil {
			return
		}
		// 启动蓝牙设备需要时间
		time.Sleep(time.Second)
	}
	return
}

func (b *Bluetooth) SetAdapterPowered(apath dbus.ObjectPath,
	powered bool) *dbus.Error {

	logger.Debug("SetAdapterPowered", apath, powered)
	a, err := b.getAdapter(apath)
	if err != nil {
		return dbusutil.ToError(err)
	}

	// 如果要手动开启蓝牙，需要判断蓝牙设备是否在飞行模式下被禁用，如果禁用则需要开启
	if powered {
		checkAndEnableBluetoothDevice()
	}

	if !powered {
		b.devicesLock.Lock()
		for _, dobjlist := range b.devices {
			for _, device := range dobjlist {
				//获取每个device的状态，若正在链接或已链接则断开
				if device != nil {
					if device.State == 1 || device.State == 2 {
						go device.Disconnect()
					}
				}

			}
		}
		b.devicesLock.Unlock()
	}

	err = a.core.Powered().Set(0, powered)
	if err != nil {
		logger.Warningf("failed to set %s powered: %v", a, err)
		return dbusutil.ToError(err)

	}
	// save the powered state
	b.config.setAdapterConfigPowered(a.address, powered)
	/*
		if powered {
			err := a.core.Discoverable().Set(0, b.config.Discoverable)
			if err != nil {
				logger.Warningf("failed to set discoverable for %s: %v", a, err)
			}
			err = a.core.StartDiscovery(0)
			if err != nil {
				logger.Warningf("failed to start discovery for %s: %v", a, err)
			} else {
				// start discovering success, reset discovering timer
				a.discoveringTimeout.Reset(defaultDiscoveringTimeout)
			}
			//move reconnect devices into adapter.go when power signal on coming
			//go b.tryConnectPairedDevices()
		}
	*/
	return nil
}

func (b *Bluetooth) SetAdapterAlias(apath dbus.ObjectPath, alias string) *dbus.Error {
	a, err := b.getAdapter(apath)
	if err != nil {
		return dbusutil.ToError(err)
	}

	err = a.core.Alias().Set(0, alias)
	if err != nil {
		logger.Warningf("failed to set %s alias: %v", a, err)
		return dbusutil.ToError(err)
	}

	return nil
}

func (b *Bluetooth) SetAdapterDiscoverable(apath dbus.ObjectPath,
	discoverable bool) *dbus.Error {
	logger.Debug("SetAdapterDiscoverable", apath, discoverable)

	a, err := b.getAdapter(apath)
	if err != nil {
		return dbusutil.ToError(err)
	}

	if !a.Powered {
		err = fmt.Errorf("'%s' power off", a)
		return dbusutil.ToError(err)
	}

	err = a.core.Discoverable().Set(0, discoverable)
	if err != nil {
		logger.Warningf("failed to set %s discoverable: %v", a, err)
		return dbusutil.ToError(err)
	}

	return nil
}

func (b *Bluetooth) SetAdapterDiscovering(apath dbus.ObjectPath,
	discovering bool) *dbus.Error {
	logger.Debug("SetAdapterDiscovering", apath, discovering)

	a, err := b.getAdapter(apath)
	if err != nil {
		return dbusutil.ToError(err)
	}

	if !a.Powered {
		err = fmt.Errorf("'%s' power off", a)
		return dbusutil.ToError(err)
	}

	if discovering {
		a.startDiscovery()
	} else {
		err = a.core.StopDiscovery(0)
		if err != nil {
			logger.Warningf("failed to stop discovery for %s: %v", a, err)
			return dbusutil.ToError(err)
		}
	}

	return nil
}

func (b *Bluetooth) SetAdapterDiscoverableTimeout(apath dbus.ObjectPath,
	discoverableTimeout uint32) *dbus.Error {
	logger.Debug("SetAdapterDiscoverableTimeout", apath, discoverableTimeout)

	a, err := b.getAdapter(apath)
	if err != nil {
		return dbusutil.ToError(err)
	}

	err = a.core.DiscoverableTimeout().Set(0, discoverableTimeout)
	if err != nil {
		logger.Warningf("failed to set %s discoverableTimeout: %v", a, err)
		return dbusutil.ToError(err)
	}

	return nil
}

//Confirm should call when you receive RequestConfirmation signal
func (b *Bluetooth) Confirm(devPath dbus.ObjectPath, accept bool) *dbus.Error {
	logger.Infof("Confirm %q %v", devPath, accept)
	err := b.feed(devPath, accept, "")
	return dbusutil.ToError(err)
}

//FeedPinCode should call when you receive RequestPinCode signal, notice that accept must true
//if you accept connect request. If accept is false, pinCode will be ignored.
func (b *Bluetooth) FeedPinCode(devPath dbus.ObjectPath, accept bool, pinCode string) *dbus.Error {
	logger.Infof("FeedPinCode %q %v %q", devPath, accept, pinCode)
	err := b.feed(devPath, accept, pinCode)
	return dbusutil.ToError(err)
}

//FeedPasskey should call when you receive RequestPasskey signal, notice that accept must true
//if you accept connect request. If accept is false, passkey will be ignored.
//passkey must be range in 0~999999.
func (b *Bluetooth) FeedPasskey(devPath dbus.ObjectPath, accept bool, passkey uint32) *dbus.Error {
	logger.Infof("FeedPasskey %q %v %d", devPath, accept, passkey)
	err := b.feed(devPath, accept, fmt.Sprintf("%06d", passkey))
	return dbusutil.ToError(err)
}

func (b *Bluetooth) DebugInfo() (string, *dbus.Error) {
	info := fmt.Sprintf("adapters: %s\ndevices: %s", marshalJSON(b.adapters), marshalJSON(b.devices))
	return info, nil
}

//ClearUnpairedDevice will remove all device in unpaired list
func (b *Bluetooth) ClearUnpairedDevice() *dbus.Error {
	logger.Debug("ClearUnpairedDevice")
	var removeDevices []*device
	b.devicesLock.Lock()
	for _, devices := range b.devices {
		for _, d := range devices {
			if d != nil && !d.Paired {
				logger.Info("remove unpaired device", d)
				removeDevices = append(removeDevices, d)
			}
		}
	}
	b.devicesLock.Unlock()

	for _, d := range removeDevices {
		b.RemoveDevice(d.AdapterPath, d.Path)
	}
	return nil
}
