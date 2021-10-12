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
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus"
	bluez "github.com/linuxdeepin/go-dbus-factory/org.bluez"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	bluezDBusServiceName      = "org.bluez"
	bluezAdapterDBusInterface = "org.bluez.Adapter1"
	bluezDeviceDBusInterface  = "org.bluez.Device1"

	dbusServiceName = "com.deepin.system.Bluetooth"
	dbusPath        = "/com/deepin/system/Bluetooth"
	dbusInterface   = dbusServiceName
)

const (
	StateUnavailable = 0
	StateAvailable   = 1
	StateConnected   = 2
)

const (
	devIconComputer        = "computer"
	devIconPhone           = "phone"
	devIconModem           = "modem"
	devIconNetworkWireless = "network-wireless"
	devIconAudioCard       = "audio-card"
	devIconCameraVideo     = "camera-video"
	devIconCameraPhoto     = "camera-photo"
	devIconPrinter         = "printer"
	devIconInputGaming     = "input-gaming"
	devIconInputKeyboard   = "input-keyboard"
	devIconInputTablet     = "input-tablet"
	devIconInputMouse      = "input-mouse"
)

//go:generate dbusutil-gen -type SysBluetooth bluetooth.go
//go:generate dbusutil-gen em -type SysBluetooth,agent

type SysBluetooth struct {
	service       *dbusutil.Service
	sigLoop       *dbusutil.SignalLoop
	config        *config
	objectManager bluez.ObjectManager
	sysDBusDaemon ofdbus.DBus
	loginManager  login1.Manager
	agent         *agent
	userAgents    *userAgentMap

	// adapter
	adaptersMu sync.Mutex
	adapters   map[dbus.ObjectPath]*adapter

	// device
	devicesMu sync.Mutex
	devices   map[dbus.ObjectPath][]*device

	// backup device
	backupDevicesMu sync.Mutex
	backupDevices   map[dbus.ObjectPath][]*backupDevice

	acm *autoConnectManager

	PropsMu sync.RWMutex
	State   uint32 // StateUnavailable/StateAvailable/StateConnected

	// 当发起设备连接成功后，应该把连接的设备添加进设备列表
	connectedDevices map[dbus.ObjectPath][]*device
	connectedMu      sync.RWMutex
	//设备被清空后需要连接的设备路径
	prepareToConnectedDevice dbus.ObjectPath
	prepareToConnectedMu     sync.Mutex

	// nolint
	signals *struct {
		// adapter/device properties changed signals
		AdapterAdded, AdapterRemoved, AdapterPropertiesChanged struct {
			adapterJSON string
		}

		DeviceAdded, DeviceRemoved, DevicePropertiesChanged struct {
			devJSON string
		}
	}
}

func newSysBluetooth(service *dbusutil.Service) (b *SysBluetooth) {
	sysBus := service.Conn()
	b = &SysBluetooth{
		service:    service,
		sigLoop:    dbusutil.NewSignalLoop(sysBus, 10),
		userAgents: newUserAgentMap(),
	}

	b.config = newConfig()
	b.loginManager = login1.NewManager(sysBus)
	b.sysDBusDaemon = ofdbus.NewDBus(sysBus)
	b.objectManager = bluez.NewObjectManager(sysBus)
	b.adapters = make(map[dbus.ObjectPath]*adapter)
	b.devices = make(map[dbus.ObjectPath][]*device)
	b.backupDevices = make(map[dbus.ObjectPath][]*backupDevice)
	b.connectedDevices = make(map[dbus.ObjectPath][]*device)
	b.acm = newAutoConnectManager()
	b.acm.connectCb = func(adapterPath, devicePath dbus.ObjectPath, wId int) error {
		adapter, err := b.getAdapter(adapterPath)
		if err != nil {
			// 可能是适配器被移除了，停止连接
			return nil
		}
		if !adapter.Powered {
			// 适配器电源关闭了，停止连接
			return nil
		}

		return b.autoConnectPairedDevice(devicePath, adapterPath)
	}

	return
}

func (b *SysBluetooth) destroy() {
	b.agent.destroy()

	b.objectManager.RemoveAllHandlers()
	b.sysDBusDaemon.RemoveAllHandlers()
	b.loginManager.RemoveAllHandlers()

	b.devicesMu.Lock()
	for _, devices := range b.devices {
		for _, device := range devices {
			device.destroy()
		}
	}
	b.devicesMu.Unlock()

	b.adaptersMu.Lock()
	for _, adapter := range b.adapters {
		adapter.destroy()
	}
	b.adaptersMu.Unlock()

	err := b.service.StopExport(b)
	if err != nil {
		logger.Warning(err)
	}
	b.sigLoop.Stop()
}

func (*SysBluetooth) GetInterfaceName() string {
	return dbusInterface
}

func (b *SysBluetooth) init() {
	b.sigLoop.Start()
	b.config.load()
	b.sysDBusDaemon.InitSignalExt(b.sigLoop, true)
	_, err := b.sysDBusDaemon.ConnectNameOwnerChanged(b.handleDBusNameOwnerChanged)
	if err != nil {
		logger.Warning(err)
	}

	b.loginManager.InitSignalExt(b.sigLoop, true)
	_, err = b.loginManager.ConnectSessionNew(b.handleSessionNew)

	_, err = b.loginManager.ConnectSessionRemoved(func(sessionId string, sessionPath dbus.ObjectPath) {
		logger.Info("session removed", sessionId, sessionPath)
		b.userAgents.removeSession(sessionPath)
	})
	if err != nil {
		logger.Warning(err)
	}

	_, err = b.loginManager.ConnectUserRemoved(func(uid uint32, userPath dbus.ObjectPath) {
		uidStr := strconv.Itoa(int(uid))
		b.userAgents.removeUser(uidStr)
	})

	_, err = b.loginManager.ConnectPrepareForSleep(b.handlePrepareForSleep)
	if err != nil {
		logger.Warning(err)
	}

	b.objectManager.InitSignalExt(b.sigLoop, true)
	_, err = b.objectManager.ConnectInterfacesAdded(b.handleInterfacesAdded)
	if err != nil {
		logger.Warning(err)
	}

	_, err = b.objectManager.ConnectInterfacesRemoved(b.handleInterfacesRemoved)
	if err != nil {
		logger.Warning(err)
	}

	b.agent.init()
	b.loadObjects()

	b.config.clearSpareConfig(b)
	go b.tryConnectPairedDevices("")
}

func (b *SysBluetooth) handleSessionNew(sessionId string, sessionPath dbus.ObjectPath) {
	logger.Info("session added", sessionId, sessionPath)
	sysBus := b.sigLoop.Conn()
	session, err := login1.NewSession(sysBus, sessionPath)
	if err != nil {
		logger.Warning(err)
		return
	}

	userInfo, err := session.User().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	uidStr := strconv.Itoa(int(userInfo.UID))
	if !b.userAgents.hasUser(uidStr) {
		// 不关心这个用户的新 session
		return
	}

	newlyAdded := b.userAgents.addSession(uidStr, session)
	if newlyAdded {
		b.watchSession(uidStr, session)
	}
}

func (b *SysBluetooth) handlePrepareForSleep(beforeSleep bool) {
	if beforeSleep {
		logger.Debug("prepare to sleep")
		return
	}
	logger.Debug("Wakeup from sleep, will set adapter and try connect device")
	time.AfterFunc(time.Second*3, func() {
		//for _, adapter := range b.adapters {
		//	if !adapter.Powered {
		//		continue
		//	}
		//	//_ = adapter.core.Adapter().Discoverable().Set(0, b.config.Discoverable)
		//}
		b.tryConnectPairedDevices("")
	})
}

func (b *SysBluetooth) watchSession(uid string, session login1.Session) {
	session.InitSignalExt(b.sigLoop, true)
	err := session.Active().ConnectChanged(func(hasValue bool, active bool) {
		if !hasValue {
			return
		}
		if active {
			b.userAgents.setActiveUid(uid)
		}
	})

	if err != nil {
		logger.Warning(err)
	}

	active, err := session.Active().Get(0)
	if err != nil {
		logger.Warning(err)
	}
	if active {
		b.userAgents.setActiveUid(uid)
	}
}

func (b *SysBluetooth) unblockBluetoothDevice() {
	has, err := hasBluetoothDeviceBlocked()
	if err != nil {
		logger.Warning(err)
		return
	}
	if has {
		err := exec.Command(rfkillBin, "unblock", rfkillDeviceTypeBluetooth).Run()
		if err != nil {
			logger.Warning(err)
		}
	}
}

func (b *SysBluetooth) loadObjects() {
	// add exists adapters and devices
	objects, err := b.objectManager.GetManagedObjects(0)
	if err != nil {
		logger.Error(err)
		return
	}

	b.unblockBluetoothDevice()
	// add adapters
	for path, obj := range objects {
		if _, ok := obj[bluezAdapterDBusInterface]; ok {
			b.addAdapter(path)
		}
	}

	// then add devices
	for path, obj := range objects {
		if _, ok := obj[bluezDeviceDBusInterface]; ok {
			b.addDevice(path)
		}
	}
}

func (b *SysBluetooth) removeAllObjects() {
	b.devicesMu.Lock()
	for _, devices := range b.devices {
		for _, device := range devices {
			device.notifyDeviceRemoved()
			device.destroy()
		}
	}
	b.devices = make(map[dbus.ObjectPath][]*device)
	b.devicesMu.Unlock()

	b.adaptersMu.Lock()
	for _, adapter := range b.adapters {
		adapter.notifyAdapterRemoved()
		adapter.destroy()
	}
	b.adapters = make(map[dbus.ObjectPath]*adapter)
	b.adaptersMu.Unlock()
}

func (b *SysBluetooth) handleInterfacesAdded(path dbus.ObjectPath, data map[string]map[string]dbus.Variant) {
	if _, ok := data[bluezAdapterDBusInterface]; ok {
		b.unblockBluetoothDevice()
		b.addAdapter(path)
	}
	if _, ok := data[bluezDeviceDBusInterface]; ok {
		b.addDevice(path)
	}
}

func (b *SysBluetooth) handleInterfacesRemoved(path dbus.ObjectPath, interfaces []string) {
	if isStringInArray(bluezAdapterDBusInterface, interfaces) {
		b.removeAdapter(path)
	}
	if isStringInArray(bluezDeviceDBusInterface, interfaces) {
		b.removeDevice(path)
	}
}

func (b *SysBluetooth) handleDBusNameOwnerChanged(name, oldOwner, newOwner string) {
	// if a new dbus session was installed, the name and newOwner
	// will be not empty, if a dbus session was uninstalled, the
	// name and oldOwner will be not empty
	if name == bluezDBusServiceName {
		if newOwner != "" {
			logger.Info("bluetooth is starting")
			time.AfterFunc(1*time.Second, func() {
				b.loadObjects()
				b.agent.registerDefaultAgent()
				b.tryConnectPairedDevices("")
			})
		} else {
			logger.Info("bluetooth stopped")
			b.removeAllObjects()
		}
	} else {
		if strings.HasPrefix(name, ":") && oldOwner != "" && newOwner == "" {
			b.userAgents.handleNameLost(name)
		}
	}
}

func (b *SysBluetooth) addDevice(devPath dbus.ObjectPath) {
	logger.Debug("receive signal device added", devPath)
	if b.isDeviceExists(devPath) {
		return
	}

	d := newDevice(b.sigLoop, devPath)
	b.adaptersMu.Lock()
	d.adapter = b.adapters[d.AdapterPath]
	b.adaptersMu.Unlock()

	if d.adapter == nil {
		logger.Warningf("failed to add device %s, not found adapter", devPath)
		return
	}

	// device detail info is needed to write into config file
	b.config.addDeviceConfig(d)

	b.devicesMu.Lock()
	b.devices[d.AdapterPath] = append(b.devices[d.AdapterPath], d)
	b.devicesMu.Unlock()

	// 设备加入的同时进行备份
	_, idx := b.findBackupDevice(devPath)
	if idx == -1 {
		b.backupDevicesMu.Lock()
		b.backupDevices[d.AdapterPath] = append(b.backupDevices[d.AdapterPath], newBackupDevice(d))
		b.backupDevicesMu.Unlock()
	}

	d.notifyDeviceAdded()

	// 若扫描到需要连接的设备，直接连接
	if b.prepareToConnectedDevice == d.Path {
		go func() {
			err := d.Connect()
			if err != nil {
				logger.Warning(err)
			}
		}()
	}
}

func (b *SysBluetooth) removeDevice(devPath dbus.ObjectPath) {
	logger.Debug("receive signal device removed", devPath)
	adapterPath, idx := b.getDeviceIndex(devPath)
	if idx == -1 {
		logger.Warning("repeat remove device", devPath)
		return
	}

	var removedDev *device
	b.devicesMu.Lock()
	b.devices[adapterPath], removedDev = b.doRemoveDevice(b.devices[adapterPath], idx)
	b.devicesMu.Unlock()

	_, idx = b.findBackupDevice(devPath)
	if idx == -1 {
		// 未找到备份设备，需要发送设备移除信号
		removedDev.notifyDeviceRemoved()
	}
}

func (b *SysBluetooth) doRemoveDevice(devices []*device, i int) ([]*device, *device) {
	// NOTE: do not remove device from config
	d := devices[i]
	d.destroy()
	copy(devices[i:], devices[i+1:])
	devices[len(devices)-1] = nil
	devices = devices[:len(devices)-1]
	return devices, d
}

func (b *SysBluetooth) removeBackupDevice(devPath dbus.ObjectPath) {
	b.backupDevicesMu.Lock()
	defer b.backupDevicesMu.Unlock()

	adapterPath, i := b.findBackupDevice(devPath)
	if i < 0 {
		logger.Debug("repeat remove device", devPath)
		return
	}

	b.backupDevices[adapterPath] = b.doRemoveBackupDevice(b.backupDevices[adapterPath], i)
}

func (b *SysBluetooth) doRemoveBackupDevice(devices []*backupDevice, i int) []*backupDevice {
	copy(devices[i:], devices[i+1:])
	devices[len(devices)-1] = nil
	devices = devices[:len(devices)-1]
	return devices
}

func (b *SysBluetooth) syncCommonToBackupDevices(adapterPath dbus.ObjectPath) {
	logger.Debug("syncCommonToBackupDevices", adapterPath)
	devices := b.getDevices(adapterPath)

	// 转换为 map
	deviceMap := make(map[dbus.ObjectPath]*device)
	for _, d := range devices {
		deviceMap[d.Path] = d
	}

	// 应该只需要关注备份的设备比普通设备多的情况。
	var removedBackupDevices []*backupDevice
	var newBackupDevices []*backupDevice

	b.backupDevicesMu.Lock()
	for _, backupDevice := range b.backupDevices[adapterPath] {
		// 从 devices 能找到，保留，不能找到，则删除
		_, ok := deviceMap[backupDevice.Path]
		if !ok {
			removedBackupDevices = append(removedBackupDevices, backupDevice)
		}
	}

	for _, d := range devices {
		newBackupDevices = append(newBackupDevices, newBackupDevice(d))
	}
	b.backupDevices[adapterPath] = newBackupDevices
	b.backupDevicesMu.Unlock()

	for _, d := range removedBackupDevices {
		d.notifyDeviceRemoved()
	}
}

func (b *SysBluetooth) isDeviceExists(devPath dbus.ObjectPath) bool {
	_, i := b.getDeviceIndex(devPath)
	return i >= 0
}

func (b *SysBluetooth) findDevice(devPath dbus.ObjectPath) (adapterPath dbus.ObjectPath, index int) {
	for p, devices := range b.devices {
		for i, d := range devices {
			if d.Path == devPath {
				return p, i
			}
		}
	}
	return "", -1
}

func (b *SysBluetooth) findBackupDevice(devPath dbus.ObjectPath) (adapterPath dbus.ObjectPath, index int) {
	for p, devices := range b.backupDevices {
		for i, d := range devices {
			if d.Path == devPath {
				return p, i
			}
		}
	}
	return "", -1
}

func (b *SysBluetooth) getDeviceIndex(devPath dbus.ObjectPath) (adapterPath dbus.ObjectPath, index int) {
	b.devicesMu.Lock()
	defer b.devicesMu.Unlock()
	return b.findDevice(devPath)
}

func (b *SysBluetooth) getDevice(devPath dbus.ObjectPath) (*device, error) {
	b.devicesMu.Lock()
	defer b.devicesMu.Unlock()
	adapterPath, index := b.findDevice(devPath)
	if index < 0 {
		return nil, errInvalidDevicePath
	}
	return b.devices[adapterPath][index], nil
}

func (b *SysBluetooth) getBackupDevice(devPath dbus.ObjectPath) (*backupDevice, error) {
	b.backupDevicesMu.Lock()
	defer b.backupDevicesMu.Unlock()
	adapterPath, index := b.findBackupDevice(devPath)
	if index < 0 {
		return nil, errInvalidDevicePath
	}
	return b.backupDevices[adapterPath][index], nil
}

// 更新backupDevices数据
func (b *SysBluetooth) updateBackupDevices(adapterPath dbus.ObjectPath) {
	devices := b.devices[adapterPath]
	for _, device := range devices {
		aPath, index := b.findBackupDevice(device.Path)
		if index < 0 {
			logger.Warning("invalid device path: ", device.Path)
			break
		} else {
			b.backupDevices[aPath][index] = newBackupDevice(device)
		}
	}
}

func (b *SysBluetooth) getAdapterDevices(adapterAddress string) []*device {
	var aPath dbus.ObjectPath
	b.adaptersMu.Lock()
	for adapterPath, adapter := range b.adapters {
		if adapter.Address == adapterAddress {
			aPath = adapterPath
			break
		}
	}
	b.adaptersMu.Unlock()

	if aPath == "" {
		return nil
	}

	b.devicesMu.Lock()
	defer b.devicesMu.Unlock()

	devices := b.devices[aPath]
	if devices == nil {
		return nil
	}

	result := make([]*device, 0, len(devices))
	result = append(result, devices...)
	return result
}

func (b *SysBluetooth) addAdapter(adapterPath dbus.ObjectPath) {
	logger.Debug("receive signal adapter added", adapterPath)
	if b.isAdapterExists(adapterPath) {
		return
	}

	a := newAdapter(b.sigLoop, adapterPath)
	a.bt = b
	// initialize adapter power state
	b.config.addAdapterConfig(a.Address)
	cfgPowered := b.config.getAdapterConfigPowered(a.Address)
	err := a.core.Adapter().Powered().Set(0, cfgPowered)
	if err != nil {
		logger.Warning(err)
	}

	err = a.core.Adapter().DiscoverableTimeout().Set(0, 0)
	if err != nil {
		logger.Warning(err)
	}

	//if cfgPowered {
	//err = a.core.Adapter().Discoverable().Set(0, b.config.Discoverable)
	//if err != nil {
	//	logger.Warning(err)
	//}
	//}

	b.adaptersMu.Lock()
	b.adapters[adapterPath] = a
	b.adaptersMu.Unlock()

	a.notifyAdapterAdded()
	b.acm.addAdapter(adapterPath)
	logger.Debug("addAdapter", adapterPath)
}

func (b *SysBluetooth) removeAdapter(adapterPath dbus.ObjectPath) {
	logger.Debug("receive signal adapter removed", adapterPath)
	b.adaptersMu.Lock()

	if b.adapters[adapterPath] == nil {
		b.adaptersMu.Unlock()
		logger.Warning("repeat remove adapter", adapterPath)
		return
	}

	b.doRemoveAdapter(adapterPath)
	b.adaptersMu.Unlock()

	b.devicesMu.Lock()
	b.devices[adapterPath] = nil
	b.devicesMu.Unlock()
	b.syncCommonToBackupDevices(adapterPath)
}

func (b *SysBluetooth) doRemoveAdapter(adapterPath dbus.ObjectPath) {
	// NOTE: do not remove adapter from config file
	removeAdapter := b.adapters[adapterPath]
	delete(b.adapters, adapterPath)
	removeAdapter.notifyAdapterRemoved()
	removeAdapter.destroy()
	b.acm.removeAdapter(adapterPath)
}

func (b *SysBluetooth) getAdapter(adapterPath dbus.ObjectPath) (adapter *adapter, err error) {
	b.adaptersMu.Lock()
	defer b.adaptersMu.Unlock()

	adapter = b.adapters[adapterPath]
	if adapter == nil {
		err = fmt.Errorf("adapter not exists %s", adapterPath)
		logger.Error(err)
		return
	}
	return
}

func (b *SysBluetooth) isAdapterExists(adapterPath dbus.ObjectPath) bool {
	b.adaptersMu.Lock()
	defer b.adaptersMu.Unlock()
	return b.adapters[adapterPath] != nil
}

func (b *SysBluetooth) updateState() {
	newState := StateUnavailable
	if len(b.adapters) > 0 {
		newState = StateAvailable
	}

	for _, devices := range b.devices {
		for _, d := range devices {
			if d.connected && d.Paired {
				newState = StateConnected
				break
			}
		}
	}

	b.PropsMu.Lock()
	b.setPropState(uint32(newState))
	b.PropsMu.Unlock()
}

func (b *SysBluetooth) getAdapters() []*adapter {
	b.adaptersMu.Lock()
	defer b.adaptersMu.Unlock()

	result := make([]*adapter, 0, len(b.adapters))
	for _, adapter := range b.adapters {
		result = append(result, adapter)
	}
	return result
}

func (b *SysBluetooth) getDevices(adapterPath dbus.ObjectPath) []*device {
	b.devicesMu.Lock()
	defer b.devicesMu.Unlock()

	devices := b.devices[adapterPath]
	result := make([]*device, len(devices))
	copy(result, devices)
	return result
}

func (b *SysBluetooth) tryConnectPairedDevices(adapterPath dbus.ObjectPath) {
	inputOnly := true
	connectDuration := 2 * time.Minute
	if b.getActiveUserAgent() != nil {
		// 表示用户已经登录
		inputOnly = false
	}
	logger.Debugf("tryConnectPairedDevices adapterPath: %q, inputOnly: %v", adapterPath, inputOnly)
	adapterDevicesMap := b.getPairedAndNotConnectedDevices(adapterPath)

	for adapterPath, devices := range adapterDevicesMap {
		if inputOnly {
			// 登录界面，只需要输入设备
			devices = filterOutDevices(devices, func(device *device) bool {
				return strings.HasPrefix(device.Icon, "input")
			})
		}
		logger.Debug("before soft devices:", devices)
		b.config.softDevices(devices)
		logger.Debug("after soft devices:", devices)
		var deviceInfos []autoDeviceInfo
		priority := 0
		now := time.Now()
		for _, d := range devices {
			logger.Debug("try auto connect", d.String())
			deviceInfos = append(deviceInfos, autoDeviceInfo{
				adapter:       adapterPath,
				device:        d.Path,
				alias:         d.Alias,
				priority:      priority,
				retryDeadline: now.Add(connectDuration),
			})
			priority++
		}
		b.acm.addDevices(adapterPath, deviceInfos, now)
	}
}

func (b *SysBluetooth) autoConnectPairedDevice(devPath dbus.ObjectPath, adapterPath dbus.ObjectPath) error {
	device, err := b.getDevice(devPath)
	if err != nil {
		return err
	}

	// if device using LE mode, will suspend, try to connect it should be failed, filter it.
	if !b.isBREDRDevice(device) {
		logger.Debugf("%v using LE mode, do not auto connect it", device)
		return nil
	}

	switch device.Icon {
	// 只自动连接一个这些图标的设备
	case devIconAudioCard, devIconInputKeyboard, devIconInputMouse, devIconInputTablet:
		connectedDevice := b.findFirstConnectedDeviceByIcon(device.Icon)
		if connectedDevice != nil {
			logger.Debugf("there is already a connected %v, icon: %v, do not auto connect it: %v",
				connectedDevice, device.Icon, device)
			return nil
		}
	}

	logger.Debug("auto connect paired", device)
	err = device.doConnect(false)
	if err != nil {
		logger.Debugf("failed to auto connect %v: %v", device, err)
		// 设置 connect phase 可以造成界面上一直在连接，而不中断的情况。
		device.setConnectPhase(connectPhaseConnectFailedSleep)
		time.Sleep(3 * time.Second)
		device.setConnectPhase(connectPhaseNone)
	}
	return err
}

// 当 adapterPath 为空时，获取所有适配器的配对，但未连接的设备。
// 当 adapterPath 不为空时，或者获取指定适配器的配对，但未连接的设备。
func (b *SysBluetooth) getPairedAndNotConnectedDevices(adapterPath dbus.ObjectPath) map[dbus.ObjectPath][]*device {
	adapters := b.getAdapters()
	if adapterPath != "" {
		var theAdapter *adapter
		for _, adapter := range adapters {
			if adapter.Path == adapterPath {
				theAdapter = adapter
				break
			}
		}
		if theAdapter == nil {
			return nil
		}
		adapters = []*adapter{theAdapter}
	}
	result := make(map[dbus.ObjectPath][]*device)
	for _, adapter := range adapters {
		// 当适配器打开电源时才自动连接
		if !adapter.Powered {
			continue
		}
		devices := b.getDevices(adapter.Path)
		var tmpDevices []*device
		for _, d := range devices {
			if d != nil && d.Paired && !d.connected {
				tmpDevices = append(tmpDevices, d)
			}
		}

		result[adapter.Path] = tmpDevices
	}

	return result
}

// 判断设备是否为经典蓝牙设备
func (b *SysBluetooth) isBREDRDevice(dev *device) bool {
	technologies, err := dev.getTechnologies()
	if err != nil {
		logger.Warningf("failed to get device(%s -- %s) technologies: %v",
			dev.adapter.Address, dev.Address, err)
		return false
	}
	for _, tech := range technologies {
		if tech == "BR/EDR" {
			return true
		}
	}
	return false
}

func (b *SysBluetooth) addConnectedDevice(connectedDev *device) {
	b.connectedMu.Lock()
	b.connectedDevices[connectedDev.AdapterPath] = append(b.connectedDevices[connectedDev.AdapterPath], connectedDev)
	b.connectedMu.Unlock()
}

func (b *SysBluetooth) removeConnectedDevice(disconnectedDev *device) {
	b.connectedMu.Lock()
	// check if dev exist in connectedDevices map, if exist, remove device
	if connectedDevices, ok := _bt.connectedDevices[disconnectedDev.AdapterPath]; ok {
		var tempDevices []*device
		for _, dev := range connectedDevices {
			// check if disconnected device exist in connected devices, if exist, abandon this
			if dev.Address != disconnectedDev.Address {
				tempDevices = append(tempDevices, dev)
			}
		}
		_bt.connectedDevices[disconnectedDev.AdapterPath] = tempDevices
	}
	b.connectedMu.Unlock()
}

func (b *SysBluetooth) getConnectedDeviceByAddress(address string) *device {
	b.connectedMu.Lock()
	defer b.connectedMu.Unlock()

	for _, devices := range b.connectedDevices {
		for _, dev := range devices {
			if dev.Address == address {
				return dev
			}
		}
	}

	return nil
}

func (b *SysBluetooth) findFirstConnectedDeviceByIcon(icon string) *device {
	b.connectedMu.Lock()
	defer b.connectedMu.Unlock()

	for _, devices := range b.connectedDevices {
		for _, dev := range devices {
			if dev.Icon == icon {
				return dev
			}
		}
	}
	return nil
}
