/*
 * Copyright (C) 2016 ~ 2020 Deepin Technology Co., Ltd.
 *
 * Author:     hubenchang <hubenchang@uniontech.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package keyevent

import (
	"pkg.deepin.io/lib/dbusutil"
)

type Manager struct {
	service *dbusutil.Service
	quit    chan bool
	ch      chan *KeyEvent

	leftCtrlPressed  bool
	leftShiftPressed bool
	leftAltPressed   bool
	leftSuperPressed bool

	rightCtrlPressed  bool
	rightShiftPressed bool
	rightAltPressed   bool
	rightSuperPressed bool

	// nolint
	signals *struct {
		KeyEvent struct {
			keycode      uint32
			pressed      bool // true:按下事件，false松开事件
			ctrlPressed  bool // ctrl 是否处于按下状态
			shiftPressed bool // shift 是否处于按下状态
			altPressed   bool // alt 是否处于按下状态
			superPressed bool // super 是否处于按下状态
		}
	}
}

// 允许发送的按键列表
var allowList = map[uint32]bool{
	KEY_TOUCHPAD_TOGGLE: true,
	KEY_POWER:           true,
}

func newManager(service *dbusutil.Service) *Manager {
	m := &Manager{
		service: service,
		quit:    make(chan bool),
		ch:      make(chan *KeyEvent, 64),
	}

	return m
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) start() {
	addKeyEventChannel(m.ch)
	startKeyEventMonitor()

	go m.monitor()
}

func (m *Manager) stop() {
	stopKeyEventMonitor()
	m.quit <- true
}

func (m *Manager) monitor() {
	for {
		select {
		case ev := <-m.ch:
			logger.Debugf("event keycode(%d) state(%v)", ev.Keycode, ev.State)
			m.handleEvent(ev)
		case <-m.quit:
			logger.Debug("key event monitor stop")
			return
		}
	}
}

func (m *Manager) handleEvent(ev *KeyEvent) {
	pressed := ev.State == KEY_STATE_PRESSED
	// 保存修饰键的状态
	switch ev.Keycode {
	case KEY_LEFTCTRL:
		m.leftCtrlPressed = pressed
	case KEY_RIGHTCTRL:
		m.rightCtrlPressed = pressed
	case KEY_LEFTALT:
		m.leftAltPressed = pressed
	case KEY_RIGHTALT:
		m.rightAltPressed = pressed
	case KEY_LEFTSHIFT:
		m.leftShiftPressed = pressed
	case KEY_RIGHTSHIFT:
		m.rightShiftPressed = pressed
	case KEY_LEFTMETA:
		m.leftSuperPressed = pressed
	case KEY_RIGHTMETA:
		m.leftSuperPressed = pressed
	}

	// 发送DBus signal通知按键事件
	allow := allowList[ev.Keycode]
	if allow {
		m.emitKeyEvent(ev)
	}
}

func (m *Manager) emitKeyEvent(ev *KeyEvent) {
	err := m.service.Emit(
		m,
		"KeyEvent",
		ev.Keycode,
		ev.State == KEY_STATE_PRESSED,
		m.leftCtrlPressed || m.rightCtrlPressed,
		m.leftShiftPressed || m.rightShiftPressed,
		m.leftAltPressed || m.rightAltPressed,
		m.leftSuperPressed || m.rightSuperPressed,
	)

	if err != nil {
		logger.Warning(err)
	}
}
