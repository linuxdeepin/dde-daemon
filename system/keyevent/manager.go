// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package keyevent

import (
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

//go:generate dbusutil-gen em -type Manager

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

const (
	touchpadSwitchFile = "/proc/uos/touchpad_switch"
)

// 允许发送的按键列表
var allowList = map[uint32]bool{
	KEY_TOUCHPAD_TOGGLE: true,
	KEY_POWER:           true,
	KEY_BLUETOOTH:       true,
	KEY_WLAN:            true,
	KEY_RFKILL:          true,
	KEY_TOUCHPAD_ON:     true,
	KEY_TOUCHPAD_OFF:    true,
	KEY_FN_ESC:          true,
	KEY_MICMUTE:         true,
	KEY_SWITCHVIDEOMODE: true,
	KEY_SETUP:           true,
	KEY_CYCLEWINDOWS:    true,
	KEY_MODE:            true,
	KEY_SCREENLOCK:      true,
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
		if ev.Keycode == KEY_TOUCHPAD_TOGGLE && pressed {
			go func() {
				_, err := os.Stat(touchpadSwitchFile)
				if err != nil {
					logger.Warning(err)
					return
				}
				content, err := os.ReadFile(touchpadSwitchFile)
				if err != nil {
					logger.Warning(err)
					return
				}
				enable := strings.Contains(string(content), "enable")
				systemBus, err := dbus.SystemBus()
				if err != nil {
					logger.Warning(err)
					return
				}
				obj := systemBus.Object("com.deepin.system.InputDevices", "/com/deepin/system/InputDevices/TouchPad")
				err = obj.Call("com.deepin.system.InputDevices.TouchPad.SetTouchPadEnable", 0, !enable).Err
				if err != nil {
					logger.Warning(err)
					return
				}
			}()
		}
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
