// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	hostname1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.hostname1"
	"github.com/linuxdeepin/go-lib/arch"
)

func (m *Manager) initLidSwitch() {
	hostnameObj := hostname1.NewHostname(m.service.Conn())
	chassis, err := hostnameObj.Chassis().Get(0)
	if err != nil {
		logger.Warningf("failed to get chassis type(%v)", err)
		return
	}
	if chassis == "convertible" || chassis == "laptop" {
		if arch.Get() == arch.Sunway && isSWLidStateFileExist() {
			m.initLidSwitchSW()
		} else {
			err := m.initLidSwitchByUPower()
			if err != nil {
				logger.Warningf("failed to init watch lid switch by upower(%v),start init watch by gudev", err)
				m.initLidSwitchCommon()
			}
		}
	}
	logger.Debug("hasLidSwitch:", m.HasLidSwitch)
}

func (m *Manager) handleLidSwitchEvent(closed bool) {
	if closed {
		logger.Info("Lid Closed")
		err := m.service.Emit(m, "LidClosed")
		if err != nil {
			logger.Warning(err)
		}
	} else {
		logger.Info("Lid Opened")
		err := m.service.Emit(m, "LidOpened")
		if err != nil {
			logger.Warning(err)
		}
	}
}
