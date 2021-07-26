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
	"errors"
	"fmt"

	nmdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.networkmanager"
	"pkg.deepin.io/dde/daemon/network/nm"
	dbus "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/utils"
)

type apSecType uint32

const (
	apSecNone apSecType = iota
	apSecWep
	apSecPsk
	apSecEap
)

func (v apSecType) String() string {
	switch v {
	case apSecNone:
		return "none"
	case apSecWep:
		return "wep"
	case apSecPsk:
		return "wpa-psk"
	case apSecEap:
		return "wpa-eap"
	default:
		return fmt.Sprintf("<invalid apSecType %d>", v)
	}
}

type accessPoint struct {
	nmAp    *nmdbus.AccessPoint
	devPath dbus.ObjectPath

	Ssid         string
	Secured      bool
	SecuredInEap bool
	Strength     uint8
	Path         dbus.ObjectPath

	Hidden bool
}

func (m *Manager) newAccessPoint(devPath, apPath dbus.ObjectPath) (ap *accessPoint, err error) {
	nmAp, err := nmNewAccessPoint(apPath)
	if err != nil {
		logger.Warningf("create access point failed, err: %v", err)
		return
	}

	ap = &accessPoint{
		nmAp:    nmAp,
		devPath: devPath,
		Path:    apPath,
	}
	ap.updateProps()
	if len(ap.Ssid) == 0 {
		err = fmt.Errorf("ignore hidden access point")
		logger.Warningf("ssid is nil, err: %v", err)
		return
	}

	// add hidden
	if m.isHidden(ap.Ssid) {
		ap.Hidden = true
	}

	// connect property changed signals
	ap.nmAp.InitSignalExt(m.sysSigLoop, true)
	ap.nmAp.AccessPoint().ConnectPropertiesChanged(func(properties map[string]dbus.Variant) {
		if !m.isAccessPointExists(apPath) {
			return
		}

		m.accessPointsLock.Lock()
		defer m.accessPointsLock.Unlock()
		ignoredBefore := ap.shouldBeIgnore()
		ap.updateProps()
		ignoredNow := ap.shouldBeIgnore()
		apJSON, _ := marshalJSON(ap)
		if ignoredNow == ignoredBefore {
			// ignored state not changed, only send properties changed
			// signal when not ignored
			if ignoredNow {
				logger.Debugf("access point(ignored) properties changed %#v", ap)
			} else {
				logger.Debugf("access point properties changed %#v", ap)
				m.service.Emit(m, "AccessPointPropertiesChanged", string(devPath), apJSON)
			}
		} else {
			// ignored state changed, if became ignored now, send
			// removed signal or send added signal
			if ignoredNow {
				logger.Debugf("access point is ignored %#v", ap)
				m.service.Emit(m, "AccessPointRemoved", string(devPath), apJSON)
			} else {
				logger.Debugf("AccessPointAdded %#v", ap)
				m.service.Emit(m, "AccessPointAdded", string(devPath), apJSON)
			}
		}
	})

	if ap.shouldBeIgnore() {
		logger.Debugf("new access point is ignored %#v", ap)
	} else {
		apJSON, _ := marshalJSON(ap)
		logger.Debugf("AccessPointAdded %#v", ap)
		m.service.Emit(m, "AccessPointAdded", string(devPath), apJSON)
	}

	return
}
func (m *Manager) destroyAccessPoint(ap *accessPoint) {
	// emit AccessPointRemoved signal
	apJSON, _ := marshalJSON(ap)
	m.service.Emit(m, "AccessPointRemoved", string(ap.devPath), apJSON)
	nmDestroyAccessPoint(ap.nmAp)
}
func (a *accessPoint) updateProps() {
	ssid, _ := a.nmAp.Ssid().Get(0)
	a.Ssid = decodeSsid(ssid)
	a.Strength, _ = a.nmAp.Strength().Get(0)
	secTyp, err := getApSecType(a.nmAp)
	if err != nil {
		logger.Warningf("get ap sec typ failed, err: %v", err)
		return
	}
	a.Secured = secTyp != apSecNone
	a.SecuredInEap = secTyp == apSecEap

}

func getApSecType(ap *nmdbus.AccessPoint) (apSecType, error) {
	flags, err := ap.Flags().Get(0)
	if err != nil {
		return 0, err
	}

	wpaFlags, err := ap.WpaFlags().Get(0)
	if err != nil {
		return 0, err
	}
	rsnFlags, err := ap.RsnFlags().Get(0)
	if err != nil {
		return 0, err
	}
	return doParseApSecType(flags, wpaFlags, rsnFlags), nil
}

func doParseApSecType(flags, wpaFlags, rsnFlags uint32) apSecType {
	r := apSecNone

	if (flags&nm.NM_802_11_AP_FLAGS_PRIVACY != 0) && (wpaFlags == nm.NM_802_11_AP_SEC_NONE) && (rsnFlags == nm.NM_802_11_AP_SEC_NONE) {
		r = apSecWep
	}
	if wpaFlags != nm.NM_802_11_AP_SEC_NONE {
		r = apSecPsk
	}
	if rsnFlags != nm.NM_802_11_AP_SEC_NONE {
		r = apSecPsk
	}
	if (wpaFlags&nm.NM_802_11_AP_SEC_KEY_MGMT_802_1X != 0) || (rsnFlags&nm.NM_802_11_AP_SEC_KEY_MGMT_802_1X != 0) {
		r = apSecEap
	}
	return r
}

// Check if current access point should be ignore in front-end. Hide
// the access point that strength less than 10 (not include 0 which
// should be caused by the network driver issue) and not activated.
func (a *accessPoint) shouldBeIgnore() bool {
	if a.Strength < 10 && a.Strength != 0 &&
		!manager.isAccessPointActivated(a.devPath, a.Ssid) {
		return true
	}
	return false
}

func (m *Manager) isAccessPointActivated(devPath dbus.ObjectPath, ssid string) bool {
	for _, path := range nmGetActiveConnections() {
		aconn := m.newActiveConnection(path)
		if aconn.typ == nm.NM_SETTING_WIRELESS_SETTING_NAME && isDBusPathInArray(devPath, aconn.Devices) {
			if ssid == string(nmGetWirelessConnectionSsidByUuid(aconn.Uuid)) {
				return true
			}
		}
	}
	return false
}

func (m *Manager) clearAccessPoints() {
	m.accessPointsLock.Lock()
	defer m.accessPointsLock.Unlock()
	for _, aps := range m.accessPoints {
		for _, ap := range aps {
			m.destroyAccessPoint(ap)
		}
	}
	m.accessPoints = make(map[dbus.ObjectPath][]*accessPoint)
}

func (m *Manager) isHidden(ssid string) bool {
	m.connectionsLock.Lock()
	wirelessCon := m.connections[connectionWireless]
	m.connectionsLock.Unlock()
	for _, conn := range wirelessCon {
		if conn.Ssid == ssid && conn.Hidden {
			logger.Debugf("access point %s is hidden ", ssid)
			return true
		}
	}
	return false
}

func (m *Manager) addAccessPoint(devPath, apPath dbus.ObjectPath) {
	if m.isAccessPointExists(apPath) {
		return
	}

	m.accessPointsLock.Lock()
	defer m.accessPointsLock.Unlock()
	ap, err := m.newAccessPoint(devPath, apPath)
	if err != nil {
		return
	}
	//logger.Debug("add access point", devPath, apPath)
	m.accessPoints[devPath] = append(m.accessPoints[devPath], ap)
}

func (m *Manager) removeAccessPoint(devPath, apPath dbus.ObjectPath) {
	if !m.isAccessPointExists(apPath) {
		return
	}
	devPath, i := m.getAccessPointIndex(apPath)

	m.accessPointsLock.Lock()
	defer m.accessPointsLock.Unlock()
	//logger.Debug("remove access point", devPath, apPath)
	m.accessPoints[devPath] = m.doRemoveAccessPoint(m.accessPoints[devPath], i)
}
func (m *Manager) doRemoveAccessPoint(aps []*accessPoint, i int) []*accessPoint {
	m.destroyAccessPoint(aps[i])
	copy(aps[i:], aps[i+1:])
	aps[len(aps)-1] = nil
	aps = aps[:len(aps)-1]
	return aps
}

func (m *Manager) getAccessPoint(apPath dbus.ObjectPath) (ap *accessPoint) {
	devPath, i := m.getAccessPointIndex(apPath)
	if i < 0 {
		logger.Warning("access point not found", apPath)
		return
	}

	m.accessPointsLock.Lock()
	defer m.accessPointsLock.Unlock()
	ap = m.accessPoints[devPath][i]
	return
}
func (m *Manager) isAccessPointExists(apPath dbus.ObjectPath) bool {
	_, i := m.getAccessPointIndex(apPath)
	if i >= 0 {
		return true
	}
	return false
}
func (m *Manager) getAccessPointIndex(apPath dbus.ObjectPath) (devPath dbus.ObjectPath, index int) {
	m.accessPointsLock.Lock()
	defer m.accessPointsLock.Unlock()
	for d, aps := range m.accessPoints {
		for i, ap := range aps {
			if ap.Path == apPath {
				return d, i
			}
		}
	}
	return "", -1
}

func (m *Manager) isSsidExists(devPath dbus.ObjectPath, ssid string) bool {
	for _, ap := range m.accessPoints[devPath] {
		if ap.Ssid == ssid {
			return true
		}
	}
	return false
}

// GetAccessPoints return all access points object which marshaled by json.
func (m *Manager) GetAccessPoints(path dbus.ObjectPath) (apsJSON string, busErr *dbus.Error) {
	m.accessPointsLock.Lock()
	defer m.accessPointsLock.Unlock()
	accessPoints := m.accessPoints[path]
	filteredAccessPoints := make([]*accessPoint, 0, len(m.accessPoints))
	for _, ap := range accessPoints {
		if !ap.shouldBeIgnore() {
			filteredAccessPoints = append(filteredAccessPoints, ap)
		}
	}
	apsJSON, err := marshalJSON(filteredAccessPoints)
	busErr = dbusutil.ToError(err)
	return
}

func (m *Manager) ActivateAccessPoint(uuid string, apPath, devPath dbus.ObjectPath) (dbus.ObjectPath,
	*dbus.Error) {
	var err error
	cpath, err := m.activateAccessPoint(uuid, apPath, devPath)
	if err != nil {
		logger.Warning("failed to activate access point:", err)
		return "/", dbusutil.ToError(err)
	}
	return cpath, nil
}

func fixApSecTypeChange(uuid string, secType apSecType) (needUserEdit bool, err error) {
	var cpath dbus.ObjectPath
	cpath, err = nmGetConnectionByUuid(uuid)
	if err != nil {
		return
	}

	var conn *nmdbus.ConnectionSettings
	conn, err = nmNewSettingsConnection(cpath)
	if err != nil {
		return
	}
	var connData connectionData
	connData, err = conn.GetSettings(0)
	if err != nil {
		return
	}

	logger.Debugf(">>>>>> conn data: #%v", connData)

	secTypeOld, err := getApSecTypeFromConnData(connData)
	if err != nil {
		logger.Warning("failed to get apSecType from connData")
		return false, nil
	}

	logger.Debugf(">>>>>> current sec type is %s", secTypeOld.String())

	if secTypeOld == secType {
		return
	}
	logger.Debug("apSecType change to", secType)

	switch secType {
	case apSecNone:
		logicSetSettingVkWirelessSecurityKeyMgmt(connData, "none")
	case apSecWep:
		logicSetSettingVkWirelessSecurityKeyMgmt(connData, "wep")
	case apSecPsk:
		logicSetSettingVkWirelessSecurityKeyMgmt(connData, "wpa-psk")
	case apSecEap:
		needUserEdit = true
		return
	}

	// fix ipv6 addresses and routes data structure, interface{}
	if isSettingIP6ConfigAddressesExists(connData) {
		setSettingIP6ConfigAddresses(connData, getSettingIP6ConfigAddresses(connData))
	}
	if isSettingIP6ConfigRoutesExists(connData) {
		setSettingIP6ConfigRoutes(connData, getSettingIP6ConfigRoutes(connData))
	}

	logger.Debugf(">>>>>> update conn data: #%v", connData)
	err = conn.Update(0, connData)
	return
}

// ActivateAccessPoint add and activate connection for access point.
func (m *Manager) activateAccessPoint(uuid string, apPath, devPath dbus.ObjectPath) (cpath dbus.ObjectPath, err error) {
	logger.Debugf("ActivateAccessPoint: uuid=%s, apPath=%s, devPath=%s", uuid, apPath, devPath)
	cpath = "/"

	var nmAp *nmdbus.AccessPoint
	nmAp, err = nmNewAccessPoint(apPath)
	if err != nil {
		logger.Debugf("new access point failed, err: %v", err)
		return
	}

	// adjust uuid
	ssid, err := nmAp.Ssid().Get(0)
	// if ap exist now, but uuid not exist or /, need adjust uuid
	if err == nil && (uuid == "" || uuid == `/`) {
		logger.Debugf("need adjust uuid, ap: %s", apPath)
		wirelessCons := m.connections[connectionWireless]
		for _, conn := range wirelessCons {
			// check if ssid has already config
			if conn.Ssid == string(ssid) {
				logger.Debugf("ssid has conn settings #%v already, fix uuid", conn)
				uuid = conn.Uuid
				break
			}
		}
	}

	secType, err := getApSecType(nmAp)
	if err != nil {
		logger.Warningf("get ap sec typ failed, err: %v", err)
	}
	if uuid != "" {
		// if get sec type success, should fix ap typ
		if err == nil {
			logger.Debugf("get type success, ap sec typ: %v", secType)
			var needUserEdit bool
			needUserEdit, err = fixApSecTypeChange(uuid, secType)
			if err != nil {
				return
			}
			if needUserEdit {
				err = errors.New("need user edit")
				return
			}
		}
		cpath, err = m.activateConnection(uuid, devPath)
	} else {
		// if there is no connection for current access point, create one
		uuid = utils.GenUuid()
		var ssid []byte
		ssid, err = nmAp.Ssid().Get(0)
		if err != nil {
			logger.Warning("failed to get Ap Ssid:", err)
			return
		}

		data := newWirelessConnectionData(decodeSsid(ssid), uuid, ssid, secType)
		cpath, _, err = nmAddAndActivateConnection(data, devPath, true)
		if err != nil {
			return
		}
	}
	return
}
