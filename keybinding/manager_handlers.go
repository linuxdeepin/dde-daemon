// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package keybinding

import (
	"fmt"
	"time"

	. "github.com/linuxdeepin/dde-daemon/keybinding/shortcuts"
)

func (m *Manager) shouldShowCapsLockOSD() bool {
	return m.gsKeyboard.GetBoolean(gsKeyShowCapsLockOSD)
}

func (m *Manager) initHandlers() {
	m.handlers = make([]KeyEventFunc, ActionTypeCount)
	logger.Debug("initHandlers", len(m.handlers))

	m.handlers[ActionTypeNonOp] = func(ev *KeyEvent) {
		logger.Debug("non-Op do nothing")
	}

	m.handlers[ActionTypeCallback] = func(ev *KeyEvent) {
		action := ev.Shortcut.GetAction()
		fn, ok := action.Arg.(func(ev *KeyEvent))
		if !ok {
			logger.Warning(ErrTypeAssertionFail)
			return
		}

		if fn != nil {
			go fn(ev)
		}
	}

	m.handlers[ActionTypeExecCmd] = func(ev *KeyEvent) {
		// prevent shortcuts such as switch window managers from being
		// triggered twice by mistake.
		const minExecCmdInterval = 600 * time.Millisecond
		type0 := ev.Shortcut.GetType()
		id := ev.Shortcut.GetId()
		if type0 == ShortcutTypeSystem && id == "wm-switcher" {
			now := time.Now()
			duration := now.Sub(m.lastExecCmdTime)
			logger.Debug("handle ActionTypeExecCmd duration:", duration)
			if 0 < duration && duration < minExecCmdInterval {
				logger.Debug("handle ActionTypeExecCmd ignore key event")
				return
			}
			m.lastExecCmdTime = now
		}

		action := ev.Shortcut.GetAction()
		arg, ok := action.Arg.(*ActionExecCmdArg)
		if !ok {
			logger.Warning(ErrTypeAssertionFail)
			return
		}

		go func() {
			err := m.execCmd(arg.Cmd, true)
			if err != nil {
				logger.Warning("execCmd error:", arg.Cmd, err)
			}
		}()
	}

	m.handlers[ActionTypeShowNumLockOSD] = func(ev *KeyEvent) {
		if _useWayland {
			m.handleKeyEventByWayland("numlock")
		} else {
			state, err := queryNumLockState(m.conn)
			if err != nil {
				logger.Warning(err)
				return
			}
			save := m.gsKeyboard.GetBoolean(gsKeySaveNumLockState)
			switch state {
			case NumLockOn:
				if save {
					m.NumLockState.Set(int32(NumLockOn))
				}
				showOSD("NumLockOn")
			case NumLockOff:
				if save {
					m.NumLockState.Set(int32(NumLockOff))
				}
				showOSD("NumLockOff")
			}
		}
	}

	m.handlers[ActionTypeShowCapsLockOSD] = func(ev *KeyEvent) {
		if _useWayland {
			m.handleKeyEventByWayland("capslock")
		} else {
			if !m.shouldShowCapsLockOSD() {
				return
			}

			state, err := queryCapsLockState(m.conn)
			if err != nil {
				logger.Warning(err)
				return
			}

			switch state {
			case CapsLockOff:
				showOSD("CapsLockOff")
			case CapsLockOn:
				showOSD("CapsLockOn")
			}
		}
	}

	m.handlers[ActionTypeOpenMimeType] = func(ev *KeyEvent) {
		action := ev.Shortcut.GetAction()
		mimeType, ok := action.Arg.(string)
		if !ok {
			logger.Warning(ErrTypeAssertionFail)
			return
		}

		go func() {
			desktop, err := queryAppDesktopByMime(mimeType)
			if err != nil {
				logger.Debug("queryAppByMime error:", err)
				return
			}

			err = m.runDesktopFile(desktop)
			if err != nil {
				logger.Warningf("launch %s error: %v", desktop, err)
			}
		}()
	}

	m.handlers[ActionTypeDesktopFile] = func(ev *KeyEvent) {
		action := ev.Shortcut.GetAction()

		go func() {
			err := m.runDesktopFile(action.Arg.(string))
			if err != nil {
				logger.Warning("runDesktopFile error:", err)
			}
		}()
	}

	m.handlers[ActionTypeAudioCtrl] = buildHandlerFromController(m.audioController)
	m.handlers[ActionTypeMediaPlayerCtrl] = buildHandlerFromController(m.mediaPlayerController)
	m.handlers[ActionTypeDisplayCtrl] = buildHandlerFromController(m.displayController)
	m.handlers[ActionTypeKbdLightCtrl] = buildHandlerFromController(m.kbdLightController)
	m.handlers[ActionTypeTouchpadCtrl] = buildHandlerFromController(m.touchPadController)

	m.handlers[ActionTypeSystemSuspend] = func(ev *KeyEvent) {
		m.systemSuspendByFront()
	}

	m.handlers[ActionTypeSystemLogOff] = func(ev *KeyEvent) {
		m.systemLogout()
	}

	m.handlers[ActionTypeSystemAway] = func(ev *KeyEvent) {
		m.systemAway()
	}

	// handle Switch Kbd Layout
	m.handlers[ActionTypeSwitchKbdLayout] = func(ev *KeyEvent) {
		logger.Debug("Switch Kbd Layout shortcut was disabled by TASK-67900")
	}

	m.handlers[ActionTypeShowControlCenter] = func(ev *KeyEvent) {
		err := m.execCmd("dbus-send --session --dest=org.deepin.dde.ControlCenter1  --print-reply /org/deepin/dde/ControlCenter1 org.deepin.dde.ControlCenter1.Show",
			false)
		if err != nil {
			logger.Warning("failed to show control center:", err)
		}
	}

	if m.shortcutManager == nil {
		m.shortcutManager = NewShortcutManager(m.conn, m.keySymbols, m.handleKeyEvent)
	}

	m.shortcutManager.SetAllModKeysReleasedCallback(func() {
		switch m.switchKbdLayoutState {
		case SKLStateWait:
			showOSD("DirectSwitchLayout")
			m.terminateSKLWait()
		case SKLStateOSDShown:
			showOSD("SwitchLayoutDone")
		case SKLStateNone:
			return
		}
		m.switchKbdLayoutState = SKLStateNone
	})
}

func (m *Manager) sklWait() {
	defer func() {
		logger.Debug("sklWait end")
		m.sklWaitQuit = nil
	}()

	m.sklWaitQuit = make(chan int)
	timer := time.NewTimer(350 * time.Millisecond)
	select {
	case <-m.sklWaitQuit:
		return
	case _, ok := <-timer.C:
		if !ok {
			logger.Error("Invalid ticker event")
			return
		}

		logger.Debug("timer fired")
		if m.switchKbdLayoutState == SKLStateWait {
			m.switchKbdLayoutState = SKLStateOSDShown
			showOSD("SwitchLayout")
		}
	}
}

func (m *Manager) terminateSKLWait() {
	if m.sklWaitQuit != nil {
		close(m.sklWaitQuit)
	}
}

type Controller interface {
	ExecCmd(cmd ActionCmd) error
	Name() string
}

func buildHandlerFromController(c Controller) KeyEventFunc {
	return func(ev *KeyEvent) {
		if c == nil {
			logger.Warning("controller is nil")
			return
		}
		name := c.Name()

		action := ev.Shortcut.GetAction()
		cmd, ok := action.Arg.(ActionCmd)
		if !ok {
			logger.Warning(ErrTypeAssertionFail)
			return
		}
		logger.Debugf("%v Controller exec cmd %v", name, cmd)
		if err := c.ExecCmd(cmd); err != nil {
			logger.Warning(name, "Controller exec cmd err:", err)
		}
	}
}

type ErrInvalidActionCmd struct {
	Cmd ActionCmd
}

func (err ErrInvalidActionCmd) Error() string {
	return fmt.Sprintf("invalid action cmd %v", err.Cmd)
}

type ErrIsNil struct {
	Name string
}

func (err ErrIsNil) Error() string {
	return fmt.Sprintf("%s is nil", err.Name)
}
