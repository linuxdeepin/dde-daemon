// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package timedate

import (
	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/strv"
)

func (m *Manager) listenPropChanged() {
	m.td.InitSignalExt(m.systemSigLoop, true)
	err := m.td.CanNTP().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		logger.Debug("property CanTTP changed to", value)

		m.PropsMu.Lock()
		m.setPropCanNTP(value)
		m.PropsMu.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = m.td.NTP().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		logger.Debug("property NTP changed to", value)

		m.PropsMu.Lock()
		m.setPropNTP(value)
		m.PropsMu.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = m.td.LocalRTC().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		logger.Debug("property LocalRTC changed to", value)

		m.PropsMu.Lock()
		m.setPropLocalRTC(value)
		m.PropsMu.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}

	err = m.td.Timezone().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}
		logger.Debug("property Timezone changed to", value)
		m.PropsMu.Lock()
		// If system timezone is Asia/Shanghai, check installer configuration
		// If installer configured a city from customTimeZoneList, use installer timezone
		actualTimezone := value
		if value == "Asia/Shanghai" {
			installerTimeZone, err := m.getSystemTimeZoneFromInstaller()
			if err == nil && installerTimeZone != "" &&
			   strv.Strv(customTimeZoneList).Contains(installerTimeZone) {
				actualTimezone = installerTimeZone
				logger.Debugf("Use installer timezone %s instead of %s", installerTimeZone, value)
			}
		}
		m.setPropTimezone(actualTimezone)
		m.PropsMu.Unlock()
		err = m.dConfigManager.SetValue(dbus.Flags(0), dSettingsTimeZone, dbus.MakeVariant(actualTimezone))
		if err != nil {
			logger.Warning(err)
		}
		err := m.AddUserTimezone(m.Timezone)
		if err != nil {
			logger.Warning("AddUserTimezone error:", err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	m.setter.InitSignalExt(m.systemSigLoop, true)
	err = m.setter.NTPServer().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}

		m.PropsMu.Lock()
		m.setPropNTPServer(value)
		m.PropsMu.Unlock()
		err = m.dConfigManager.SetValue(dbus.Flags(0), dSettingsNTPServer, dbus.MakeVariant(value))
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}
