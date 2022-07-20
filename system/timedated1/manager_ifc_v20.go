package timedated

import "github.com/godbus/dbus"

const (
	dbusServiceNameV20 = "com.deepin.daemon.Timedated"
	dbusPathV20        = "/com/deepin/daemon/Timedated"
	dbusInterfaceV20   = dbusServiceName
)

func NewManagerV20(m *Manager) *ManagerV20 {
	managerV20 := &ManagerV20{
		m: m,
		NTPServer: m.NTPServer,
	}
	return managerV20
}

type ManagerV20 struct {
	m *Manager
	NTPServer      string
}

func (manager *ManagerV20) GetInterfaceName() string {
	return dbusInterfaceV20
}

func (manager *ManagerV20) SetNTPServer(sender dbus.Sender, server, message string) *dbus.Error {
	return manager.m.SetNTPServer(sender, server, message)
}

func (manager *ManagerV20) syncNTPServer(value string) {
	manager.NTPServer = value
	manager.emitPropChangedNTPServer(value)
}

func (manager *ManagerV20) emitPropChangedNTPServer(value string) error {
	return manager.m.service.EmitPropertyChanged(manager, "NTPServer", value)
}
