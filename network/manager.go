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
	"net/http"
	"os/exec"
	"sync"
	"time"

	dbus "github.com/godbus/dbus"
	sessionmanager "github.com/linuxdeepin/go-dbus-factory/com.deepin.sessionmanager"
	ipwatchd "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.ipwatchd"
	sysNetwork "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.network"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	nmdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.networkmanager"
	secrets "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.secrets"
	"pkg.deepin.io/dde/daemon/common/dsync"
	"pkg.deepin.io/dde/daemon/network/nm"
	"pkg.deepin.io/dde/daemon/network/proxychains"
	"pkg.deepin.io/dde/daemon/session/common"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/proxy"
	. "pkg.deepin.io/lib/gettext"
	"pkg.deepin.io/lib/strv"
)

const (
	dbusServiceName = "com.deepin.daemon.Network"
	dbusPath        = "/com/deepin/daemon/Network"
	dbusInterface   = "com.deepin.daemon.Network"
)

const checkRepeatTime = 1 * time.Second

type connectionData map[string]map[string]dbus.Variant

var globalSessionActive bool

//go:generate dbusutil-gen em -type Manager,SecretAgent

// Manager is the main DBus object for network module.
type Manager struct {
	sysSigLoop         *dbusutil.SignalLoop
	service            *dbusutil.Service
	sysNetwork         sysNetwork.Network
	sysIPWatchD        ipwatchd.IPWatchD
	nmObjManager       nmdbus.ObjectManager
	PropsMu            sync.RWMutex
	sessionManager     sessionmanager.SessionManager
	currentSessionPath dbus.ObjectPath
	currentSession     login1.Session

	// update by manager.go
	State        uint32 // global networking state
	Connectivity uint32

	NetworkingEnabled bool `prop:"access:rw"` // airplane mode for NetworkManager
	VpnEnabled        bool `prop:"access:rw"`

	// hidden properties
	wirelessEnabled bool
	wwanEnabled     bool
	wiredEnabled    bool

	delayEnableVpn bool
	delayVpnLock   sync.Mutex

	// update by manager_devices.go
	devicesLock sync.Mutex
	devices     map[string][]*device
	Devices     string // array of device objects and marshaled by json

	accessPointsLock sync.Mutex
	accessPoints     map[dbus.ObjectPath][]*accessPoint

	// update by manager_connections.go
	connectionsLock sync.Mutex
	connections     map[string]connectionSlice
	Connections     string // array of connection information and marshaled by json

	// update by manager_active.go
	activeConnectionsLock sync.Mutex
	activeConnections     map[dbus.ObjectPath]*activeConnection
	ActiveConnections     string // array of connections that activated and marshaled by json

	secretAgent        *SecretAgent
	stateHandler       *stateHandler
	proxyChainsManager *proxychains.Manager

	sessionSigLoop *dbusutil.SignalLoop
	syncConfig     *dsync.Config

	portalLastDetectionTime time.Time

	WirelessAccessPoints    string `prop:"access:r"` //用于读取AP
	debugChangeAPBand       string //调用接口切换ap频段
	checkAPStrengthTimer    *time.Timer
	protalAuthBrowserOpened bool // PORTAL认证中状态

	//nolint
	signals *struct {
		AccessPointAdded, AccessPointRemoved, AccessPointPropertiesChanged struct {
			devPath, apJSON string
		}
		DeviceEnabled struct {
			devPath string
			enabled bool
		}
		ActiveConnectionInfoChanged struct {
		}
		IPConflict struct {
			ip  string
			mac string
		}
	}
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

// initialize slice code manually to make i18n works
func initSlices() {
	initProxyGsettings()
	initNmStateReasons()
}

func NewManager(service *dbusutil.Service) (m *Manager) {
	m = &Manager{
		service: service,
	}
	return
}

func (m *Manager) init() {
	logger.Info("initialize network")

	systemBus, err := dbus.SystemBus()
	if err != nil {
		return
	}

	sessionBus := m.service.Conn()
	m.sessionSigLoop = dbusutil.NewSignalLoop(sessionBus, 10)
	m.sessionSigLoop.Start()

	m.sysSigLoop = sysSigLoop
	m.initDbusObjects()

	disableNotify()
	defer enableNotify()

	m.sessionManager = sessionmanager.NewSessionManager(sessionBus)
	m.currentSessionPath, err = m.sessionManager.CurrentSessionPath().Get(0)
	if err != nil {
		logger.Warning("get sessionManager CurrentSessionPath failed:", err)
	}
	m.currentSession, err = login1.NewSession(systemBus, m.currentSessionPath)
	if err != nil {
		logger.Error("Failed to connect self session:", err)
	}

	sysService, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Warning(err)
		return
	}

	// TODO(jouyouyun): improve in future
	// Sometimes the 'org.freedesktop.secrets' is not exists, this would block the 'init' function, so move to goroutine
	go func() {
		secServiceObj := secrets.NewService(sessionBus)
		sa, err := newSecretAgent(secServiceObj, m)
		if err != nil {
			logger.Warning(err)
			return
		}
		m.secretAgent = sa

		logger.Debug("unique name on system bus:", systemBus.Names()[0])
		err = sysService.Export("/org/freedesktop/NetworkManager/SecretAgent", sa)
		if err != nil {
			logger.Warning(err)
			return
		}

		// register secret agent
		nmAgentManager := nmdbus.NewAgentManager(systemBus)
		err = nmAgentManager.Register(0, "com.deepin.daemon.network.SecretAgent")
		if err != nil {
			logger.Debug("failed to register secret agent:", err)
		} else {
			logger.Debug("register secret agent ok")
		}
	}()

	// initialize device and connection handlers
	m.sysNetwork = sysNetwork.NewNetwork(systemBus)
	m.initConnectionManage()
	m.initDeviceManage()
	m.initActiveConnectionManage()
	m.initNMObjManager(systemBus)
	m.stateHandler = newStateHandler(m.sysSigLoop, m)
	m.initSysNetwork(systemBus)
	m.initIPConflictManager(systemBus)

	// update property "State"
	err = nmManager.PropState().ConnectChanged(func(hasValue bool, value uint32) {
		m.updatePropState()
		// get network state
		avail, err := isNetworkAvailable()
		if err != nil {
			logger.Warningf("get network state failed, err: %v", err)
			return
		}
		// check network state
		if !avail {
			return
		}
		// get delay vpn state
		delay := m.getDelayEnableVpn()
		// if vpn enable is true, but network disconnect last time, try to auto connect vpn.
		// delay is marked as true when trying to enable vpn state but network cant be available,
		// so need to retry enable vpn and try to auto connect vpn.
		if !delay && !m.VpnEnabled {
			return
		}
		m.setVpnEnable(true)
	})
	if err != nil {
		logger.Warning(err)
	}
	m.updatePropState()

	// update property Connectivity
	_ = nmManager.Connectivity().ConnectChanged(func(hasValue bool, value uint32) {
		logger.Debug("connectivity state changed ", hasValue, value)
		if hasValue && value == nm.NM_CONNECTIVITY_PORTAL {
			go m.doPortalAuthentication()
		}
		m.updatePropConnectivity()
	})
	m.updatePropConnectivity()
	go func() {
		time.Sleep(3 * time.Second)
		connectivity, err := nmManager.CheckConnectivity(0)
		if err != nil {
			logger.Warning(err)
			return
		}
		if connectivity == nm.NM_CONNECTIVITY_PORTAL {
			m.doPortalAuthentication()
		}
	}()

	// 调整nmDev的状态
	m.adjustDeviceStatus()
	// move to power module
	// connect computer suspend signal
	// _, err = loginManager.ConnectPrepareForSleep(func(active bool) {
	// 	if active {
	// 		// suspend
	// 		disableNotify()
	// 	} else {
	// 		// restore
	// 		enableNotify()

	// 		_ = m.RequestWirelessScan()
	// 	}
	// })
	// if err != nil {
	// 	logger.Warning(err)
	// }

	m.syncConfig = dsync.NewConfig("network", &syncConfig{m: m},
		m.sessionSigLoop, dbusPath, logger)
	m.localeFirstConnection() // TODO: 如果安装器在系统服务启动前配置系统语言,则该方法的调用可以移除
}

// 安装完成第一次开机时,NetworkManager自动创建一个网络连接(系统语言未配置),因此需要手动更新一次翻译
func (m *Manager) localeFirstConnection() {
	for _, path := range nmGetActiveConnections() {
		activeConnection, err := nmNewActiveConnection(path)
		if err != nil {
			logger.Warning(err)
			return
		}
		connType, err := activeConnection.Type().Get(0)
		if err != nil {
			logger.Warning(err)
			return
		}
		if connType == nm.NM_SETTING_WIRED_SETTING_NAME {
			connPath, err := activeConnection.Connection().Get(0)
			if err != nil {
				logger.Warning(err)
				continue
			}
			settingsConnection, err := nmNewSettingsConnection(connPath)
			if err != nil {
				logger.Warning(err)
				continue
			}
			unsaved, err := settingsConnection.Unsaved().Get(0)
			if err != nil {
				logger.Warning(err)
				continue
			}
			if unsaved == true {
				var connData connectionData
				connData, err = settingsConnection.GetSettings(0)
				if err != nil {
					logger.Warning(err)
					continue
				}
				if isSettingConnectionIdExists(connData) {
					setSettingConnectionId(connData, Tr("Wired connection"))
				}
				if isSettingIP6ConfigAddressesExists(connData) {
					setSettingIP6ConfigAddresses(connData, getSettingIP6ConfigAddresses(connData))
				}
				if isSettingIP6ConfigRoutesExists(connData) {
					setSettingIP6ConfigRoutes(connData, getSettingIP6ConfigRoutes(connData))
				}
				err = settingsConnection.UpdateUnsaved(0, connData)
				if err != nil {
					logger.Warning(err)
					continue
				}
			}
		}
	}
}

func (m *Manager) destroy() {
	logger.Info("destroy network")
	m.sessionSigLoop.Stop()
	m.syncConfig.Destroy()
	m.nmObjManager.RemoveHandler(proxy.RemoveAllHandlers)
	m.sysNetwork.RemoveHandler(proxy.RemoveAllHandlers)
	destroyDbusObjects()
	destroyStateHandler(m.stateHandler)
	m.clearDevices()
	m.clearAccessPoints()
	m.clearConnections()
	m.clearActiveConnections()

	// reset dbus properties
	m.setPropNetworkingEnabled(false)
	m.updatePropState()

	if m.checkAPStrengthTimer != nil {
		m.checkAPStrengthTimer.Stop()
		m.checkAPStrengthTimer = nil
	}
}

func watchNetworkManagerRestart(m *Manager) {
	_, err := dbusDaemon.ConnectNameOwnerChanged(func(name, oldOwner, newOwner string) {
		if name == "org.freedesktop.NetworkManager" {
			// if a new dbus session was installed, the name and newOwner
			// will be no empty, if a dbus session was uninstalled, the
			// name and oldOwner will be not empty
			if len(newOwner) != 0 {
				// network-manager is starting
				logger.Info("network-manager is starting")
				time.Sleep(1 * time.Second)
				m.init()
			} else {
				// network-manager stopped
				logger.Info("network-manager stopped")
				m.destroy()
			}
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) initSysNetwork(sysBus *dbus.Conn) {
	m.sysNetwork.InitSignalExt(m.sysSigLoop, true)
	err := common.ActivateSysDaemonService(m.sysNetwork.ServiceName_())
	if err != nil {
		logger.Warning(err)
	}

	_, err = m.sysNetwork.ConnectDeviceEnabled(func(devPath dbus.ObjectPath, enabled bool) {
		err := m.service.Emit(manager, "DeviceEnabled", string(devPath), enabled)
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	vpnEnabled, err := m.sysNetwork.VpnEnabled().Get(0)
	if err != nil {
		logger.Warning(err)
	} else {
		// set vpn enable
		m.setVpnEnable(vpnEnabled)
	}
	err = m.sysNetwork.VpnEnabled().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}

		m.PropsMu.Lock()
		m.setPropVpnEnabled(value)
		m.PropsMu.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) initNMObjManager(systemBus *dbus.Conn) {
	objManager := nmdbus.NewObjectManager(systemBus)
	m.nmObjManager = objManager
	objManager.InitSignalExt(m.sysSigLoop, true)
	_, err := objManager.ConnectInterfacesAdded(func(objectPath dbus.ObjectPath,
		interfacesAndProperties map[string]map[string]dbus.Variant) {
		_, ok := interfacesAndProperties["org.freedesktop.NetworkManager.Connection.Active"]
		if ok {
			// add active connection
			m.activeConnectionsLock.Lock()
			defer m.activeConnectionsLock.Unlock()

			logger.Debug("add active connection", objectPath)
			aConn := m.newActiveConnection(objectPath)
			m.activeConnections[objectPath] = aConn
			m.updatePropActiveConnections()
		}
	})
	if err != nil {
		logger.Warning(err)
	}
	_, err = objManager.ConnectInterfacesRemoved(func(objectPath dbus.ObjectPath, interfaces []string) {
		if strv.Strv(interfaces).Contains("org.freedesktop.NetworkManager.Connection.Active") {
			// remove active connection
			m.activeConnectionsLock.Lock()
			defer m.activeConnectionsLock.Unlock()

			logger.Debug("remove active connection", objectPath)
			delete(m.activeConnections, objectPath)
			m.updatePropActiveConnections()
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) doPortalAuthentication() {
	err := exec.Command("pgrep", "startdde").Run()
	if err != nil {
		return
	}

	sincePortalDetection := time.Since(m.portalLastDetectionTime)
	// 处于认证中状态无需再次打开认证窗口
	if sincePortalDetection < checkRepeatTime || m.protalAuthBrowserOpened {
		return
	}

	// http client to get url
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	// get url
	detectUrl := "http://detectportal.deepin.com"
	res, err := client.Get(detectUrl)
	if err != nil {
		logger.Warningf("get remote http failed ,err: %v", err)
		return
	}
	// get portal addr from response
	portal, err := getRedirectFromResponse(res, detectUrl)
	if err != nil {
		logger.Warningf("get redirect hosts failed, err: %v", err)
		return
	}
	logger.Debugf("portal addr is %v", portal)
	err = exec.Command(`xdg-open`, portal).Run()
	if err != nil {
		logger.Warningf("xdg open windows failed, err: %v", err)
		return
	}
	m.portalLastDetectionTime = time.Now()
	m.protalAuthBrowserOpened = true
}

// auto connect vpn
func (m *Manager) autoConnectVpn() {
	// get vpn list from NetworkManager/Settings
	uuidList, err := getAutoConnectConnUuidListByConnType("vpn")
	if err != nil {
		logger.Warningf("get vpn conn uuid list failed, err: %v", err)
		return
	}
	logger.Debugf("all auto connect vpn is %v", uuidList)
	// auto connect vpn list
	for _, uuid := range uuidList {
		_, err := m.activateConnection(uuid, "/")
		if err != nil {
			logger.Warningf("activate connection vpn failed, err: %v", err)
		}
	}
}

// set vpn enable
func (m *Manager) setVpnEnable(vpnEnabled bool) {
	// if vpn enable is true, check if network is available.
	if vpnEnabled {
		// get network available state
		avail, err := isNetworkAvailable()
		if err != nil {
			logger.Warning(err)
			return
		}
		// check if network is available
		if avail {
			logger.Debug("network available is true")
			// if network available is true and enable is true,
			// set vpn enable and emit signal immediately.
			m.setPropVpnEnabled(true)
			// reset delay vpn enable
			m.setDelayEnableVpn(false)
			// auto connect vpn
			m.autoConnectVpn()
		} else {
			logger.Debug("network available is false")
			// mark delayEnableVpn as true
			m.setDelayEnableVpn(true)
		}
	} else {
		logger.Debug("set vpn enable false")
		// reset delay enable vpn as false
		m.setDelayEnableVpn(false)
	}
}

// set delay enable vpn
func (m *Manager) setDelayEnableVpn(enable bool) {
	m.delayVpnLock.Lock()
	m.delayEnableVpn = enable
	m.delayVpnLock.Unlock()
}

// get delay enable vpn
func (m *Manager) getDelayEnableVpn() bool {
	m.delayVpnLock.Lock()
	enable := m.delayEnableVpn
	m.delayVpnLock.Unlock()
	return enable
}
