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

package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	mmdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.modemmanager1"
	nmdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.networkmanager"

	"pkg.deepin.io/dde/daemon/network/nm"
	dbus "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

type device struct {
	nmDev      *nmdbus.Device
	mmDevModem *mmdbus.Modem
	nmDevType  uint32
	id         string
	udi        string

	Path          dbus.ObjectPath
	State         uint32
	Interface     string
	ClonedAddress string
	HwAddress     string
	Driver        string
	Managed       bool

	// Vendor is the device vendor ID and product ID, if failed, use
	// interface name instead. BTW, we use Vendor instead of
	// Description as the name to keep compatible with the old
	// front-end code.
	Vendor string

	// Unique connection uuid for this device, works for wired,
	// wireless and modem devices, for wireless device the unique uuid
	// will be the connection uuid of hotspot mode.
	UniqueUuid string

	UsbDevice bool // not works for mobile device(modem)

	// used for wireless device
	ActiveAp       dbus.ObjectPath
	SupportHotspot bool

	// used for mobile device
	MobileNetworkType   string
	MobileSignalQuality uint32
}

func (m *Manager) initDeviceManage() {
	m.devicesLock.Lock()
	m.devices = make(map[string][]*device)
	m.devicesLock.Unlock()

	m.accessPointsLock.Lock()
	m.accessPoints = make(map[dbus.ObjectPath][]*accessPoint)
	m.accessPointsLock.Unlock()

	_, err := nmManager.ConnectDeviceAdded(func(path dbus.ObjectPath) {
		m.addDevice(path)
	})
	if err != nil {
		logger.Warning(err)
	}
	_, err = nmManager.ConnectDeviceRemoved(func(path dbus.ObjectPath) {
		notifyDeviceRemoved(path)
		m.removeDevice(path)
	})
	if err != nil {
		logger.Warning(err)
	}
	for _, path := range nmGetDevices() {
		m.addDevice(path)
	}
}

func (m *Manager) newDevice(devPath dbus.ObjectPath) (dev *device, err error) {
	nmDev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}

	// ignore virtual network interfaces
	if isVirtualDeviceIfc(nmDev) {
		driver, _ := nmDev.Driver().Get(0)
		err = fmt.Errorf("ignore virtual network interface which driver is %s %s", driver, devPath)
		logger.Info(err)
		return
	}

	devType, _ := nmDev.DeviceType().Get(0)
	if !isDeviceTypeValid(devType) {
		err = fmt.Errorf("ignore invalid device type %d", devType)
		logger.Info(err)
		return
	}

	dev = &device{
		nmDev:     nmDev,
		nmDevType: devType,
		Path:      nmDev.Path_(),
	}
	dev.udi, _ = nmDev.Udi().Get(0)
	dev.Interface, _ = nmDev.Interface().Get(0)
	dev.Driver, _ = nmDev.Driver().Get(0)

	dev.Managed = nmGeneralIsDeviceManaged(devPath)
	dev.Vendor = nmGeneralGetDeviceDesc(devPath)
	dev.UsbDevice = nmGeneralIsUsbDevice(devPath)
	dev.id, _ = nmGeneralGetDeviceIdentifier(devPath)
	dev.UniqueUuid = nmGeneralGetDeviceUniqueUuid(devPath)

	nmDev.InitSignalExt(m.sysSigLoop, true)

	//add device state change signal 
	err=nmDev.State().ConnectChanged(func(hasValue bool, value uint32) {
		dev.State,_ = nmDev.State().Get(0)
		m.updatePropDevices()
	})

	// dispatch for different device types
	switch dev.nmDevType {
	case nm.NM_DEVICE_TYPE_ETHERNET:
		// for mac address clone
		nmDevWired := nmDev.Wired()
		err = nmDevWired.HwAddress().ConnectChanged(func(hasValue bool, value string) {
			if !hasValue {
				return
			}
			if value == dev.ClonedAddress {
				return
			}
			dev.ClonedAddress = value
			m.updatePropDevices()
		})
		if err != nil {
			logger.Warning(err)
		}
		dev.ClonedAddress, _ = nmDevWired.HwAddress().Get(0)
		dev.HwAddress, _ = nmDevWired.PermHwAddress().Get(0)

		if dev.HwAddress == "" {
			dev.HwAddress = dev.ClonedAddress
		}

		if nmHasSystemSettingsModifyPermission() {
			carrierChanged := func(hasValue, value bool) {
				if !hasValue || !value {
					return
				}

				logger.Info("wired plugin", dev.Path)
				logger.Debug("ensure wired connection exists", dev.Path)
				_, _, err = m.ensureWiredConnectionExists(dev.Path, true)
				if err != nil {
					logger.Warning(err)
				}
			}

			nmDev.Wired().Carrier().ConnectChanged(carrierChanged)

			carrier, _ := nmDev.Wired().Carrier().Get(0)
			carrierChanged(true, carrier)
		} else {
			logger.Debug("do not have modify permission")
		}
	case nm.NM_DEVICE_TYPE_WIFI:
		nmDevWireless := nmDev.Wireless()
		dev.ClonedAddress, _ = nmDevWireless.HwAddress().Get(0)
		dev.HwAddress, _ = nmDevWireless.PermHwAddress().Get(0)

		// connect property, about wireless active access point
		err = nmDevWireless.ActiveAccessPoint().ConnectChanged(func(hasValue bool,
			value dbus.ObjectPath) {
			if !hasValue {
				return
			}
			if !m.isDeviceExists(devPath) {
				return
			}
			m.devicesLock.Lock()
			defer m.devicesLock.Unlock()
			dev.ActiveAp = value
			m.updatePropDevices()

			// Re-active connection if wireless 'ActiveAccessPoint' not equal active connection 'SpecificObject'
			// such as wifi roaming, but the active connection state is activated
			err := m.wirelessReActiveConnection(nmDev)
			if err != nil {
				logger.Warning("Failed to re-active connection:", err)
			}
		})
		if err != nil {
			logger.Warning(err)
		}
		dev.ActiveAp, _ = nmDevWireless.ActiveAccessPoint().Get(0)
		permHwAddress, _ := nmDevWireless.PermHwAddress().Get(0)
		dev.SupportHotspot = isWirelessDeviceSupportHotspot(permHwAddress)

		err = nmDevWireless.HwAddress().ConnectChanged(func(hasValue bool, value string) {
			if !hasValue {
				return
			}
			if value == dev.ClonedAddress {
				return
			}
			dev.ClonedAddress = value
			m.updatePropDevices()
		})
		if err != nil {
			logger.Warning(err)
		}
		// connect signals AccessPointAdded() and AccessPointRemoved()
		_, err = nmDevWireless.ConnectAccessPointAdded(func(apPath dbus.ObjectPath) {
			m.addAccessPoint(dev.Path, apPath)
		})
		if err != nil {
			logger.Warning(err)
		}

		_, err = nmDevWireless.ConnectAccessPointRemoved(func(apPath dbus.ObjectPath) {
			if dev.State == nm.NM_DEVICE_STATE_UNMANAGED {
				//在待机时，不能删除全部热点，唤醒时再清除
				return
			}
			m.removeAccessPoint(dev.Path, apPath)
		})
		if err != nil {
			logger.Warning(err)
		}
		for _, apPath := range nmGetAccessPoints(dev.Path) {
			m.addAccessPoint(dev.Path, apPath)
		}
	case nm.NM_DEVICE_TYPE_MODEM:
		if len(dev.id) == 0 {
			// some times, modem device will not be identified
			// successful for battery issue, so check and ignore it
			// here
			err = fmt.Errorf("modem device is not properly identified, please re-plugin it")
			return
		}
		go func() {
			// disable autoconnect property for mobile devices
			// notice: sleep is necessary seconds before setting dbus values
			// FIXME: seems network-manager will restore Autoconnect's value some times
			time.Sleep(3 * time.Second)
			nmSetDeviceAutoconnect(dev.Path, false)
		}()
		if mmDevModem, err := mmNewModem(dbus.ObjectPath(dev.udi)); err == nil {
			mmDevModem.InitSignalExt(m.sysSigLoop, true)
			dev.mmDevModem = mmDevModem

			// connect properties
			err = dev.mmDevModem.AccessTechnologies().ConnectChanged(func(hasValue bool,
				value uint32) {
				if !m.isDeviceExists(devPath) {
					return
				}
				if !hasValue {
					return
				}
				m.devicesLock.Lock()
				defer m.devicesLock.Unlock()
				dev.MobileNetworkType = mmDoGetModemMobileNetworkType(value)
				m.updatePropDevices()
			})
			if err != nil {
				logger.Warning(err)
			}
			accessTech, _ := mmDevModem.AccessTechnologies().Get(0)
			dev.MobileNetworkType = mmDoGetModemMobileNetworkType(accessTech)

			err = dev.mmDevModem.SignalQuality().ConnectChanged(func(hasValue bool,
				value mmdbus.ModemSignalQuality) {
				if !m.isDeviceExists(devPath) {
					return
				}
				if !hasValue {
					return
				}

				m.devicesLock.Lock()
				defer m.devicesLock.Unlock()
				dev.MobileSignalQuality = value.Quality
				m.updatePropDevices()
			})
			if err != nil {
				logger.Warning(err)
			}
			dev.MobileSignalQuality = mmDoGetModemDeviceSignalQuality(mmDevModem)
		}
	}

	// connect signals
	_, err = dev.nmDev.ConnectStateChanged(func(newState uint32, oldState uint32, reason uint32) {
		logger.Debugf("device state changed, %d => %d, reason[%d] %s",
			oldState, newState, reason, deviceErrorTable[reason])

		if !m.isDeviceExists(devPath) {
			return
		}
		dev.State = newState
		m.devicesLock.Lock()
		m.updatePropDevices()
		m.devicesLock.Unlock()

	})
	if err != nil {
		logger.Warning(err)
	}

	err = dev.nmDev.Interface().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}

		dev.Interface = value
		m.devicesLock.Lock()
		m.updatePropDevices()
		m.devicesLock.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = dev.nmDev.Managed().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}

		dev.Managed = value
		m.devicesLock.Lock()
		m.updatePropDevices()
		m.devicesLock.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}

	// 后端创建Device的时候如果networkmanager发生了状态变迁，不会被后端感知
	// 因为device还没有创建完成，也不会给前端发送信号
	// 创建完成之前手动同步，完成之后由信号感知
	dev.State, _ = nmDev.State().Get(0)
	return
}

func (m *Manager) destroyDevice(dev *device) {
	// destroy object to reset all property connects
	if dev.mmDevModem != nil {
		mmDestroyModem(dev.mmDevModem)
	}
	nmDestroyDevice(dev.nmDev)
}

func (m *Manager) clearDevices() {
	m.devicesLock.Lock()
	defer m.devicesLock.Unlock()
	for _, devs := range m.devices {
		for _, dev := range devs {
			m.destroyDevice(dev)
		}
	}
	m.devices = make(map[string][]*device)
	m.updatePropDevices()
}

func (m *Manager) addDevice(devPath dbus.ObjectPath) {
	if m.isDeviceExists(devPath) {
		logger.Warning("device already exists", devPath)
		return
	}

	m.devicesLock.Lock()
	defer m.devicesLock.Unlock()
	dev, err := m.newDevice(devPath)
	if err != nil {
		return
	}
	logger.Debug("add device", devPath)
	devType := getCustomDeviceType(dev.nmDevType)
	m.devices[devType] = append(m.devices[devType], dev)
	m.updatePropDevices()
}

func (m *Manager) removeDevice(devPath dbus.ObjectPath) {
	if !m.isDeviceExists(devPath) {
		return
	}
	devType, i := m.getDeviceIndex(devPath)

	m.devicesLock.Lock()
	defer m.devicesLock.Unlock()
	m.devices[devType] = m.doRemoveDevice(m.devices[devType], i)
	m.updatePropDevices()
}
func (m *Manager) doRemoveDevice(devs []*device, i int) []*device {
	logger.Infof("remove device %#v", devs[i])
	m.destroyDevice(devs[i])
	copy(devs[i:], devs[i+1:])
	devs[len(devs)-1] = nil
	devs = devs[:len(devs)-1]
	return devs
}

func (m *Manager) getDevice(devPath dbus.ObjectPath) (dev *device) {
	devType, i := m.getDeviceIndex(devPath)
	if i < 0 {
		return
	}

	m.devicesLock.Lock()
	defer m.devicesLock.Unlock()
	return m.devices[devType][i]
}
func (m *Manager) isDeviceExists(devPath dbus.ObjectPath) bool {
	_, i := m.getDeviceIndex(devPath)
	if i >= 0 {
		return true
	}
	return false
}
func (m *Manager) getDeviceIndex(devPath dbus.ObjectPath) (devType string, index int) {
	m.devicesLock.Lock()
	defer m.devicesLock.Unlock()
	for t, devs := range m.devices {
		for i, dev := range devs {
			if dev.Path == devPath {
				return t, i
			}
		}
	}
	return "", -1
}

func (m *Manager) IsDeviceEnabled(devPath dbus.ObjectPath) (bool, *dbus.Error) {
	b, err := m.sysNetwork.IsDeviceEnabled(0, string(devPath))
	return b, dbusutil.ToError(err)
}

// 飞行模式下，第一次wifi回连失败，则去10s轮询是否有可用的热点，然后继续回连
func (m *Manager) doAutoConnect(devPath dbus.ObjectPath) {
	for i := 0; i < 10; i++ {
		// 每次休眠一秒再去获取
		time.Sleep(time.Second)
		// 获取当前device的状态
		dev := m.getDevice(devPath)
		if dev == nil {
			return
		}
		// wifi设备是否处于可连接而未连接状态，那么可以连接
		if dev.State == nm.NM_DEVICE_STATE_DISCONNECTED {
			err := m.enableDevice(devPath, true,true)
			if err == nil {
				// 连接成功，退出
				return
			}
		} else {
			return
		}
	}
}

func (m *Manager) EnableDevice(devPath dbus.ObjectPath, enabled bool) *dbus.Error {
	logger.Info("call EnableDevice in session", devPath, enabled)
	err := m.enableDevice(devPath, enabled,true)
	
	/*
	飞行模式没有调用此处的接口，屏蔽此概率可能会导致wifi开关异常代码
	// 特殊情况：飞行模式开启和关闭的时候，开启wifi模块，会出现回连失败
	
	if err != nil {
		// 回连失败，起个线程，在未来的10s内自动回连
		if enabled{
			go m.doAutoConnect(devPath)
		}
	}
	*/
	return dbusutil.ToError(err)
}

func (m *Manager) enableDevice(devPath dbus.ObjectPath, enabled bool, activate bool) (err error) {
	cpath, err := m.sysNetwork.EnableDevice(0, string(devPath), enabled)
	if err != nil {
		return
	}
	// check if need activate connection
	if enabled && activate {
		var uuid string
		//回连之前获取回连热点数据,若没有,则直接返回
		_,err =nmGetConnectionData(cpath)
		if err != nil {
			return nil
		}
		uuid, err = nmGetConnectionUuid(cpath)
		if err != nil {
			return
		}
		m.ActivateConnection(uuid, devPath)
	}

	// set enable device state
	m.setDeviceEnabled(enabled, devPath)
	return
}

func (m *Manager) setDeviceEnabled(enabled bool, devPath dbus.ObjectPath) {
	m.stateHandler.locker.Lock()
	defer m.stateHandler.locker.Unlock()
	dsi, ok := m.stateHandler.devices[devPath]
	if !ok {
		return
	}
	dsi.enabled = enabled
	return
}

func (m *Manager) getDeviceEnabled(devPath dbus.ObjectPath) (bool, error) {
	m.stateHandler.locker.Lock()
	defer m.stateHandler.locker.Unlock()
	dsi, ok := m.stateHandler.devices[devPath]
	if !ok {
		return false, errors.New("device path not exist")
	}
	return dsi.enabled, nil
}

// SetDeviceManaged set target device managed or unmnaged from
// NetworkManager, and a little difference with other interface is
// that devPathOrIfc could be a device DBus path or the device
// interface name.
func (m *Manager) SetDeviceManaged(devPathOrIfc string, managed bool) *dbus.Error {
	err := m.setDeviceManaged(devPathOrIfc, managed)
	return dbusutil.ToError(err)
}

func (m *Manager) setDeviceManaged(devPathOrIfc string, managed bool) (err error) {
	var devPath dbus.ObjectPath
	if strings.HasPrefix(devPathOrIfc, "/org/freedesktop/NetworkManager/Devices") {
		devPath = dbus.ObjectPath(devPathOrIfc)
	} else {
		m.devicesLock.Lock()
		defer m.devicesLock.Unlock()
	out:
		for _, devs := range m.devices {
			for _, dev := range devs {
				if dev.Interface == devPathOrIfc {
					devPath = dev.Path
					break out
				}
			}
		}
	}
	if len(devPath) > 0 {
		err = nmSetDeviceManaged(devPath, managed)
	} else {
		err = fmt.Errorf("invalid device identifier: %s", devPathOrIfc)
		logger.Error(err)
	}
	return
}

// ListDeviceConnections return the available connections for the device
func (m *Manager) ListDeviceConnections(devPath dbus.ObjectPath) ([]dbus.ObjectPath, *dbus.Error) {
	paths, err := m.listDeviceConnections(devPath)
	return paths, dbusutil.ToError(err)
}

func (m *Manager) listDeviceConnections(devPath dbus.ObjectPath) ([]dbus.ObjectPath, error) {
	nmDev, err := nmNewDevice(devPath)
	if err != nil {
		return nil, err
	}

	// ignore virtual network interfaces
	if isVirtualDeviceIfc(nmDev) {
		driver, _ := nmDev.Driver().Get(0)
		err = fmt.Errorf("ignore virtual network interface which driver is %s %s", driver, devPath)
		logger.Info(err)
		return nil, err
	}

	devType, _ := nmDev.DeviceType().Get(0)
	if !isDeviceTypeValid(devType) {
		err = fmt.Errorf("ignore invalid device type %d", devType)
		logger.Info(err)
		return nil, err
	}

	availableConnections, _ := nmDev.AvailableConnections().Get(0)
	return availableConnections, nil
}

// RequestWirelessScan request all wireless devices re-scan access point list.
func (m *Manager) RequestWirelessScan() *dbus.Error {
	m.devicesLock.Lock()
	wirelessAccessPoints := make(map[dbus.ObjectPath][]*accessPoint)
	if devices, ok := m.devices[deviceWifi]; ok {
		for _, dev := range devices {
			err := dev.nmDev.RequestScan(0, nil)
			if err != nil {
				logger.Debug(err)
			}
			//每一次扫描就获取每个热点是否存在相对应的uuid配置
			accessPoints := m.getAccessPointsByPath(dev.Path)
			wirelessconnections := m.connections[deviceWifi]
			for _, accessPoint := range accessPoints {
				for _, connectiondata := range wirelessconnections {
					if connectiondata.Ssid == accessPoint.Ssid {
						accessPoint.Uuid = connectiondata.Uuid
					}

				}
			}
			wirelessAccessPoints[dev.Path] = accessPoints
		}
	}
	m.devicesLock.Unlock()
	wirelessAccessPointsJson, err := json.Marshal(wirelessAccessPoints)
	if err != nil {
		logger.Warning(err)
	}
	wirelessAccessPointsJsonStr := string(wirelessAccessPointsJson)
	m.PropsMu.Lock()
	m.WirelessAccessPoints = wirelessAccessPointsJsonStr
	m.PropsMu.Unlock()
	m.emitPropChangedWirelessAccessPoints(wirelessAccessPointsJsonStr)
	return nil
}

func (m *Manager) getAccessPointsByPath(path dbus.ObjectPath) []*accessPoint {
	m.accessPointsLock.Lock()
	defer m.accessPointsLock.Unlock()
	accessPoints := m.accessPoints[path]
	var filteredAccessPoints []*accessPoint
	for _, ap := range accessPoints {
		if !ap.shouldBeIgnore() {
			filteredAccessPoints = append(filteredAccessPoints, ap)
		}
	}
	return filteredAccessPoints
}

func (m *Manager) wirelessReActiveConnection(nmDev *nmdbus.Device) error {
	wireless := nmDev.Wireless()
	apPath, err := wireless.ActiveAccessPoint().Get(0)
	if err != nil {
		return err
	}
	if apPath == "/" {
		logger.Debug("Invalid active access point path:", nmDev.Path_())
		return nil
	}
	connPath, err := nmDev.ActiveConnection().Get(0)
	if err != nil {
		return err
	}
	if connPath == "/" {
		logger.Debug("Invalid active connection path:", nmDev.Path_())
		return nil
	}

	connObj, err := nmNewActiveConnection(connPath)
	if err != nil {
		return err
	}

	// check network connectivity state
	state, err := connObj.State().Get(0)
	if err != nil {
		return err
	}
	if state != nm.NM_ACTIVE_CONNECTION_STATE_ACTIVATED {
		logger.Debug("[Inactive] re-active connection not activated:", connPath, nmDev.Path_(), state)
		return nil
	}

	spePath, err := connObj.SpecificObject().Get(0)
	if err != nil {
		return err
	}
	if spePath == "/" {
		logger.Debug("Invalid specific access point path:", connObj.Path_(), nmDev.Path_())
		return nil
	}

	if string(apPath) == string(spePath) {
		logger.Debug("[NONE] re-active connection not changed:", connPath, spePath, nmDev.Path_())
		return nil
	}

	ip4Path, _ := connObj.Ip4Config().Get(0)
	if m.checkGatewayConnectivity(ip4Path) {
		logger.Debug("Network is connectivity, don't re-active")
		return nil
	}

	settingsPath, err := connObj.Connection().Get(0)
	if err != nil {
		return err
	}
	logger.Debug("[DO] re-active connection:", settingsPath, connPath, spePath, nmDev.Path_())
	_, err = nmActivateConnection(settingsPath, nmDev.Path_())
	return err
}

func (m *Manager) checkGatewayConnectivity(ipPath dbus.ObjectPath) bool {
	if ipPath == "/" {
		return false
	}

	addr, mask, gateways, domains := nmGetIp4ConfigInfo(ipPath)
	logger.Debugf("The active connection ip4 info: address(%s), mask(%s), gateways(%v), domains(%v)",
		addr, mask, gateways, domains)
	// check whether the gateway is connected by ping
	for _, gw := range gateways {
		if len(gw) == 0 {
			continue
		}
		if !m.doPing(gw, 3) {
			return false
		}
	}
	return true
}

func (m *Manager) doPing(addr string, retries int) bool {
	for i := 0; i < retries; i++ {
		err := m.sysNetwork.Ping(0, addr)
		if err == nil {
			return true
		}
		logger.Warning("Failed to ping gateway:", i, addr, err)
		time.Sleep(time.Millisecond * 500)
	}
	return false
}
