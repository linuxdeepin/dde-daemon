package airplane_mode

import (
	"bufio"
	"errors"
	"os/exec"
	"strings"

	"github.com/godbus/dbus"
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
	go mgr.monitor()
	return nil
}

// recover recover origin state from config
func (mgr *Manager) recover() {
	logger.Debug("recover last state")
	// bluetooth
	mgr.BluetoothEnabled = mgr.config.GetBlocked(RfkillBluetooth)
	err := rfkillAction(RfkillBluetooth, mgr.BluetoothEnabled)
	if err != nil {
		logger.Warningf("recover all action failed, err: %v", err)
	}
	// wlan
	mgr.WifiEnabled = mgr.config.GetBlocked(RfkillWlan)
	err = rfkillAction(RfkillWlan, mgr.WifiEnabled)
	if err != nil {
		logger.Warningf("recover all action failed, err: %v", err)
	}
	// all
	mgr.Enabled = mgr.config.GetBlocked(RfkillAll)
	return
}

// monitor use to monitor udev event
func (mgr *Manager) monitor() {
	logger.Debug("begin monitor")
	// monitor udev event
	args := []string{"rfkill", "event"}
	cmd := exec.Command("/bin/bash", "-c", strings.Join(args, " "))
	output, err := cmd.StdoutPipe()
	if err != nil {
		logger.Warningf("out pipe failed, err: %v", err)
		return
	}
	logger.Debug("open rfkill event std out successfully")
	// begin to read rfkill event
	reader := bufio.NewReader(output)
	go func() {
		for {
			// read output
			buf, _, err := reader.ReadLine()
			if err != nil {
				logger.Warningf("read rfkill event failed, msg: %v err: %v", string(buf), err)
				return
			}
			logger.Debug(">>>>>> rfkill event received, need update rfkill state")
			// once receive event, should update list
			mgr.update()
		}
	}()

	// run rfkill command
	err = cmd.Run()
	if err != nil {
		logger.Warningf("run rfkill event, err: %v", err)
	}
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

	// rfkill action
	err = rfkillAction(RfkillAll, blocked)
	if err != nil {
		logger.Warningf("rfkill action failed, err: %v", err)
		return dbusutil.ToError(err)
	}
	return nil
}

// EnableWifi enable or disable rfkill wlan
func (mgr *Manager) EnableWifi(sender dbus.Sender, blocked bool) *dbus.Error {
	err := checkAuthorization(actionId, string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	// rfkill action
	err = rfkillAction(RfkillWlan, blocked)
	if err != nil {
		logger.Warningf("rfkill action failed, err: %v", err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (m *Manager) EnableBluetooth(sender dbus.Sender, blocked bool) *dbus.Error {
	err := checkAuthorization(actionId, string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	// rfkill action
	err = rfkillAction(RfkillBluetooth, blocked)
	if err != nil {
		logger.Warningf("rfkill action failed, err: %v", err)
		return dbusutil.ToError(err)
	}
	return nil
}

// update when rfkill event arrived,
// should check all rfkill state
func (mgr *Manager) update() {
	// check bluetooth state
	bltBlocked := isModuleBlocked(RfkillBluetooth)
	logger.Debugf("bluetooth enabled state: %v", bltBlocked)
	// check wifi state
	wlanBlocked := isModuleBlocked(RfkillWlan)
	logger.Debugf("wlan enabled state: %v", wlanBlocked)

	// set bluetooth enabled state
	mgr.setPropBluetoothEnabled(bltBlocked)
	mgr.config.SetBlocked(RfkillBluetooth, bltBlocked)
	// set wlan enabled state
	mgr.setPropWifiEnabled(wlanBlocked)
	mgr.config.SetBlocked(RfkillWlan, wlanBlocked)

	// if current bluetooth is disabled and wifi is also disabled
	// the state is diff with last time
	if bltBlocked && wlanBlocked && !mgr.Enabled {
		mgr.setPropEnabled(true)
		mgr.config.SetBlocked(RfkillAll, true)
	} else if (!bltBlocked || !wlanBlocked) && mgr.Enabled {
		mgr.setPropEnabled(false)
		mgr.config.SetBlocked(RfkillAll, false)
	}

	logger.Debugf("rfkill state, bluetooth: %v, wifi: %v, airplane: %v", mgr.BluetoothEnabled, mgr.WifiEnabled, mgr.Enabled)

	// save current config to file
	err := mgr.config.SaveConfig()
	if err != nil {
		logger.Warningf("save rfkill config file failed, err: %v", err)
	}
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
