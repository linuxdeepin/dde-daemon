package network

import (
	dbus "github.com/godbus/dbus"
	ipwatchd "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.ipwatchd"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func activateSystemService(sysBus *dbus.Conn, serviceName string) error {
	sysBusObj := ofdbus.NewDBus(sysBus)

	has, err := sysBusObj.NameHasOwner(0, serviceName)
	if err != nil {
		return err
	}

	if has {
		logger.Debug("service activated", serviceName)
		return nil
	}
	_, err = sysBusObj.StartServiceByName(0, serviceName, 0)
	if err != nil {
		return err
	}

	return nil
}

func (m *Manager) initIPConflictManager(sysBus *dbus.Conn) {
	m.sysIPWatchD = ipwatchd.NewIPWatchD(sysBus)
	m.sysIPWatchD.InitSignalExt(m.sysSigLoop, true)
	err := activateSystemService(sysBus, m.sysIPWatchD.ServiceName_())
	if err != nil {
		logger.Warning(err)
	}

	_, err = m.sysIPWatchD.ConnectIPConflict(func(ip, smac, dmac string) {
		err := m.service.Emit(manager, "IPConflict", ip, dmac)
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) RequestIPConflictCheck(ip, ifc string) *dbus.Error {
	mac, err := m.sysIPWatchD.RequestIPConflictCheck(0, ip, ifc)
	if err != nil {
		return dbusutil.ToError(err)
	}

	if len(mac) != 0 {
		err := m.service.Emit(manager, "IPConflict", ip, mac)
		if err != nil {
			logger.Warning(err)
		}
	}

	return nil
}
