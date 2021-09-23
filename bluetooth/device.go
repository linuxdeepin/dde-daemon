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
	"sync"
	"time"

	dbus "github.com/godbus/dbus"
	bluez "github.com/linuxdeepin/go-dbus-factory/org.bluez"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/proxy"
)

const (
	deviceStateDisconnected = 0
	// device state is connecting or disconnecting, mark them as device state doing
	deviceStateConnecting    = 1
	deviceStateConnected     = 2
	deviceStateDisconnecting = 3
)

const (
	resourceUnavailable = "Resource temporarily unavailable"
)

type deviceState uint32

func (s deviceState) String() string {
	switch s {
	case deviceStateDisconnected:
		return "Disconnected"
	case deviceStateConnecting:
		return "Connecting"
	case deviceStateConnected:
		return "Connected"
	case deviceStateDisconnecting:
		return "Disconnecting"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

var (
	errInvalidDevicePath = fmt.Errorf("invalid device path")
)

type device struct {
	core    bluez.Device
	adapter *adapter

	Path        dbus.ObjectPath
	AdapterPath dbus.ObjectPath

	Alias            string
	Trusted          bool
	Paired           bool
	State            deviceState
	ServicesResolved bool
	ConnectState     bool

	// optional
	UUIDs   []string
	Name    string
	Icon    string
	RSSI    int16
	Address string

	connected         bool
	connectedTime     time.Time
	retryConnectCount int
	agentWorking      bool
	needNotify        bool

	connectPhase      connectPhase
	disconnectPhase   disconnectPhase
	disconnectChan    chan struct{}
	mu                sync.Mutex
	pairingFailedTime time.Time

	// mark if pc or mobile request a connection
	// if is pc, then do not need to show notification window
	// else show notification window
	isInitiativeConnect bool
	// remove device when device state is connecting or disconnecting may cause blueZ crash
	// to avoid this situation, remove device only allowed when connected or disconnected finished
	needRemove bool
	removeLock sync.Mutex
}

//设备的备份，扫描结束3分钟后保存设备
type backupDevice struct {
	Path        dbus.ObjectPath
	AdapterPath dbus.ObjectPath

	Alias            string
	Trusted          bool
	Paired           bool
	State            deviceState
	ServicesResolved bool
	ConnectState     bool

	// optional
	UUIDs   []string
	Name    string
	Icon    string
	RSSI    int16
	Address string

	connected bool
}

type connectPhase uint32

const (
	connectPhaseNone = iota
	connectPhaseStart
	connectPhasePairStart
	connectPhasePairEnd
	connectPhaseConnectProfilesStart
	connectPhaseConnectProfilesEnd
)

type disconnectPhase uint32

const (
	disconnectPhaseNone = iota
	disconnectPhaseStart
	disconnectPhaseDisconnectStart
	disconnectPhaseDisconnectEnd
)

func (d *device) setDisconnectPhase(value disconnectPhase) {
	d.mu.Lock()
	d.disconnectPhase = value
	d.mu.Unlock()

	switch value {
	case disconnectPhaseDisconnectStart:
		logger.Debugf("%s disconnect start", d)
	case disconnectPhaseDisconnectEnd:
		logger.Debugf("%s disconnect end", d)
	}
	d.updateState()
	d.notifyDevicePropertiesChanged()
}

func (d *device) getDisconnectPhase() disconnectPhase {
	d.mu.Lock()
	value := d.disconnectPhase
	d.mu.Unlock()
	return value
}

func (d *device) setConnectPhase(value connectPhase) {
	d.mu.Lock()
	d.connectPhase = value
	d.mu.Unlock()

	switch value {
	case connectPhasePairStart:
		logger.Debugf("%s pair start", d)
	case connectPhasePairEnd:
		logger.Debugf("%s pair end", d)

	case connectPhaseConnectProfilesStart:
		logger.Debugf("%s connect profiles start", d)
	case connectPhaseConnectProfilesEnd:
		logger.Debugf("%s connect profiles end", d)
	}

	d.updateState()
	d.notifyDevicePropertiesChanged()
	if d.Paired && d.State == deviceStateConnected && d.ConnectState && d.needNotify {
		d.needNotify = false
		notifyConnected(d.Alias)
	}
}

func (d *device) getConnectPhase() connectPhase {
	d.mu.Lock()
	value := d.connectPhase
	d.mu.Unlock()
	return value
}

func (d *device) agentWorkStart() {
	logger.Debugf("%s agent work start", d)
	d.mu.Lock()
	d.agentWorking = true
	d.mu.Unlock()
	d.updateState()
	d.notifyDevicePropertiesChanged()
}

func (d *device) agentWorkEnd() {
	logger.Debugf("%s agent work end", d)
	d.mu.Lock()
	d.agentWorking = false
	d.mu.Unlock()
	d.updateState()
	d.notifyDevicePropertiesChanged()
}

func (d *device) String() string {
	return fmt.Sprintf("device [%s] %s", d.Address, d.Alias)
}

func newDevice(systemSigLoop *dbusutil.SignalLoop, dpath dbus.ObjectPath) (d *device) {
	d = &device{Path: dpath}
	systemConn := systemSigLoop.Conn()
	d.core, _ = bluez.NewDevice(systemConn, dpath)
	d.AdapterPath, _ = d.core.Adapter().Get(0)
	d.Name, _ = d.core.Name().Get(0)
	d.Alias, _ = d.core.Alias().Get(0)
	d.Address, _ = d.core.Address().Get(0)
	d.Trusted, _ = d.core.Trusted().Get(0)
	d.Paired, _ = d.core.Paired().Get(0)
	d.connected, _ = d.core.Connected().Get(0)
	d.UUIDs, _ = d.core.UUIDs().Get(0)
	d.ServicesResolved, _ = d.core.ServicesResolved().Get(0)
	d.Icon, _ = d.core.Icon().Get(0)
	d.RSSI, _ = d.core.RSSI().Get(0)
	d.needNotify = true
	d.updateState()
	if d.Paired && d.connected {
		d.ConnectState = true
		//切換用户时添加设备到connectedDevices列表中
		globalBluetooth.addConnectedDevice(d)
	}
	d.disconnectChan = make(chan struct{})
	d.core.InitSignalExt(systemSigLoop, true)
	d.connectProperties()
	return
}

func (d *device) destroy() {
	d.core.RemoveHandler(proxy.RemoveAllHandlers)
}

func (d *device) notifyDeviceAdded() {
	logger.Debug("DeviceAdded", d)
	err := globalBluetooth.service.Emit(globalBluetooth, "DeviceAdded", marshalJSON(d))
	if err != nil {
		logger.Warning(err)
	}
	globalBluetooth.updateState()
}

func (d *device) notifyDeviceRemoved() {
	logger.Debug("DeviceRemoved", d)
	err := globalBluetooth.service.Emit(globalBluetooth, "DeviceRemoved", marshalJSON(d))
	if err != nil {
		logger.Warning(err)
	}
	globalBluetooth.updateState()
}

func (d *device) notifyDevicePropertiesChanged() {
	err := globalBluetooth.service.Emit(globalBluetooth, "DevicePropertiesChanged", marshalJSON(d))
	if err != nil {
		logger.Warning(err)
	}
	globalBluetooth.updateState()
}

func (d *device) connectProperties() {
	err := d.core.Connected().ConnectChanged(func(hasValue bool, connected bool) {
		if !hasValue {
			return
		}
		logger.Debugf("%s Connected: %v", d, connected)
		d.connected = connected

		//音频设备主动发起连接时也断开之前的音频连接
		if d.connected && d.Paired {
			go d.audioA2DPWorkaround()
		}

		// check if device need to be removed, if is, remove device
		needRemove := d.getAndResetNeedRemove()
		if needRemove {
			// start remove device
			err := d.adapter.core.Adapter().RemoveDevice(0, d.Path)
			if err != nil {
				logger.Warningf("failed to remove device %q from adapter %q: %v",
					d.adapter.Path, d.Path, err)
			}
			return
		}

		if connected {
			d.ConnectState = true
			d.connectedTime = time.Now()
			globalBluetooth.config.setDeviceConfigConnected(d, true)
			dev := globalBluetooth.getConnectedDeviceByAddress(d.Address)
			if dev == nil {
				globalBluetooth.addConnectedDevice(d)
				logger.Debug("connectedDevices", globalBluetooth.connectedDevices)
			}
		} else {
			//If the pairing is successful and connected, the signal will be sent when the device is disconnected
			if d.Paired && d.ConnectState {
				notifyDisconnected(d.Alias)
			}
			d.needNotify = true
			d.ConnectState = false

			// if disconnect success, remove device from map
			globalBluetooth.removeConnectedDevice(d)
			// when disconnected quickly after connecting, automatically try to connect
			sinceConnected := time.Since(d.connectedTime)
			logger.Debug("sinceConnected:", sinceConnected)
			logger.Debug("retryConnectCount:", d.retryConnectCount)

			if sinceConnected < 300*time.Millisecond {
				if d.retryConnectCount == 0 {
					go d.Connect()
				}
				d.retryConnectCount++
			} else if sinceConnected > 2*time.Second {
				d.retryConnectCount = 0
			}

			select {
			case d.disconnectChan <- struct{}{}:
				logger.Debugf("%s disconnectChan send done", d)
			default:
			}
		}

		d.updateState()
		d.notifyDevicePropertiesChanged()

		if d.needNotify && d.Paired && d.State == deviceStateConnected && d.ConnectState {
			d.notifyConnectedChanged()
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	_ = d.core.Name().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}
		logger.Debugf("%s Name: %v", d, value)
		d.Name = value
		d.notifyDevicePropertiesChanged()
	})

	_ = d.core.Alias().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}
		d.Alias = value
		logger.Debugf("%s Alias: %v", d, value)
		d.notifyDevicePropertiesChanged()
	})

	_ = d.core.Address().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}
		d.Address = value
		logger.Debugf("%s Address: %v", d, value)
		d.notifyDevicePropertiesChanged()
	})

	_ = d.core.Trusted().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		d.Trusted = value
		logger.Debugf("%s Trusted: %v", d, value)
		d.notifyDevicePropertiesChanged()
	})

	_ = d.core.Paired().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		d.Paired = value
		d.updateState() // Paired属性被修改，影响到下面使用的State和ConnectState
		logger.Debugf("%s Paired: %v State: %v", d, value, d.State)

		if d.Paired && d.connected && d.State == deviceStateConnected {
			d.ConnectState = true
			dev := globalBluetooth.getConnectedDeviceByAddress(d.Address)
			if dev == nil {
				globalBluetooth.addConnectedDevice(d)
			}
		}

		if d.needNotify && d.Paired && d.State == deviceStateConnected && d.ConnectState {
			notifyConnected(d.Alias)
			d.needNotify = false
		}
		d.notifyDevicePropertiesChanged()
	})

	_ = d.core.ServicesResolved().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		d.ServicesResolved = value
		logger.Debugf("%s ServicesResolved: %v", d, value)
		d.notifyDevicePropertiesChanged()
	})

	_ = d.core.Icon().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}
		d.Icon = value
		logger.Debugf("%s Icon: %v", d, value)
		d.notifyDevicePropertiesChanged()
	})

	_ = d.core.UUIDs().ConnectChanged(func(hasValue bool, value []string) {
		if !hasValue {
			return
		}
		d.UUIDs = value
		logger.Debugf("%s UUIDs: %v", d, value)
		d.notifyDevicePropertiesChanged()
	})

	_ = d.core.RSSI().ConnectChanged(func(hasValue bool, value int16) {
		if !hasValue {
			d.RSSI = 0
			logger.Debugf("%s RSSI invalidated", d)
		} else {
			d.RSSI = value
			logger.Debugf("%s RSSI: %v", d, value)
		}
		d.notifyDevicePropertiesChanged()
	})

	_ = d.core.LegacyPairing().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		logger.Debugf("%s LegacyPairing: %v", d, value)
	})

	_ = d.core.Blocked().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		logger.Debugf("%s Blocked: %v", d, value)
	})
}

func (d *device) notifyConnectedChanged() {
	connectPhase := d.getConnectPhase()
	if connectPhase != connectPhaseNone {
		// connect is in progress
		logger.Debugf("%s handleNotifySend: connect is in progress", d)
		return
	}

	disconnectPhase := d.getDisconnectPhase()
	if disconnectPhase != disconnectPhaseNone {
		// disconnect is in progress
		logger.Debugf("%s handleNotifySend: disconnect is in progress", d)
		return
	}

	if d.connected {
		notifyConnected(d.Alias)
		d.needNotify = false
		//} else {
		//	if time.Since(d.pairingFailedTime) < 2*time.Second {
		//		return
		//	}
		//	notifyDisconnected(d.Alias)
	}
}

func (d *device) updateState() {
	newState := d.getState()
	if d.State != newState {
		d.State = newState
		logger.Debugf("%s State: %s", d, d.State)
	}
}

func (d *device) getState() deviceState {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.agentWorking {
		return deviceStateConnecting
	}

	if d.connectPhase != connectPhaseNone {
		return deviceStateConnecting

	} else if d.disconnectPhase != connectPhaseNone {
		return deviceStateDisconnecting

	} else {
		if d.connected && d.Paired {
			return deviceStateConnected
		} else {
			return deviceStateDisconnected
		}
	}
}

func (d *device) getAddress() string {
	if d.adapter != nil {
		return d.adapter.address + "/" + d.Address
	}
	return "/"
}

func (d *device) doConnect(hasNotify bool) error {
	connectPhase := d.getConnectPhase()
	disconnectPhase := d.getDisconnectPhase()
	if connectPhase != connectPhaseNone {
		logger.Warningf("%s connect is in progress", d)
		return nil
	} else if disconnectPhase != disconnectPhaseNone {
		logger.Debugf("%s disconnect is in progress", d)
		return nil
	}

	d.setConnectPhase(connectPhaseStart)
	defer d.setConnectPhase(connectPhaseNone)

	err := d.cancelBlock()
	if err != nil {
		// if hasNotify {
		// 	// TODO(jouyouyun): notify device blocked
		// }
		return err
	}

	err = d.doPair()
	if err != nil {
		d.ConnectState = false
		if hasNotify {
			notifyConnectFailedHostDown(d.Alias)
		}
		return err
	}

	d.audioA2DPWorkaround()

	err = d.doRealConnect()
	if err != nil {
		if hasNotify {
			if resourceUnavailable == err.Error() {
				notifyConnectFailedResourceUnavailable(d.Alias, d.adapter.Alias)
			} else {
				notifyConnectFailedHostDown(d.Alias)
			}
		}
		d.ConnectState = false
		return err
	}

	d.ConnectState = true
	d.notifyDevicePropertiesChanged()
	if d.needNotify && d.Paired && d.State == deviceStateConnected && d.ConnectState {
		notifyConnected(d.Alias)
		d.needNotify = false
	}
	return nil
}

func (d *device) doRealConnect() error {
	d.setConnectPhase(connectPhaseConnectProfilesStart)
	err := d.core.Connect(0)
	d.setConnectPhase(connectPhaseConnectProfilesEnd)
	if err != nil {
		// connect failed
		logger.Warningf("%s connect failed: %v", d, err)
		globalBluetooth.config.setDeviceConfigConnected(d, false)
		return err
	}

	// connect succeeded
	logger.Infof("%s connect succeeded", d)
	globalBluetooth.config.setDeviceConfigConnected(d, true)

	// auto trust device when connecting success
	err = d.doTrust()
	if err != nil {
		logger.Warning(err)
	}

	return nil
}

func (d *device) doTrust() error {
	trusted, _ := d.core.Trusted().Get(0)
	if trusted {
		return nil
	}
	err := d.core.Trusted().Set(0, true)
	if err != nil {
		logger.Warning(err)
	}
	return err
}

func (d *device) cancelBlock() error {
	blocked, err := d.core.Blocked().Get(0)
	if err != nil {
		logger.Warning(err)
		return err
	}
	if !blocked {
		return nil
	}
	err = d.core.Blocked().Set(0, false)
	if err != nil {
		logger.Warning(err)
	}
	return err
}

func (d *device) doPair() error {
	paired, err := d.core.Paired().Get(0)
	if err != nil {
		logger.Warning(err)
		return err
	}
	if paired {
		logger.Debugf("%s already paired", d)
		return nil
	}

	d.setConnectPhase(connectPhasePairStart)
	err = d.core.Pair(0)
	d.setConnectPhase(connectPhasePairEnd)
	if err != nil {
		logger.Warningf("%s pair failed: %v", d, err)
		d.pairingFailedTime = time.Now()
		d.setConnectPhase(connectPhaseNone)
		return err
	}

	logger.Warningf("%s pair succeeded", d)
	return nil
}

func (d *device) markNeedRemove(need bool) {
	d.removeLock.Lock()
	d.needRemove = need
	d.removeLock.Unlock()
}

// get and reset needRemove
func (d *device) getAndResetNeedRemove() bool {
	d.removeLock.Lock()
	defer d.removeLock.Unlock()
	needRemove := d.needRemove
	// if needRemove is true, reset needRemove
	if needRemove {
		d.needRemove = false
	}
	return needRemove
}

func (d *device) audioA2DPWorkaround() {
	// TODO: remove work code if bluez a2dp is ok
	// bluez do not support muti a2dp devices
	// disconnect a2dp device before connect
	for _, uuid := range d.UUIDs {
		if uuid == A2DP_SINK_UUID {
			globalBluetooth.disconnectA2DPDeviceExcept(d)
		}
	}
}

func (d *device) Connect() {
	logger.Debug(d, "call Connect()")
	// Pc request a connection, don not need to open add-device-window
	// set ensure state as true
	d.SetInitiativeConnect(true)
	err := d.doConnect(true)
	if err != nil {
		logger.Warning("connect Device Failed:", err)
	}
}

func (d *device) Disconnect() {
	logger.Debugf("%s call Disconnect()", d)

	disconnectPhase := d.getDisconnectPhase()
	if disconnectPhase != disconnectPhaseNone {
		logger.Debugf("%s disconnect is in progress", d)
		return
	}

	d.setDisconnectPhase(disconnectPhaseStart)
	defer d.setDisconnectPhase(disconnectPhaseNone)

	connected, err := d.core.Connected().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	if !connected {
		logger.Debugf("%s not connected", d)
		return
	}

	// 判断是否为经典设备, 如果非经典设备, 则先设置trusted为false, 防止回连
	if !globalBluetooth.isBREDRDevice(d) {
		err := d.core.Trusted().Set(0, false)
		if err != nil {
			logger.Warningf("set trust failed, err: %v", err)
		}
	}

	globalBluetooth.config.setDeviceConfigConnected(d, false)

	ch := d.goWaitDisconnect()

	d.setDisconnectPhase(disconnectPhaseDisconnectStart)
	err = d.core.Disconnect(0)
	if err != nil {
		logger.Warningf("failed to disconnect %s: %v", d, err)
	}
	d.setDisconnectPhase(disconnectPhaseDisconnectEnd)
	d.ConnectState = false
	d.notifyDevicePropertiesChanged()

	<-ch
	notifyDisconnected(d.Alias)
	d.needNotify = true
}

func (d *device) goWaitDisconnect() chan struct{} {
	ch := make(chan struct{})
	go func() {
		select {
		case <-d.disconnectChan:
			logger.Debugf("%s disconnectChan receive ok", d)
		case <-time.After(60 * time.Second):
			logger.Debugf("%s disconnectChan receive timed out", d)
		}
		ch <- struct{}{}
	}()
	return ch
}

// set and get pc or mobile request a connection first
// if is true, pc request first, dont need to show notification window
// else need show
func (d *device) SetInitiativeConnect(isInitiativeConnect bool) {
	d.mu.Lock()
	d.isInitiativeConnect = isInitiativeConnect
	d.mu.Unlock()
}
func (d *device) GetInitiativeConnect() bool {
	d.mu.Lock()
	needEnsure := d.isInitiativeConnect
	d.mu.Unlock()
	return needEnsure
}

func newBackupDevice(d *device) (bd *backupDevice) {
	bd = &backupDevice{}
	bd.AdapterPath = d.AdapterPath
	bd.Path = d.Path
	bd.connected = d.connected
	bd.Alias = d.Alias
	bd.Paired = d.Paired
	bd.Address = d.Address
	bd.State = d.State
	bd.Name = d.Name
	bd.ConnectState = d.ConnectState
	bd.Icon = d.Icon
	bd.RSSI = d.RSSI
	bd.ServicesResolved = d.ServicesResolved
	bd.Trusted = d.Trusted
	bd.UUIDs = d.UUIDs
	return bd
}
