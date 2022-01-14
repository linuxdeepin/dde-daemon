package airplane_mode

import (
	"errors"

	"github.com/godbus/dbus"
	keyevent "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.keyevent"
	polkit "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.policykit1"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	dbusServiceName = "com.deepin.daemon.AirplaneMode"
	dbusPath        = "/com/deepin/daemon/AirplaneMode"
	dbusInterface   = dbusServiceName

	actionId = "com.deepin.daemon.airplane-mode.enable-disable-any"
)

//go:generate dbusutil-gen -type Manager manager.go
//go:generate dbusutil-gen em -type Manager

type Manager struct {
	service *dbusutil.Service
	sigLoop *dbusutil.SignalLoop

	Enabled          bool
	WifiEnabled      bool
	BluetoothEnabled bool

	// all rfkill module config
	config *Config
}

// NewManager create manager
func newManager(service *dbusutil.Service) *Manager {
	mgr := &Manager{
		service: service,
		sigLoop: dbusutil.NewSignalLoop(service.Conn(), 10),
		config:  NewConfig(),
	}
	err := mgr.init()
	if err != nil {
		logger.Warningf("init manager failed, err: %v", err)
	}
	return mgr
}

func (mgr *Manager) GetInterfaceName() string {
	return dbusInterface
}

func (mgr *Manager) DumpState() *dbus.Error {
	return nil
}

// Enable enable or disable all rfkill type
func (mgr *Manager) Enable(sender dbus.Sender, blocked bool) *dbus.Error {
	// check auth
	err := checkAuthorization(actionId, string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	// try to block
	err = mgr.block(AllRadioType, blocked)
	if err != nil {
		logger.Warningf("block all radio failed, err: %v", err)
		return dbusutil.ToError(err)
	}
	// set and emit signal
	mgr.setPropEnabled(blocked)
	// it is ok, if save config failed
	err = mgr.config.SaveConfig()
	if err != nil {
		logger.Warningf("save config failed, err: %v", err)
	}
	return nil
}

// EnableWifi enable or disable rfkill wlan
func (mgr *Manager) EnableWifi(sender dbus.Sender, blocked bool) *dbus.Error {
	err := checkAuthorization(actionId, string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	// try to block
	err = mgr.block(WlanRadioType, blocked)
	if err != nil {
		logger.Warningf("block wifi radio failed, err: %v", err)
		return dbusutil.ToError(err)
	}
	// set and emit signal
	mgr.setPropWifiEnabled(blocked)
	// it is ok, if save config failed
	err = mgr.config.SaveConfig()
	if err != nil {
		logger.Warningf("save config failed, err: %v", err)
	}
	return nil
}

func (mgr *Manager) EnableBluetooth(sender dbus.Sender, blocked bool) *dbus.Error {
	err := checkAuthorization(actionId, string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	// try to block
	err = mgr.block(BluetoothRadioType, blocked)
	if err != nil {
		logger.Warningf("block bluetooth radio failed, err: %v", err)
		return dbusutil.ToError(err)
	}
	// set and emit signal
	mgr.setPropBluetoothEnabled(blocked)
	// it is ok, if save config failed
	err = mgr.config.SaveConfig()
	if err != nil {
		logger.Warningf("save config failed, err: %v", err)
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
	mgr.monitor()

	// loop start
	mgr.sigLoop.Start()
	return nil
}

// recover recover origin state from config
func (mgr *Manager) recover() {
	logger.Debug("recover last state")
	// bluetooth
	mgr.BluetoothEnabled = mgr.config.GetBlocked(BluetoothRadioType)
	// bluetooth enabled, means bluetooth soft/hard block is enabled last time
	// should recover block state here
	err := mgr.block(BluetoothRadioType, mgr.BluetoothEnabled)
	if err != nil {
		logger.Warningf("recover bluetooth failed, state: %v, err: %v", mgr.WifiEnabled, err)
	}

	// wlan
	mgr.WifiEnabled = mgr.config.GetBlocked(WlanRadioType)
	// wifi enabled, means wlan soft/hard block is enabled last time
	// should recover block state here
	err = mgr.block(WlanRadioType, mgr.WifiEnabled)
	if err != nil {
		logger.Warningf("recover wifi failed, state: %v, err: %v", mgr.WifiEnabled, err)
	}

	// all
	mgr.Enabled = mgr.config.GetBlocked(AllRadioType)
	// enabled, means all soft/hard block is enabled last time
	// should recover block state here
	err = mgr.block(AllRadioType, mgr.Enabled)
	if err != nil {
		logger.Warningf("recover all failed, state: %v, err: %v", mgr.WifiEnabled, err)
	}

	return
}

// block use rfkill to block wifi
func (mgr *Manager) block(typ RadioType, block bool) error {
	var err error
	// get operator
	module := getRadioModule(typ)
	// check if type is legal
	if module == nil {
		return errors.New("operation not support")
	}
	// try to un/block
	if block {
		err = module.Block()
	} else {
		err = module.Unblock()
	}
	// if un/block failed, dont try to update block state
	if err != nil {
		return err
	}
	mgr.config.SetBlocked(typ, block)
	return nil
}

// monitor use to monitor udev event
func (mgr *Manager) monitor() {
	logger.Info("begin monitor radio key event")
	// create system bus
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warningf("create system bus failed, err: %v", err)
		return
	}
	// create key event
	keyMonitor := keyevent.NewKeyEvent(sysBus)
	keyMonitor.InitSignalExt(mgr.sigLoop, true)
	// monitor key event
	_, err = keyMonitor.ConnectKeyEvent(func(keycode uint32, pressed bool, ctrlPressed bool, shiftPressed bool, altPressed bool, superPressed bool) {
		logger.Debugf("key: %v, pressed: %v", keycode, pressed)
		// only monitor rfkill pressed release event
		key := RadioKey(keycode)
		if key.Ignore() || pressed {
			return
		}
		// once wlan bluetooth or rfkill key event is received
		// should refresh state here
		mgr.refresh(key.ToRadioType())
	})
}

// refresh when rfkill event arrived,
// should check all rfkill state
func (mgr *Manager) refresh(typ RadioType) {
	// get operator
	module := getRadioModule(typ)
	// check if type is legal
	if module == nil {
		return
	}
	// check module len first, if dont exist any device,
	// should ignore this command
	if module.Len() <= 0 {
		logger.Debugf("current module %v has no devices, should ignore", module)
		return
	}
	// check is module is blocked
	isBlocked := module.IsBlocked()
	switch typ {
	case BluetoothRadioType:
		mgr.setPropBluetoothEnabled(isBlocked)
		logger.Debugf("refresh bluetooth blocked state: %v", isBlocked)
	case WlanRadioType:
		mgr.setPropWifiEnabled(isBlocked)
		logger.Debugf("refresh wifi blocked state: %v", isBlocked)
	case AllRadioType:
		mgr.setPropEnabled(isBlocked)
		logger.Debugf("refresh all blocked state: %v", isBlocked)
	}
	mgr.config.SetBlocked(typ, isBlocked)
	// save rfkill key event result to config file
	err := mgr.config.SaveConfig()
	if err != nil {
		logger.Warningf("save rfkill config file failed, err: %v", err)
	}
	logger.Debugf("rfkill state, bluetooth: %v, wifi: %v, airplane: %v", mgr.BluetoothEnabled, mgr.WifiEnabled, mgr.Enabled)
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
