// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package keybinding

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/keybinding1/constants"
	. "github.com/linuxdeepin/dde-daemon/keybinding1/shortcuts"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
)

const (
	gsSchemaDefaultTerminal = "com.deepin.desktop.default-applications.terminal"
	gsKeyTerminalAppId      = "app-id"
)

func (m *Manager) shouldShowCapsLockOSD() bool {
	showCapsLockOSDValue, err := m.keyboardConfigMgr.Value(0, constants.DSettingsKeyShowCapsLockOSD)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return showCapsLockOSDValue.Value().(bool)
}

func (m *Manager) initHandlers() {
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
		action := ev.Shortcut.GetAction()
		arg, ok := action.Arg.(*ActionExecCmdArg)
		if !ok {
			logger.Warning(ErrTypeAssertionFail)
			return
		}

		go func() {
			var err error
			if arg.Cmd == "deepin-camera" {
				err = m.handleCheckCamera()
				if err != nil {
					logger.Warning(err)
				}
			} else {
				err = m.execCmd(arg.Cmd, true)
				if err != nil {
					logger.Warning("execCmd error:", err)
				}
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
			saveStateEnabledValue, err := m.keyboardConfigMgr.Value(0, constants.DSettingsKeySaveNumLockState)
			if err != nil {
				logger.Warning(err)
			}
			save := saveStateEnabledValue.Value().(bool)
			switch state {
			case NumLockOn:
				if save {
					m.keyboardConfigMgr.SetValue(0, constants.DSettingsKeyNumLockState, dbus.MakeVariant(NumLockOn))
				}
				showOSD("NumLockOn")
			case NumLockOff:
				if save {
					m.keyboardConfigMgr.SetValue(0, constants.DSettingsKeyNumLockState, dbus.MakeVariant(NumLockOff))
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
			err := m.execCmd(queryCommandByMime(mimeType), true)
			if err != nil {
				logger.Warning("execCmd error:", err)
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

	m.handlers[ActionTypeLaunchMimeType] = func(ev *KeyEvent) {
		action := ev.Shortcut.GetAction()
		mimeType, ok := action.Arg.(string)
		if !ok {
			logger.Warning(ErrTypeAssertionFail)
			return
		}
		go m.launchDefaultForMimeType(mimeType)
	}

	m.handlers[ActionTypeLaunchTerminal] = func(ev *KeyEvent) {
		go m.launchDefaultTerminal()
	}

	m.handlers[ActionTypeLockScreen] = func(ev *KeyEvent) {
		go m.lockScreen()
	}

	m.handlers[ActionTypeAudioCtrl] = buildHandlerFromController(m.audioController)
	m.handlers[ActionTypeMediaPlayerCtrl] = buildHandlerFromController(m.mediaPlayerController)
	m.handlers[ActionTypeDisplayCtrl] = buildHandlerFromController(m.displayController)
	m.handlers[ActionTypeKbdLightCtrl] = buildHandlerFromController(m.kbdLightController)
	m.handlers[ActionTypeTouchpadCtrl] = buildHandlerFromController(m.touchPadController)

	m.handlers[ActionTypeSystemSuspend] = func(ev *KeyEvent) {
		sleepLockValue, err := m.powerConfigMgr.Value(0, constants.DSettingsKeySleepLock)
		if err != nil {
			logger.Warning(err)
		}
		if sleepLockValue.Value().(bool) && !isTreeLand() {
			m.systemSuspendByFront()
		} else {
			m.systemSuspend()
		}
	}

	m.handlers[ActionTypeSystemLogOff] = func(ev *KeyEvent) {
		m.systemLogout()
	}

	m.handlers[ActionTypeSystemAway] = func(ev *KeyEvent) {
		m.systemAway()
	}

	// handle Switch Kbd Layout
	m.handlers[ActionTypeSwitchKbdLayout] = func(ev *KeyEvent) {
		logger.Warning("Switch Kbd Layout shortcut was disbaled by TASK-67900")
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

// launchDefaultForMimeType launches the default application for the given mime type
// via AM in-process, avoiding subprocess spawning.
func (m *Manager) launchDefaultForMimeType(mimeType string) {
	appInfo := gio.AppInfoGetDefaultForType(mimeType, false)
	if appInfo == nil {
		logger.Warning("no default app for mime type:", mimeType)
		return
	}
	defer appInfo.Unref()

	dAppInfo := gio.ToDesktopAppInfo(appInfo)
	desktopFile := filepath.Base(dAppInfo.GetFilename())

	err := m.runDesktopFile(desktopFile)
	if err != nil {
		logger.Warning("launch mime type error:", err)
	}
}

// launchDefaultTerminal launches the default terminal via AM in-process,
// using GSettings to find the configured terminal application.
func (m *Manager) launchDefaultTerminal() {
	settings := gio.NewSettings(gsSchemaDefaultTerminal)
	if settings == nil {
		logger.Warning("failed to get terminal settings")
		return
	}
	defer settings.Unref()

	appId := settings.GetString(gsKeyTerminalAppId)
	err := m.runDesktopFile(appId)
	if err != nil {
		logger.Warning("launch terminal error:", err)
	}
}

// lockScreen performs X11 grab cleanup and shows the lock screen via
// in-process DBus, avoiding subprocess spawning for dbus-send.
func (m *Manager) lockScreen() {
	origOption := m.getCurrentXkbOption()
	logger.Debug("lockScreen: orig option:", origOption)

	m.breakXGrab()
	defer m.restoreXkbOption(origOption)

	m.showLockFront()
}

// getCurrentXkbOption returns the current keyboard options from setxkbmap -query.
// Returns empty string if no options are set or an error occurs.
func (m *Manager) getCurrentXkbOption() string {
	out, err := exec.Command("setxkbmap", "-query").Output()
	if err != nil {
		logger.Warning("lockScreen: setxkbmap -query failed:", err)
		return ""
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "options:") {
			// line format: "options:   grp:alt_shift_toggle,terminate:ctrl_alt_bksp"
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return ""
}

// breakXGrab sets grab:break_actions and sends XF86Ungrab to break X11 keyboard grabs.
// On Wayland, the xdotool step is skipped (see Bug-224309).
func (m *Manager) breakXGrab() {
	exec.Command("setxkbmap", "-option", "grab:break_actions").Run()
	if !_useWayland {
		exec.Command("xdotool", "key", "XF86Ungrab").Run()
	}
}

// restoreXkbOption restores the previous keyboard options if non-empty.
func (m *Manager) restoreXkbOption(orig string) {
	if orig != "" {
		exec.Command("setxkbmap", "-option", orig).Run()
	}
}

// showLockFront calls LockFront1.Show via in-process DBus.
func (m *Manager) showLockFront() {
	lockObj := m.sessionSigLoop.Conn().Object(
		"org.deepin.dde.LockFront1",
		"/org/deepin/dde/LockFront1")
	err := lockObj.Call("org.deepin.dde.LockFront1.Show", 0).Err
	if err != nil {
		logger.Warning("lockScreen: failed to show lock front:", err)
	}
}
