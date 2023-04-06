// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package network

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	dbus "github.com/godbus/dbus/v5"
	nmdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.networkmanager"
	"github.com/linuxdeepin/go-lib/dbusutil"
	. "github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/dde-daemon/network/nm"
)

type connectionSlice []*connection

func (c connectionSlice) Len() int           { return len(c) }
func (c connectionSlice) Less(i, j int) bool { return c[i].Id < c[j].Id }
func (c connectionSlice) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }

type connection struct {
	nmConn   nmdbus.ConnectionSettings
	connType string

	Path    dbus.ObjectPath
	Uuid    string
	Id      string
	IfcName string

	// if not empty, the connection will only apply to special device,
	// works for wired, wireless, infiniband, wimax devices
	HwAddress     string
	ClonedAddress string

	// works for wireless, olpc-mesh connections
	Ssid   string
	Hidden bool
}

func (m *Manager) initConnectionManage() {
	m.connectionsLock.Lock()
	m.connections = make(map[string]connectionSlice)
	m.connectionsLock.Unlock()

	_, err := nmSettings.ConnectNewConnection(func(cpath dbus.ObjectPath) {
		logger.Info("add connection", cpath)
		m.addConnection(cpath)
	})
	if err != nil {
		logger.Warning(err)
	}
	_, err = nmSettings.ConnectConnectionRemoved(func(cpath dbus.ObjectPath) {
		logger.Info("remove connection", cpath)
		m.removeConnection(cpath)
	})
	if err != nil {
		logger.Warning(err)
	}
	for _, cpath := range nmGetConnectionList() {
		m.addConnection(cpath)
	}
}

func isTempWiredConnectionSettings(conn nmdbus.ConnectionSettings, cdata connectionData) (bool, error) {
	connType := getSettingConnectionType(cdata)
	if connType != nm.NM_SETTING_WIRED_SETTING_NAME {
		return false, nil
	}
	// wired connection
	autoConnect := getSettingConnectionAutoconnect(cdata)
	autoConnectPriority := getSettingConnectionAutoconnectPriority(cdata)
	if autoConnect && autoConnectPriority == -999 {
		unsaved, err := conn.Unsaved().Get(0)
		if err != nil {
			return false, err
		}
		if unsaved {
			return true, nil
		}
	}
	return false, nil
}

func (m *Manager) deleteTempConnectionSettings(conn *connection, cdata connectionData) {
	isTemp, err := isTempWiredConnectionSettings(conn.nmConn, cdata)
	if err != nil {
		logger.Warning(err)
		return
	}
	if isTemp {
		uuid := getSettingConnectionUuid(cdata)
		logger.Debug("delete unsaved connection", conn.Path, uuid)
		err = conn.nmConn.Delete(0)
		if err != nil {
			logger.Warningf("failed to delete connection %s: %v", conn.Path, err)
		}
	}
}

func (m *Manager) newConnection(cpath dbus.ObjectPath) (conn *connection, err error) {
	conn = &connection{Path: cpath}
	nmConn, err := nmNewSettingsConnection(cpath)
	if err != nil {
		return
	}

	conn.nmConn = nmConn
	cdata := conn.updateProps()
	if cdata != nil {
		m.deleteTempConnectionSettings(conn, cdata)
	}

	// connect signals
	nmConn.InitSignalExt(m.sysSigLoop, true)
	_, err = nmConn.ConnectUpdated(func() {
		m.updateConnection(cpath)
	})
	if err != nil {
		logger.Warning(err)
	}

	err = nil
	return
}

func (conn *connection) updateProps() connectionData {
	cdata, err := conn.nmConn.GetSettings(0)
	if err != nil {
		logger.Error(err)
		return nil
	}

	conn.Uuid = getSettingConnectionUuid(cdata)
	conn.Id = getSettingConnectionId(cdata)
	conn.IfcName = getSettingConnectionInterfaceName(cdata)

	switch getSettingConnectionType(cdata) {
	case nm.NM_SETTING_GSM_SETTING_NAME, nm.NM_SETTING_CDMA_SETTING_NAME:
		conn.connType = connectionMobile
	case nm.NM_SETTING_VPN_SETTING_NAME:
		conn.connType = connectionVpn
	default:
		conn.connType = getCustomConnectionType(cdata)
	}

	switch getSettingConnectionType(cdata) {
	case nm.NM_SETTING_WIRED_SETTING_NAME, nm.NM_SETTING_PPPOE_SETTING_NAME:
		if isSettingWiredMacAddressExists(cdata) {
			conn.HwAddress = convertMacAddressToString(getSettingWiredMacAddress(cdata))
		} else {
			conn.HwAddress = ""
		}
		if isSettingWiredClonedMacAddressExists(cdata) {
			conn.ClonedAddress = convertMacAddressToString(getSettingWiredClonedMacAddress(cdata))
		} else {
			conn.ClonedAddress = ""
		}
	case nm.NM_SETTING_WIRELESS_SETTING_NAME:
		conn.Ssid = decodeSsid(getSettingWirelessSsid(cdata))
		if isSettingWirelessMacAddressExists(cdata) {
			conn.HwAddress = convertMacAddressToString(getSettingWirelessMacAddress(cdata))
		} else {
			conn.HwAddress = ""
		}
		if isSettingWirelessChannelExists(cdata) {
			conn.ClonedAddress = convertMacAddressToString(getSettingWirelessClonedMacAddress(cdata))
		} else {
			conn.ClonedAddress = ""
		}
		conn.Hidden = getSettingWirelessHidden(cdata)
	}
	return cdata
}

func (m *Manager) destroyConnection(conn *connection) {
	nmDestroySettingsConnection(conn.nmConn)
}

func (m *Manager) clearConnections() {
	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	for _, conns := range m.connections {
		for _, conn := range conns {
			m.destroyConnection(conn)
		}
	}
	m.connections = make(map[string]connectionSlice)
	m.updatePropConnections()
}

func (m *Manager) addConnection(cpath dbus.ObjectPath) {
	if m.isConnectionExists(cpath) {
		logger.Warning("connection already exists", cpath)
		return
	}

	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	conn, err := m.newConnection(cpath)
	if err != nil {
		return
	}
	logger.Debug("add connection", cpath)
	switch conn.connType {
	case connectionUnknown:
	default:
		m.connections[conn.connType] = append(m.connections[conn.connType], conn)
		sort.Sort(m.connections[conn.connType])
	}
	m.updatePropConnections()
}

func (m *Manager) removeConnection(cpath dbus.ObjectPath) {
	if !m.isConnectionExists(cpath) {
		logger.Warning("connection not found", cpath)
		return
	}
	connType, i := m.getConnectionIndex(cpath)

	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	m.connections[connType] = m.doRemoveConnection(m.connections[connType], i)
	m.updatePropConnections()
}

func (m *Manager) doRemoveConnection(conns connectionSlice, i int) connectionSlice {
	logger.Debugf("remove connection %#v", conns[i])
	m.destroyConnection(conns[i])
	copy(conns[i:], conns[i+1:])
	conns = conns[:len(conns)-1]
	return conns
}

func (m *Manager) updateConnection(cpath dbus.ObjectPath) {
	if !m.isConnectionExists(cpath) {
		logger.Warning("connection not found", cpath)
		return
	}
	conn := m.getConnection(cpath)

	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	conn.updateProps()
	logger.Debugf("update connection %#v", conn)
	m.updatePropConnections()
}

func (m *Manager) getConnection(cpath dbus.ObjectPath) (conn *connection) {
	connType, i := m.getConnectionIndex(cpath)
	if i < 0 {
		logger.Warning("connection not found", cpath)
		return
	}

	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	conn = m.connections[connType][i]
	return
}
func (m *Manager) isConnectionExists(cpath dbus.ObjectPath) bool {
	_, i := m.getConnectionIndex(cpath)
	return i >= 0
}
func (m *Manager) getConnectionIndex(cpath dbus.ObjectPath) (connType string, index int) {
	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	for t, conns := range m.connections {
		for i, c := range conns {
			if c.Path == cpath {
				return t, i
			}
		}
	}
	return "", -1
}

// GetSupportedConnectionTypes return all supported connection types
func (m *Manager) GetSupportedConnectionTypes() (types []string, err *dbus.Error) {
	return supportedConnectionTypes, nil
}

func (m *Manager) ensureUniqueConnectionExists(devPath dbus.ObjectPath, active bool) (cpath dbus.ObjectPath, exists bool, err error) {
	cpath = "/"
	switch nmGetDeviceType(devPath) {
	case nm.NM_DEVICE_TYPE_ETHERNET:
		cpath, exists, err = m.ensureWiredConnectionExists(devPath, active)
	}
	return
}

// ensureWiredConnectionExists will check if wired connection for
// target device exists, if not, create one.
func (m *Manager) ensureWiredConnectionExists(wiredDevPath dbus.ObjectPath, active bool) (cpath dbus.ObjectPath, exists bool, err error) {
	m.connectionSettingsLock.Lock()
	defer m.connectionSettingsLock.Unlock()

	uuid := nmGeneralGetDeviceUniqueUuid(wiredDevPath)
	logger.Debug("uuid:", uuid)

	cpath, err = nmGetConnectionByUuid(uuid)
	if err == nil {
		exists = true
		return
	}

	// try get uuid from active or available connection
	existedUuid := getNonTempWiredDeviceConnectionUuid(wiredDevPath)
	logger.Debug("existed uuid:", existedUuid)
	if existedUuid != "" {
		cpath, err = nmGetConnectionByUuid(existedUuid)
		if err == nil {
			exists = true
			return
		}
	}

	// connection not exists, create one
	logger.Debug("connection not exist, create one, uuid:", uuid)
	var id string
	if nmGeneralIsUsbDevice(wiredDevPath) {
		id = nmGeneralGetDeviceDesc(wiredDevPath)
	} else {
		id = m.getCreateConnectionName()
	}
	cpath, err = newWiredConnectionForDevice(id, uuid, wiredDevPath, active)
	return
}

// lock connection
func (m *Manager) getCreateConnectionName() string {
	m.connectionsLock.Lock()
	defer m.connectionsLock.Unlock()
	// get all wired connection
	wiredConnSl := m.connections[deviceEthernet]
	var numSl []int
	// add 0, make sure at least elem
	numSl = append(numSl, 0)
	// get all connection index
	for _, conn := range wiredConnSl {
		// if current id dont contains Tr, ignore
		if !strings.Contains(conn.Id, Tr("Wired Connection")) {
			continue
		}
		// trim left, " 1"
		numStr := strings.TrimLeft(conn.Id, Tr("Wired Connection"))
		// id is Wired Connection, add 0 to sl
		if numStr == "" {
			numSl = append(numSl, 0)
			continue
		}
		// trim " "
		numStr = strings.Trim(numStr, " ")
		// convert num string to int
		num, err := strconv.Atoi(numStr)
		// could fail here, such as id is "Wired Connection A/B"
		if err != nil {
			logger.Debugf("connection index convert unexpected happens %v", err)
			continue
		}
		// append
		numSl = append(numSl, num)
	}

	// sort num sl
	sort.Ints(numSl)

	logger.Debugf("num Sl is %v", numSl)

	var num int
	for index := 0; index < len(numSl); index++ {
		// already in the last one, such as numSl is [0,0], num is  0+1
		if index == len(numSl)-1 {
			num = numSl[index] + 1
			break
		}
		// check if equal next or larger
		if numSl[index] == numSl[index+1] || numSl[index]+1 == numSl[index+1] {
			continue
		}
		num = numSl[index] + 1
		break
	}

	name := fmt.Sprintf(Tr("Wired Connection %v"), num)
	logger.Debugf("create name is %v", name)

	return name
}

func isTempWiredConnectionPath(connPath dbus.ObjectPath) (isTemp bool, uuid string, err error) {
	connSettings, err := nmNewSettingsConnection(connPath)
	if err != nil {
		return
	}
	cdata, err := connSettings.GetSettings(0)
	if err != nil {
		return
	}
	isTemp, err = isTempWiredConnectionSettings(connSettings, cdata)
	if err != nil {
		return
	}
	if !isTemp {
		uuid = getSettingConnectionUuid(cdata)
	}
	return
}

func getNonTempWiredDeviceConnectionUuid(wiredDevPath dbus.ObjectPath) string {
	wired, _ := nmNewDevice(wiredDevPath)
	if wired == nil {
		return ""
	}

	apath, err := wired.Device().ActiveConnection().Get(0)
	if err == nil && isObjPathValid(apath) {
		aConn, _ := nmNewActiveConnection(apath)
		if aConn != nil {
			connPath, err := aConn.Connection().Get(0)
			if err == nil && isObjPathValid(connPath) {
				isTemp, uuid, err := isTempWiredConnectionPath(connPath)
				if err == nil && !isTemp {
					return uuid
				}
			}
		}
	}

	connPaths, _ := wired.Device().AvailableConnections().Get(0)
	for _, connPath := range connPaths {
		if !isObjPathValid(connPath) {
			continue
		}

		isTemp, uuid, err := isTempWiredConnectionPath(connPath)
		if err == nil && !isTemp {
			return uuid
		}
	}
	return ""
}

func isObjPathValid(dpath dbus.ObjectPath) bool {
	if dpath == "" || dpath == "/" {
		return false
	}
	return true
}

// ensureWirelessHotspotConnectionExists will check if wireless hotspot connection for
// target device exists, if not, create one.
func (m *Manager) ensureWirelessHotspotConnectionExists(wirelessDevPath dbus.ObjectPath, active bool) (cpath dbus.ObjectPath, exists bool, err error) {
	uuid := nmGeneralGetDeviceUniqueUuid(wirelessDevPath)
	cpath, err = nmGetConnectionByUuid(uuid)
	if err == nil {
		// connection already exists
		exists = true
		return
	}

	// connection not exists, create one
	cpath, err = newWirelessHotspotConnectionForDevice("hotspot", uuid, wirelessDevPath, active)
	return
}

// DeleteConnection delete a connection through uuid.
func (m *Manager) DeleteConnection(uuid string) *dbus.Error {
	err := m.deleteConnection(uuid)
	return dbusutil.ToError(err)
}

func (m *Manager) deleteConnection(uuid string) (err error) {
	cpath, err := nmGetConnectionByUuid(uuid)
	if err != nil {
		return
	}

	nmConn, err := nmNewSettingsConnection(cpath)
	if err != nil {
		return
	}

	return nmConn.Delete(0)
}

func (m *Manager) ActivateConnection(uuid string, devPath dbus.ObjectPath) (
	cpath dbus.ObjectPath, busErr *dbus.Error) {
	cpath, err := m.activateConnection(uuid, devPath)
	busErr = dbusutil.ToError(err)
	if err != nil {
		logger.Debug("failed to activate a connection", err)
	}
	return
}

// activateConnection try to activate target connection, if not
// special a valid devPath just left it as "/".
// TODO: return apath instead of cpath
func (m *Manager) activateConnection(uuid string, devPath dbus.ObjectPath) (cpath dbus.ObjectPath, err error) {
	logger.Debugf("ActivateConnection: uuid=%s, devPath=%s", uuid, devPath)
	cpath = "/"
	if devPath == "" {
		err = fmt.Errorf("Device path is empty")
		logger.Warning("ActivateConnection empty device path:", uuid)
		return
	}

	cpath, err = nmGetConnectionByUuid(uuid)
	if err != nil {
		// connection will be activated in ensureUniqueConnectionExists() if not exists
		if devPath != "/" && nmGeneralGetDeviceUniqueUuid(devPath) == uuid {
			cpath, _, err = m.ensureUniqueConnectionExists(devPath, true)
		}
		return
	}

	// according to NM API Doc
	// if the activate connection type is vpn, dev path is ignored, dont need to check device state
	conn, err := nmNewSettingsConnection(cpath)
	if err != nil {
		logger.Warning(err)
		return
	}
	// get setting data
	connData, err := conn.GetSettings(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	// check if type is vpn, if not, should async device state
	connTyp := getSettingConnectionType(connData)
	if connTyp != "vpn" {
		// if need enable device
		var enabled bool
		enabled, err = m.getDeviceEnabled(devPath)
		if err != nil {
			logger.Warningf("get device enabled state failed, err: %v", err)
			return
		}
		if !enabled {
			// add new connection but device is disabled, need to update device enabled state as true
			// but dont need to auto connect ActivateConnection, else may cause unexpected connection
			err = m.enableDevice(devPath, true, false)
			if err != nil {
				logger.Warningf("enable device failed, err: %v", err)
				return
			}
		}
	} else {
		// check if support multi connections
		service := getSettingVpnServiceType(connData)
		multi, ok := m.multiVpn[service]
		// if not support, should close old one first
		var multiUuid string
		if !multi || !ok {
			m.activeConnectionsLock.Lock()
			// looking for vpn uuid
			for _, actCon := range m.activeConnections {
				// check if type is the same
				if actCon.vpnType != service {
					continue
				}
				// save uuid
				multiUuid = actCon.Uuid
				break
			}
			m.activeConnectionsLock.Unlock()
		}
		if multiUuid != "" {
			m.deactivateConnection(multiUuid)
		}
	}

	_, err = nmActivateConnection(cpath, devPath)
	return
}

// DeactivateConnection deactivate a target connection.
func (m *Manager) DeactivateConnection(uuid string) *dbus.Error {
	err := m.deactivateConnection(uuid)
	return dbusutil.ToError(err)
}

func (m *Manager) deactivateConnection(uuid string) (err error) {
	apaths, err := nmGetActiveConnectionByUuid(uuid)
	if err != nil {
		// not found active connection with uuid, ignore error here
		return
	}
	for _, apath := range apaths {
		logger.Debug("DeactivateConnection:", uuid, apath)
		if isConnectionStateInActivating(nmGetActiveConnectionState(apath)) {
			if tmpErr := nmDeactivateConnection(apath); tmpErr != nil {
				err = tmpErr
			}
		}
	}
	return
}

// EnableWirelessHotspotMode activate the device related hotspot
// connection, if the connection not exists will create one.
func (m *Manager) EnableWirelessHotspotMode(devPath dbus.ObjectPath) *dbus.Error {
	err := m.enableWirelessHotSpotMode(devPath)
	return dbusutil.ToError(err)
}

func (m *Manager) enableWirelessHotSpotMode(devPath dbus.ObjectPath) (err error) {
	devType := nmGetDeviceType(devPath)
	if devType != nm.NM_DEVICE_TYPE_WIFI {
		err = fmt.Errorf("not a wireless device %s %d", devPath, devType)
		logger.Error(err)
		return
	}

	cpath, exists, err := m.ensureWirelessHotspotConnectionExists(devPath, true)
	if exists {
		// if the connection not exists, it will be activated when
		// creating, but if already exists, we should activate it
		// manually
		_, err = nmActivateConnection(cpath, devPath)
	}
	return
}

// DisableWirelessHotspotMode will disconnect the device related hotspot connection.
func (m *Manager) DisableWirelessHotspotMode(devPath dbus.ObjectPath) *dbus.Error {
	uuid := nmGeneralGetDeviceUniqueUuid(devPath)
	err := m.deactivateConnection(uuid)
	return dbusutil.ToError(err)
}

// IsWirelessHotspotModeEnabled check if the device related hotspot
// connection activated.
func (m *Manager) IsWirelessHotspotModeEnabled(devPath dbus.ObjectPath) (enabled bool, err *dbus.Error) {
	uuid := nmGeneralGetDeviceUniqueUuid(devPath)
	apaths, _ := nmGetActiveConnectionByUuid(uuid)
	if len(apaths) > 0 {
		// the target hotspot connection is activated
		enabled = true
	}
	return
}

// DisconnectDevice will disconnect all connection in target device,
// DisconnectDevice is different with DeactivateConnection, for
// example if user deactivate current connection for a wireless
// device, NetworkManager will try to activate another access point if
// available then, but if call DisconnectDevice for the device, the
// device will keep disconnected later.
func (m *Manager) DisconnectDevice(devPath dbus.ObjectPath) *dbus.Error {
	err := m.doDisconnectDevice(devPath)
	return dbusutil.ToError(err)
}

func (m *Manager) doDisconnectDevice(devPath dbus.ObjectPath) (err error) {
	nmDev, err := nmNewDevice(devPath)
	if err != nil {
		return
	}

	devState, _ := nmDev.Device().State().Get(0)
	if isDeviceStateInActivating(devState) {
		err = nmDev.Device().Disconnect(0)
		if err != nil {
			logger.Error(err)
		}
	}
	return
}

func (m *Manager) updateConnectionBand(conn *connection, band string) (err error) {
	logger.Debug("updateConnectionBand:", conn, band)
	cdata, err := conn.nmConn.GetSettings(0)
	if err != nil {
		logger.Error(err)
		return
	}
	// fix ipv6 addresses and routes data structure, interface{}
	if isSettingIP6ConfigAddressesExists(cdata) {
		setSettingIP6ConfigAddresses(cdata, getSettingIP6ConfigAddresses(cdata))
	}
	if isSettingIP6ConfigRoutesExists(cdata) {
		setSettingIP6ConfigRoutes(cdata, getSettingIP6ConfigRoutes(cdata))
	}
	setSettingWirelessBand(cdata, band)
	err = conn.nmConn.Update(0, cdata)
	if err != nil {
		logger.Error(err)
		return
	}
	return
}
