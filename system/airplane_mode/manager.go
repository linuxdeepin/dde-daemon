package airplane_mode

import (
	"errors"

	"github.com/godbus/dbus"
	polkit "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.policykit1"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	dbusServiceNameV20 = "com.deepin.daemon.AirplaneMode"
	dbusPathV20        = "/com/deepin/daemon/AirplaneMode"
	dbusInterfaceV20   = dbusServiceNameV20

	dbusServiceNameV23 = "org.deepin.daemon.AirplaneMode1"
	dbusPathV23        = "/org/deepin/daemon/AirplaneMode1"
	dbusInterfaceV23   = dbusServiceNameV23

	actionId = "com.deepin.daemon.airplane-mode.enable-disable-any"
)

type device struct {
	typ  rfkillType
	soft rfkillState
	hard rfkillState
}

//go:generate dbusutil-gen -type Manager manager.go
//go:generate dbusutil-gen em -type Manager

type Manager struct {
	service *dbusutil.Service
	devices map[uint32]device

	// Airplane Mode status
	Enabled          bool
	HasAirplaneMode  bool
	WifiEnabled      bool
	BluetoothEnabled bool

	// all rfkill module config
	config *Config
}

// NewManager create manager
func newManager(service *dbusutil.Service) *Manager {
	mgr := &Manager{
		service: service,
		devices: make(map[uint32]device),
		config:  NewConfig(),
	}
	err := mgr.init()
	if err != nil {
		logger.Warningf("init manager failed, err: %v", err)
	}
	return mgr
}

func (mgr *Manager) DumpState() *dbus.Error {
	return nil
}

// Enable enable or disable *Airplane Mode*, isn't enable the devices
func (mgr *Manager) Enable(sender dbus.Sender, enableAirplaneMode bool) *dbus.Error {
	// check auth
	err := checkAuthorization(actionId, string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	// try to block
	err = mgr.block(rfkillTypeAll, enableAirplaneMode)
	if err != nil {
		logger.Warningf("block all radio failed, err: %v", err)
		return dbusutil.ToError(err)
	}
	return nil
}

// EnableWifi enable or disable *Airplane Mode* for wlan, isn't enable the wlan devices
func (mgr *Manager) EnableWifi(sender dbus.Sender, enableAirplaneMode bool) *dbus.Error {
	err := checkAuthorization(actionId, string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	// try to block
	err = mgr.block(rfkillTypeWifi, enableAirplaneMode)
	if err != nil {
		logger.Warningf("block wifi radio failed, err: %v", err)
		return dbusutil.ToError(err)
	}
	return nil
}

// EnableBluetooth enable or disable *Airplane Mode* for bluetooth, isn't enable the bluetooth devices
func (mgr *Manager) EnableBluetooth(sender dbus.Sender, enableAirplaneMode bool) *dbus.Error {
	err := checkAuthorization(actionId, string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	// try to block
	err = mgr.block(rfkillTypeBT, enableAirplaneMode)
	if err != nil {
		logger.Warningf("block bluetooth radio failed, err: %v", err)
		return dbusutil.ToError(err)
	}
	return nil
}

// init use to init manager
func (mgr *Manager) init() error {
	// load config file
	err := mgr.config.LoadConfig()
	if err != nil {
		logger.Debugf("load airplane module config failed, err: %v", err)
	}

	// recover
	mgr.recover()

	// use goroutine to monitor rfkill event
	go mgr.listenRfkill()

	return nil
}

// recover recover origin state from config
func (mgr *Manager) recover() {
	logger.Debug("recover last state")
	// bluetooth
	mgr.BluetoothEnabled = mgr.config.GetBlocked(rfkillTypeBT)
	// bluetooth enabled, means bluetooth soft/hard block is enabled last time
	// should recover block state here
	err := mgr.block(rfkillTypeBT, mgr.BluetoothEnabled)
	if err != nil {
		logger.Warningf("recover bluetooth failed, state: %v, err: %v", mgr.WifiEnabled, err)
	}

	// wlan
	mgr.WifiEnabled = mgr.config.GetBlocked(rfkillTypeWifi)
	// wifi enabled, means wlan soft/hard block is enabled last time
	// should recover block state here
	err = mgr.block(rfkillTypeWifi, mgr.WifiEnabled)
	if err != nil {
		logger.Warningf("recover wifi failed, state: %v, err: %v", mgr.WifiEnabled, err)
	}

	// all
	mgr.Enabled = mgr.config.GetBlocked(rfkillTypeAll)
	// enabled, means all soft/hard block is enabled last time
	// should recover block state here
	err = mgr.block(rfkillTypeAll, mgr.Enabled)
	if err != nil {
		logger.Warningf("recover all failed, state: %v, err: %v", mgr.WifiEnabled, err)
	}

	return
}

// block use rfkill to block wifi
func (mgr *Manager) block(typ rfkillType, enableAirplaneMode bool) error {
	state := rfkillStateUnblock
	if enableAirplaneMode {
		state = rfkillStateBlock
	}
	return rfkillAction(typ, state)
}

func checkAuthorization(actionId string, sysBusName string) error {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	authority := polkit.NewAuthority(systemBus)
	subject := polkit.MakeSubject(polkit.SubjectKindSystemBusName)
	subject.SetDetail("name", sysBusName)

	ret, err := authority.CheckAuthorization(0, subject, actionId,
		nil, polkit.CheckAuthorizationFlagsAllowUserInteraction, "")
	if err != nil {
		return err
	}
	if !ret.IsAuthorized {
		return errors.New("not authorized")
	}
	return nil
}
