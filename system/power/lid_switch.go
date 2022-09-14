// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"github.com/linuxdeepin/go-lib/arch"
)

func (m *Manager) initLidSwitch() {
	if arch.Get() == arch.Sunway && isSWLidStateFileExist() {
		m.initLidSwitchSW()
	} else {
		m.initLidSwitchCommon()
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
