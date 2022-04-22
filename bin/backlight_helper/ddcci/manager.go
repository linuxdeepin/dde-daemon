package ddcci

import (
	"fmt"
	"sync"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
	x "github.com/linuxdeepin/go-x11-client"
)

const (
	DbusPath        = "/com/deepin/daemon/helper/Backlight/DDCCI"
	dbusInterface   = "com.deepin.daemon.helper.Backlight.DDCCI"
	configManagerId = "org.desktopspec.ConfigManager"
)

var logger = log.NewLogger("backlight_helper/ddcci")

//go:generate dbusutil-gen em -type Manager
type Manager struct {
	service *dbusutil.Service
	ddcci   *ddcci

	PropsMu         sync.RWMutex
	configTimestamp x.Timestamp

	// 亮度调节方式，策略组配置
	supportDdcci bool

	configManagerPath dbus.ObjectPath
}

func NewManager(service *dbusutil.Service) (*Manager, error) {
	m := &Manager{}
	m.service = service

	systemConnObj := service.Conn().Object(configManagerId, "/")
	err := systemConnObj.Call(configManagerId+".acquireManager", 0, "org.deepin.dde.daemon", "org.deepin.dde.daemon.brightness", "").Store(&m.configManagerPath)
	if err != nil {
		logger.Warning(err)
	}
	m.supportDdcci = m.getSupportDdcci()

	if m.supportDdcci {
		var err error
		m.ddcci, err = newDDCCI()
		if err != nil {
			return nil, fmt.Errorf("brightness: failed to init ddc/ci: %s", err)
		}
	}

	return m, nil
}

func (m *Manager) getSupportDdcci() bool {
	systemConnObj := m.service.Conn().Object("org.desktopspec.ConfigManager", m.configManagerPath)
	var value bool
	err := systemConnObj.Call("org.desktopspec.ConfigManager.Manager.value", 0, "supportDdcci").Store(&value)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return value
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) CheckSupport(edidBase64 string) (bool, *dbus.Error) {
	if m.ddcci == nil {
		return false, nil
	}

	return m.ddcci.SupportBrightness(edidBase64), nil
}

func (m *Manager) GetBrightness(edidBase64 string) (int32, *dbus.Error) {
	if m.ddcci == nil {
		return 0, nil
	}

	if !m.ddcci.SupportBrightness(edidBase64) {
		err := fmt.Errorf("brightness: not support ddc/ci: %s", edidBase64)
		return 0, dbusutil.ToError(err)
	}

	brightness, err := m.ddcci.GetBrightness(edidBase64)
	return int32(brightness), dbusutil.ToError(err)
}

func (m *Manager) SetBrightness(edidBase64 string, value int32) *dbus.Error {
	if m.ddcci == nil {
		return nil
	}
	if !m.ddcci.SupportBrightness(edidBase64) {
		err := fmt.Errorf("brightness: not support ddc/ci: %s", edidBase64)
		return dbusutil.ToError(err)
	}

	err := m.ddcci.SetBrightness(edidBase64, int(value))
	return dbusutil.ToError(err)
}

func (m *Manager) RefreshDisplays() *dbus.Error {
	if m.ddcci == nil {
		return nil
	}
	err := m.ddcci.RefreshDisplays()
	return dbusutil.ToError(err)
}
