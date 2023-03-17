// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package network

import (
	"time"

	dbus "github.com/godbus/dbus/v5"
	ipwatch "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.ipwatch1"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
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
	m.sysIPWatchD = ipwatch.NewIPWatch(sysBus)
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
	ch := make(chan *dbus.Call, 1)
	m.sysIPWatchD.GoRequestIPConflictCheck(0, ch, ip, ifc)
	go func() {
		select {
		case ret := <-ch:
			mac := ""
			err := ret.Store(&mac)
			if err != nil {
				logger.Warning(err)
				return
			}
			m.service.Emit(manager, "IPConflict", ip, mac)
		case <-time.After(1 * time.Second):
			return
		}
	}()

	return nil
}
