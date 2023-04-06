// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package network

import (
	"errors"
	"strings"
	"sync"
	"time"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/loader"
	"github.com/linuxdeepin/dde-daemon/network/nm"
	airplanemode "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.airplanemode1"
	networkmanager "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.networkmanager"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/log"
)

const (
	dbusServiceName = "org.deepin.dde.Network1"
	dbusPath        = "/org/deepin/dde/Network1"
	dbusInterface   = dbusServiceName
)

type Module struct {
	*loader.ModuleBase
	network *Network
}

func (m *Module) GetDependencies() []string {
	return nil
}

func (m *Module) Start() error {
	if m.network != nil {
		return nil
	}
	logger.Debug("start network")
	m.network = newNetwork()

	service := loader.GetService()
	m.network.service = service

	err := m.network.init()
	if err != nil {
		return err
	}

	serverObj, err := service.NewServerObject(dbusPath, m.network)
	if err != nil {
		return err
	}

	err = serverObj.SetWriteCallback(m.network, "VpnEnabled", m.network.vpnEnabledWriteCb)
	if err != nil {
		return err
	}

	err = serverObj.Export()
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	return nil
}

func (m *Module) Stop() error {
	// TODO:
	return nil
}

var logger = log.NewLogger("daemon/system/network")

func newModule(logger *log.Logger) *Module {
	m := new(Module)
	m.ModuleBase = loader.NewModuleBase("network", m, logger)
	return m
}

func init() {
	loader.Register(newModule(logger))
}

//go:generate dbusutil-gen -type Network network.go
//go:generate dbusutil-gen em -type Network

type Network struct {
	service    *dbusutil.Service
	VpnEnabled bool `prop:"access:rw"`
	config     *Config
	configMu   sync.Mutex
	devices    map[dbus.ObjectPath]*device
	devicesMu  sync.Mutex
	nmManager  networkmanager.Manager
	nmSettings networkmanager.Settings
	sigLoop    *dbusutil.SignalLoop
	airplane   airplanemode.AirplaneMode

	// nolint
	signals *struct {
		DeviceEnabled struct {
			devPath dbus.ObjectPath
			enabled bool
		}
	}
}

func (n *Network) init() error {
	sysBus := n.service.Conn()
	n.sigLoop = dbusutil.NewSignalLoop(sysBus, 10)
	n.sigLoop.Start()
	// airplane
	n.airplane = airplanemode.NewAirplaneMode(sysBus)
	n.nmManager = networkmanager.NewManager(sysBus)
	n.nmSettings = networkmanager.NewSettings(sysBus)
	// retry get all devices
	n.connectSignal()
	n.addDevicesWithRetry()
	// get vpn enable state from config
	n.VpnEnabled = n.config.VpnEnabled

	return nil
}

type device struct {
	iface    string
	nmDevice networkmanager.Device
	type0    uint32
	count    int
}

func (n *Network) getSysBus() *dbus.Conn {
	return n.service.Conn()
}

func (n *Network) connectSignal() {
	err := dbusutil.NewMatchRuleBuilder().Type("signal").
		PathNamespace("/org/freedesktop/NetworkManager/Devices").
		Interface("org.freedesktop.NetworkManager.Device").
		Member("StateChanged").Build().AddTo(n.getSysBus())
	if err != nil {
		logger.Warning(err)
	}

	err = dbusutil.NewMatchRuleBuilder().Type("signal").
		PathNamespace("/org/freedesktop/NetworkManager/ActiveConnection").
		Interface("org.freedesktop.NetworkManager.VPN.Connection").
		Member("VpnStateChanged").Build().AddTo(n.getSysBus())
	if err != nil {
		logger.Warning(err)
	}

	n.nmManager.InitSignalExt(n.sigLoop, true)
	_, err = n.nmManager.ConnectDeviceAdded(func(devPath dbus.ObjectPath) {
		logger.Debug("device added", devPath)
		err := n.addDevice(devPath)
		if err != nil {
			logger.Warning(err)
		}

	})
	if err != nil {
		logger.Warning(err)
	}

	_, err = n.nmManager.ConnectDeviceRemoved(func(devPath dbus.ObjectPath) {
		logger.Debug("device removed", devPath)
		n.devicesMu.Lock()

		n.removeDevice(devPath)

		n.devicesMu.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}

	n.sigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "org.freedesktop.NetworkManager.VPN.Connection.VpnStateChanged",
	}, func(sig *dbus.Signal) {
		if strings.HasPrefix(string(sig.Path),
			"/org/freedesktop/NetworkManager/ActiveConnection/") &&
			len(sig.Body) >= 2 {

			state, ok := sig.Body[0].(uint32)
			if !ok {
				return
			}
			reason, ok := sig.Body[1].(uint32)
			if !ok {
				return
			}
			logger.Debug(sig.Path, "vpn state changed", state, reason)
			n.handleVpnStateChanged(state)
		}
	})
}

func (n *Network) getWirelessDevices() (devices []*device) {
	n.devicesMu.Lock()

	for _, d := range n.devices {
		if d.type0 == nm.NM_DEVICE_TYPE_WIFI {
			devices = append(devices, d)
		}
	}

	n.devicesMu.Unlock()
	return
}

func (n *Network) handleVpnStateChanged(state uint32) {
	if state >= nm.NM_VPN_CONNECTION_STATE_PREPARE &&
		state <= nm.NM_VPN_CONNECTION_STATE_ACTIVATED {

		changed := n.setPropVpnEnabled(true)

		if changed {
			n.configMu.Lock()
			n.config.VpnEnabled = true
			err := n.saveConfig()
			n.configMu.Unlock()

			if err != nil {
				logger.Warning(err)
			}
		}
	}
}

func (n *Network) addDevice(devPath dbus.ObjectPath) error {
	n.devicesMu.Lock()
	_, ok := n.devices[devPath]
	if ok {
		n.devicesMu.Unlock()
		return nil
	}

	nmDev, err := networkmanager.NewDevice(n.getSysBus(), devPath)
	if err != nil {
		n.devicesMu.Unlock()
		return err
	}
	d := nmDev.Device()
	deviceType, err := d.DeviceType().Get(0)
	if err != nil {
		logger.Warningf("get device %s type failed: %v", nmDev.Path_(), err)
	}

	dev := &device{
		nmDevice: nmDev,
		type0:    deviceType,
	}
	n.devices[devPath] = dev

	n.devicesMu.Unlock()

	nmDev.InitSignalExt(n.sigLoop, true)
	_, err = d.ConnectStateChanged(func(newState uint32, oldState uint32, reason uint32) {
		//logger.Debugf("device state changed %v newState %d", d.Path_(), newState)
		if (oldState >= nm.NM_DEVICE_STATE_ACTIVATED && reason == nm.NM_DEVICE_STATE_REASON_REMOVED) ||
			(newState > oldState && newState == nm.NM_DEVICE_STATE_ACTIVATED) {
			restartIPWatchD()
		}

		enabled := n.isIfaceEnabled(dev.iface)
		state, err := d.State().Get(0)
		if err != nil {
			logger.Warning(err)
			return
		}

		if !enabled {
			if state >= nm.NM_DEVICE_STATE_PREPARE &&
				state <= nm.NM_DEVICE_STATE_ACTIVATED {
				logger.Debug("disconnect device", nmDev.Path_())
				err = d.Disconnect(0)
				if err != nil {
					logger.Warning(err)
				}
			}
		}
	})

	if err != nil {
		logger.Warning(err)
	}

	err = d.Interface().ConnectChanged(func(hasValue bool, new_iface string) {
		if !hasValue {
			return
		}
		logger.Debugf("recv interface changed signal, old iface: %s, new iface: %s", dev.iface, new_iface)
		// update dev interface
		dev, ok := n.devices[devPath]
		if !ok {
			logger.Warningf("device not exist, devPath: %s", devPath)
			return
		}
		oldIface := dev.iface
		dev.iface = new_iface
		// check new interface is legal, if is not, delete config
		if new_iface == "" || new_iface == "/" {
			n.configMu.Lock()
			delete(n.config.Devices, oldIface)
			n.configMu.Unlock()
			err = n.saveConfig()
			if err != nil {
				logger.Warningf("save config failed, err: %v", err)
			}
			return
		}

		// if is legal, reset dev state according to config file
		n.configMu.Lock()
		config, ok := n.config.Devices[dev.iface]
		logger.Debugf("devices config is #%v, iface is: %s", n.config.Devices, dev.iface)
		n.configMu.Unlock()
		if ok {
			_, err = n.enableDevice(dev.iface, config.Enabled)
			if err != nil {
				logger.Warningf("enable dev failed, err: %v", err)
			}
		}
	})

	dev.iface, err = d.Interface().Get(0)
	if err != nil {
		logger.Warning(err)
	}

	n.configMu.Lock()
	config, ok := n.config.Devices[dev.iface]
	logger.Debugf("devices config is #%v, iface is: %s", n.config.Devices, dev.iface)
	n.configMu.Unlock()
	if ok {
		n.enableDevice(dev.iface, config.Enabled)
	}

	return nil
}

func (n *Network) removeDevice(devPath dbus.ObjectPath) {
	d, ok := n.devices[devPath]
	if !ok {
		return
	}
	d.nmDevice.RemoveHandler(proxy.RemoveAllHandlers)
	delete(n.devices, devPath)
}

func (n *Network) GetInterfaceName() string {
	return dbusInterface
}

func newNetwork() *Network {
	n := new(Network)
	cfg := loadConfigSafe(configFile)
	n.config = cfg
	n.devices = make(map[dbus.ObjectPath]*device)
	return n
}

func (n *Network) EnableDevice(pathOrIface string, enabled bool) (cpath dbus.ObjectPath, error *dbus.Error) {
	logger.Infof("call EnableDevice, ifc: %v, enabled: %v", pathOrIface, enabled)
	cpath, err := n.enableDevice(pathOrIface, enabled)
	return cpath, dbusutil.ToError(err)
}

func (n *Network) enableDevice(pathOrIface string, enabled bool) (cpath dbus.ObjectPath, err error) {
	d := n.findDevice(pathOrIface)
	if d == nil {
		logger.Warningf("cant find device, pathOrIface: %s", pathOrIface)
		return "/", errors.New("not found device")
	}

	n.enableIface(d.iface, enabled)

	err = n.service.Emit(n, "DeviceEnabled", d.nmDevice.Path_(), enabled)
	if err != nil {
		logger.Warningf("emit device enabled failed, err: %v", err)
	}

	if enabled {
		cpath, err = n.enableDevice1(d)
		if err != nil {
			logger.Warning(err)
		}
	} else {
		err = n.disableDevice(d)
		if err != nil {
			logger.Warning(err)
		}
		cpath = "/"
	}

	err = n.saveConfig()
	if err != nil {
		logger.Warning(err)
	}

	return cpath, nil
}

func (n *Network) enableDevice1(d *device) (cpath dbus.ObjectPath, err error) {
	err = n.enableNetworking()
	if err != nil {
		return "/", err
	}

	if d.type0 == nm.NM_DEVICE_TYPE_WIFI {
		err = n.enableWireless()
		if err != nil {
			return "/", err
		}
	}

	err = setDeviceManaged(d.nmDevice, true)
	if err != nil {
		return "/", err
	}

	connPaths, err := d.nmDevice.Device().AvailableConnections().Get(0)
	if err != nil {
		return "/", err
	}
	logger.Debug("available connections:", connPaths)

	var connPath0 dbus.ObjectPath
	var maxTs uint64
	for _, connPath := range connPaths {
		connObj, err := networkmanager.NewConnectionSettings(n.getSysBus(), connPath)
		if err != nil {
			logger.Warning(err)
			continue
		}

		settings, err := connObj.GetSettings(0)
		if err != nil {
			logger.Warning(err)
			continue
		}

		auto := getSettingConnectionAutoconnect(settings)
		if !auto {
			continue
		}

		ts := getSettingConnectionTimestamp(settings)
		if maxTs < ts || connPath0 == "" {
			maxTs = ts
			connPath0 = connObj.Path_()
		}
	}

	if connPath0 != "" {
		/*	logger.Debug("activate connection", connPath0)
			_, err = n.nmManager.ActivateConnection(0, connPath0,
				d.nmDevice.Path_(), "/")*/
		return connPath0, err
	}
	return "/", nil
}

func (n *Network) disableDevice(d *device) error {
	err := setDeviceAutoConnect(d.nmDevice, false)
	if err != nil {
		return err
	}

	//TODO:
	//cause of nm'bug, sometimes accessapoints list is nil
	//so add a judge in system network, if get nil in GetAllAccessPoints func, set wirelessEnable down.
	state, err := d.nmDevice.Device().State().Get(0)
	if err != nil {
		return err
	}
	oldWirelessEnabled, err := n.nmManager.WirelessEnabled().Get(0)
	if err != nil {
		return err
	}
	if d.type0 == nm.NM_DEVICE_TYPE_WIFI && state > nm.NM_DEVICE_STATE_UNAVAILABLE {
		mode, err := d.nmDevice.Wireless().Mode().Get(0)
		if err != nil {
			logger.Warning(err)
		}

		if mode != nm.NM_802_11_MODE_AP {
			accessPointsList, err := d.nmDevice.Wireless().GetAllAccessPoints(0)
			if err != nil {
				logger.Warning("GetAllAccessPoints in system network ", err)
			}

			if len(accessPointsList) > 0 {
				d.count = 0
				logger.Debug("have aplist existed!!!")
			} else if d.count++; d.count > 2 {
				d.count = 0
				logger.Info("try to set wireless-enabled, because ap list is empty")
				err = n.nmManager.WirelessEnabled().Set(0, false)
				if err != nil {
					logger.Debug("set WirelessEnabled in system network failed ", err)
					return err
				}
				if oldWirelessEnabled {
					err = n.nmManager.WirelessEnabled().Set(0, true)
					if err != nil {
						logger.Debug("set WirelessEnabled in system network failed ", err)
						return err
					}
				}
				return nil
			}
		}
	}

	if state >= nm.NM_DEVICE_STATE_PREPARE &&
		state <= nm.NM_DEVICE_STATE_ACTIVATED {
		return d.nmDevice.Device().Disconnect(0)
	}
	return nil
}

func (n *Network) saveConfig() error {
	return saveConfig(configFile, n.config)
}

func (n *Network) IsDeviceEnabled(pathOrIface string) (enabled bool, busErr *dbus.Error) {
	enabled, err := n.isDeviceEnabled(pathOrIface)
	return enabled, dbusutil.ToError(err)
}

func (n *Network) isDeviceEnabled(pathOrIface string) (bool, error) {
	d := n.findDevice(pathOrIface)
	if d == nil {
		return false, errors.New("not found device")
	}

	return n.isIfaceEnabled(d.iface), nil
}

func (n *Network) isIfaceEnabled(iface string) bool {
	n.configMu.Lock()
	defer n.configMu.Unlock()

	devCfg, ok := n.config.Devices[iface]
	if !ok {
		// new device default enabled
		return true
	}
	return devCfg.Enabled
}

func (n *Network) enableIface(iface string, enabled bool) {
	n.configMu.Lock()
	deviceConfig := n.config.Devices[iface]
	if deviceConfig == nil {
		deviceConfig = new(DeviceConfig)
		n.config.Devices[iface] = deviceConfig
	}
	deviceConfig.Enabled = enabled
	n.configMu.Unlock()
}

func (n *Network) getDeviceByIface(iface string) *device {
	for _, value := range n.devices {
		if value.iface == iface {
			return value
		}
	}
	return nil
}

func (n *Network) findDevice(pathOrIface string) *device {
	n.devicesMu.Lock()
	defer n.devicesMu.Unlock()

	if strings.HasPrefix(pathOrIface, "/org/freedesktop/NetworkManager") {
		return n.devices[dbus.ObjectPath(pathOrIface)]
	}
	return n.getDeviceByIface(pathOrIface)
}

func (n *Network) enableNetworking() error {
	enabled, err := n.nmManager.NetworkingEnabled().Get(0)
	if err != nil {
		return err
	}

	if enabled {
		return nil
	}

	return n.nmManager.Enable(0, true)
}

func (n *Network) enableWireless() error {
	enabled, err := n.nmManager.WirelessEnabled().Get(0)
	if err != nil {
		return err
	}

	if enabled {
		return nil
	}

	// 飞行模式开启，不激活wifi
	if v, err := n.airplane.WifiEnabled().Get(0); err != nil {
		logger.Warning(err)
	} else if v {
		logger.Debug("disable wifi because airplane mode is on")
		return nil
	}

	return n.nmManager.WirelessEnabled().Set(0, true)
}

func (n *Network) ToggleWirelessEnabled() (enabled bool, busErr *dbus.Error) {
	enabled, err := n.toggleWirelessEnabled()
	return enabled, dbusutil.ToError(err)
}

func (n *Network) toggleWirelessEnabled() (bool, error) {
	enabled, err := n.nmManager.WirelessEnabled().Get(0)
	if err != nil {
		return false, err
	}
	enabled = !enabled

	err = n.nmManager.WirelessEnabled().Set(0, enabled)
	if err != nil {
		return false, err
	}

	device := n.getWirelessDevices()
	for _, d := range device {
		devPath := d.nmDevice.Path_()
		_, err = n.enableDevice(string(devPath), enabled)
		if err != nil {
			logger.Warningf("failed to enable %v device %s: %v", enabled, devPath, err)
		}
	}

	return enabled, nil
}

type connSettings struct {
	nmConn   networkmanager.ConnectionSettings
	uuid     string
	settings map[string]map[string]dbus.Variant
}

func (n *Network) disableVpn() {
	connSettingsList, err := n.getConnSettingsListByConnType("vpn")
	if err != nil {
		logger.Warning(err)
		return
	}

	for _, connSettings := range connSettingsList {
		n.deactivateConnectionByUuid(connSettings.uuid)
	}
}

func (n *Network) vpnEnabledWriteCb(write *dbusutil.PropertyWrite) *dbus.Error {
	enabled := write.Value.(bool)
	logger.Debug("set VpnEnabled", enabled)

	// if enable is false, disconnect all vpn.
	// if enable is true, auto connect vpn connections in session/network module.
	if !enabled {
		n.disableVpn()
	}

	n.configMu.Lock()
	n.config.VpnEnabled = enabled
	err := n.saveConfig()
	n.configMu.Unlock()
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	return nil
}

func (n *Network) deactivateConnectionByUuid(uuid string) {
	activeConns, err := n.getActiveConnectionsByUuid(uuid)
	if err != nil {
		return
	}
	for _, activeConn := range activeConns {
		logger.Debug("DeactivateConnection:", uuid, activeConn.Path_())

		state, err := activeConn.State().Get(0)
		if err != nil {
			logger.Warning(err)
			return
		}

		if state == nm.NM_ACTIVE_CONNECTION_STATE_ACTIVATING ||
			state == nm.NM_ACTIVE_CONNECTION_STATE_ACTIVATED {

			err = n.nmManager.DeactivateConnection(0, activeConn.Path_())
			if err != nil {
				logger.Warning(err)
				return
			}
		}
	}
}

func (n *Network) getActiveConnectionsByUuid(uuid string) ([]networkmanager.ActiveConnection,
	error) {
	activeConnPaths, err := n.nmManager.ActiveConnections().Get(0)
	if err != nil {
		return nil, err
	}
	var result []networkmanager.ActiveConnection
	for _, activeConnPath := range activeConnPaths {
		activeConn, err := networkmanager.NewActiveConnection(n.getSysBus(), activeConnPath)
		if err != nil {
			logger.Warning(err)
			continue
		}

		uuid0, err := activeConn.Uuid().Get(0)
		if err != nil {
			logger.Warning(err)
			continue
		}

		if uuid0 == uuid {
			result = append(result, activeConn)
		}
	}
	return result, nil
}

func (n *Network) getConnSettingsListByConnType(connType string) ([]*connSettings, error) {
	connPaths, err := n.nmSettings.ListConnections(0)
	if err != nil {
		return nil, err
	}

	var result []*connSettings
	for _, connPath := range connPaths {
		conn, err := networkmanager.NewConnectionSettings(n.getSysBus(), connPath)
		if err != nil {
			logger.Warning(err)
			continue
		}

		settings, err := conn.GetSettings(0)
		if err != nil {
			logger.Warning(err)
			continue
		}

		if getSettingConnectionType(settings) != connType {
			continue
		}

		uuid := getSettingConnectionUuid(settings)
		if uuid != "" {
			cs := &connSettings{
				nmConn:   conn,
				uuid:     uuid,
				settings: settings,
			}
			result = append(result, cs)
		}
	}
	return result, nil
}

// get devices may failed because dde-system-daemon and NetworkManager are started very nearly,
// need call method 5 times.
func (n *Network) addDevicesWithRetry() {
	// try get all devices 5 times
	for i := 0; i < 5; i++ {
		devicePaths, err := n.nmManager.GetDevices(0)
		if err != nil {
			logger.Warning(err)
			// sleep for 1 seconds, and retry get devices
			time.Sleep(1 * time.Second)
		} else {
			n.addAndCheckDevices(devicePaths)
			// if success, break
			break
		}
	}
}

// add and check devices
func (n *Network) addAndCheckDevices(devicePaths []dbus.ObjectPath) {
	// add device
	for _, devPath := range devicePaths {
		err := n.addDevice(devPath)
		if err != nil {
			logger.Warning(err)
			continue
		}
	}
}
