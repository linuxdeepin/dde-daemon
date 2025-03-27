// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"github.com/linuxdeepin/go-x11-client/ext/input"
)

// 放和 greeter-display-daemon 有关的代码

func (m *Manager) doXISelectEvents(evMask uint32) error {
	root := m.xConn.GetDefaultScreen().Root
	err := input.XISelectEventsChecked(m.xConn, root, []input.EventMask{
		{
			DeviceId: input.DeviceAllMaster,
			Mask:     []uint32{evMask},
		},
	}).Check(m.xConn)
	return err
}

func (m *Manager) beginMoveMouse() {
	if m.cursorShowed {
		return
	}
	err := m.doShowCursor(true)
	if err != nil {
		logger.Warning(err)
	}
	m.cursorShowed = true
}

func (m *Manager) beginTouch() {
	if !m.cursorShowed {
		return
	}
	err := m.doShowCursor(false)
	if err != nil {
		logger.Warning(err)
	}
	m.cursorShowed = false
}

func (m *Manager) doShowCursor(show bool) error {
	return m.mm.showCursor(show)
}
