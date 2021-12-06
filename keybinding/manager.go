/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
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

package keybinding

import (
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	dbus "github.com/godbus/dbus"
	backlight "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.helper.backlight"
	inputdevices "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.inputdevices"
	keyevent "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.keyevent"
	lockfront "github.com/linuxdeepin/go-dbus-factory/com.deepin.dde.lockfront"
	shutdownfront "github.com/linuxdeepin/go-dbus-factory/com.deepin.dde.shutdownfront"
	sessionmanager "github.com/linuxdeepin/go-dbus-factory/com.deepin.sessionmanager"
	power "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.power"
	wm "github.com/linuxdeepin/go-dbus-factory/com.deepin.wm"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/keysyms"
	"pkg.deepin.io/dde/daemon/keybinding/shortcuts"
	"pkg.deepin.io/dde/daemon/session/common"
	gio "pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/gsprop"
	"pkg.deepin.io/lib/dbusutil/proxy"
	"pkg.deepin.io/lib/gsettings"
	"pkg.deepin.io/lib/xdg/basedir"
)

//go:generate dbusutil-gen em -type Manager

const (
	// shortcut signals:
	shortcutSignalChanged = "Changed"
	shortcutSignalAdded   = "Added"
	shortcutSignalDeleted = "Deleted"

	gsSchemaKeyboard          = "com.deepin.dde.keyboard"
	gsKeyNumLockState         = "numlock-state"
	gsKeySaveNumLockState     = "save-numlock-state"
	gsKeyShortcutSwitchLayout = "shortcut-switch-layout"
	gsKeyShowCapsLockOSD      = "capslock-toggle"
	gsKeyUpperLayerWLAN       = "upper-layer-wlan"
	gsKeyNormalUser           = "normal-user"
	gsKeyRootUser             = "root-user"

	gsSchemaSystem         = "com.deepin.dde.keybinding.system"
	gsSchemaSystemPlatform = "com.deepin.dde.keybinding.system.platform"
	gsSchemaSystemEnable   = "com.deepin.dde.keybinding.system.enable"
	gsSchemaMediaKey       = "com.deepin.dde.keybinding.mediakey"
	gsSchemaRuleWhiteList  = "com.deepin.dde.keybinding.rule.whitelist"
	gsSchemaGnomeWM        = "com.deepin.wrap.gnome.desktop.wm.keybindings"
	gsSchemaSessionPower   = "com.deepin.dde.power"

	customConfigFile = "deepin/dde-daemon/keybinding/custom.ini"
)

const ( // power按键事件的响应
	powerActionShutdown int32 = iota
	powerActionSuspend
	powerActionHibernate
	powerActionTurnOffScreen
	powerActionShowUI
)

type Manager struct {
	service *dbusutil.Service
	// properties
	NumLockState         gsprop.Enum
	ShortcutSwitchLayout gsprop.Uint `prop:"access:rw"`

	conn       *x.Conn
	keySymbols *keysyms.KeySymbols

	gsKeyboard       *gio.Settings
	gsSystem         *gio.Settings
	gsSystemPlatform *gio.Settings
	gsSystemEnable   *gio.Settings
	gsMediaKey       *gio.Settings
	gsGnomeWM        *gio.Settings
	gsPower          *gio.Settings
	gsRuleWhiteList  *gio.Settings

	enableListenGSettings bool

	customShortcutManager *shortcuts.CustomShortcutManager

	lockFront     lockfront.LockFront
	shutdownFront shutdownfront.ShutdownFront

	sessionSigLoop            *dbusutil.SignalLoop
	systemSigLoop             *dbusutil.SignalLoop
	startManager              sessionmanager.StartManager
	sessionManager            sessionmanager.SessionManager
	backlightHelper           backlight.Backlight
	keyboard                  inputdevices.Keyboard
	keyboardLayout            string
	wm                        wm.Wm
	keyEvent                  keyevent.KeyEvent
	specialKeycodeBindingList map[SpecialKeycodeMapKey]func()

	// controllers
	audioController       *AudioController
	mediaPlayerController *MediaPlayerController
	displayController     *DisplayController
	kbdLightController    *KbdLightController
	touchPadController    *TouchPadController

	shortcutManager *shortcuts.ShortcutManager
	// shortcut action handlers
	handlers             []shortcuts.KeyEventFunc
	lastKeyEventTime     time.Time
	lastExecCmdTime      time.Time
	lastMethodCalledTime time.Time
	grabScreenKeystroke  *shortcuts.Keystroke

	// for switch kbd layout
	switchKbdLayoutState SKLState
	sklWaitQuit          chan int

	//nolint
	signals *struct {
		Added, Deleted, Changed struct {
			id  string
			typ int32
		}

		KeyEvent struct {
			pressed   bool
			keystroke string
		}
	}

	// for customized product
	hasWhitelist bool
	whitelist    gsprop.Strv `prop:"access:rw"`
}

// SKLState Switch keyboard Layout state
type SKLState uint

const (
	SKLStateNone SKLState = iota
	SKLStateWait
	SKLStateOSDShown
)

func newManager(service *dbusutil.Service) (*Manager, error) {
	conn, err := x.NewConn()
	if err != nil {
		return nil, err
	}

	sessionBus := service.Conn()
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	var m = Manager{
		service:               service,
		enableListenGSettings: true,
		conn:                  conn,
		keySymbols:            keysyms.NewKeySymbols(conn),
	}

	m.sessionSigLoop = dbusutil.NewSignalLoop(sessionBus, 10)
	m.systemSigLoop = dbusutil.NewSignalLoop(sysBus, 10)

	m.gsKeyboard = gio.NewSettings(gsSchemaKeyboard)
	m.NumLockState.Bind(m.gsKeyboard, gsKeyNumLockState)
	m.ShortcutSwitchLayout.Bind(m.gsKeyboard, gsKeyShortcutSwitchLayout)
	m.sessionSigLoop.Start()
	m.systemSigLoop.Start()

	m.initNumLockState(sysBus)

	// 白名单
	if gio.SettingsSchemaSourceGetDefault().Lookup(gsSchemaRuleWhiteList, true) != nil {
		m.gsRuleWhiteList = gio.NewSettings(gsSchemaRuleWhiteList)
		if common.IsNormalUser() && m.gsRuleWhiteList.GetSchema().HasKey(gsKeyNormalUser) {
			m.whitelist.Bind(m.gsRuleWhiteList, gsKeyNormalUser)
			m.hasWhitelist = true
			logger.Debug("whitelist for normal user", m.whitelist.Get())
		} else if m.gsRuleWhiteList.GetSchema().HasKey(gsKeyRootUser) {
			m.whitelist.Bind(m.gsRuleWhiteList, gsKeyRootUser)
			m.hasWhitelist = true
			logger.Debug("whitelist for root user", m.whitelist.Get())
		}
	}

	// init settings
	m.gsSystem = gio.NewSettings(gsSchemaSystem)
	m.gsSystemPlatform = gio.NewSettings(gsSchemaSystemPlatform)
	m.gsSystemEnable = gio.NewSettings(gsSchemaSystemEnable)
	m.gsMediaKey = gio.NewSettings(gsSchemaMediaKey)
	m.gsPower = gio.NewSettings(gsSchemaSessionPower)
	m.wm = wm.NewWm(sessionBus)
	m.keyEvent = keyevent.NewKeyEvent(sysBus)

	m.shortcutManager = shortcuts.NewShortcutManager(m.conn, m.keySymbols, m.handleKeyEvent)
	m.shortcutManager.AddSystem(m.gsSystem, m.gsSystemPlatform, m.gsSystemEnable, m.wm)
	m.shortcutManager.AddMedia(m.gsMediaKey)

	// when session is locked, we need handle some keyboard function event
	m.lockFront = lockfront.NewLockFront(sessionBus)
	m.lockFront.InitSignalExt(m.sessionSigLoop, true)
	_, err = m.lockFront.ConnectChangKey(func(changKey string) {
		m.handleKeyEventFromLockFront(changKey)
	})
	if err != nil {
		logger.Warning("connect ChangKey signal failed:", err)
	}

	m.shutdownFront = shutdownfront.NewShutdownFront(sessionBus)
	m.shutdownFront.InitSignalExt(m.sessionSigLoop, true)
	_, err = m.shutdownFront.ConnectChangKey(func(changKey string) {
		m.handleKeyEventFromShutdownFront(changKey)
	})
	if err != nil {
		logger.Warning("connect ChangKey signal failed:", err)
	}

	if shouldUseDDEKwin() {
		logger.Debug("Use DDE KWin")
		m.shortcutManager.AddKWin(m.wm)
	} else {
		logger.Debug("Use gnome WM")
		m.gsGnomeWM = gio.NewSettings(gsSchemaGnomeWM)
		m.shortcutManager.AddWM(m.gsGnomeWM)
	}

	customConfigFilePath := filepath.Join(basedir.GetUserConfigDir(), customConfigFile)
	m.customShortcutManager = shortcuts.NewCustomShortcutManager(customConfigFilePath)
	m.shortcutManager.AddCustom(m.customShortcutManager)

	m.backlightHelper = backlight.NewBacklight(sysBus)
	m.audioController = NewAudioController(sessionBus, m.backlightHelper)
	m.mediaPlayerController = NewMediaPlayerController(m.systemSigLoop, sessionBus)

	m.startManager = sessionmanager.NewStartManager(sessionBus)
	m.sessionManager = sessionmanager.NewSessionManager(sessionBus)
	m.keyboard = inputdevices.NewKeyboard(sessionBus)
	m.keyboard.InitSignalExt(m.sessionSigLoop, true)
	err = m.keyboard.CurrentLayout().ConnectChanged(func(hasValue bool, layout string) {
		if !hasValue {
			return
		}
		if m.keyboardLayout != layout {
			m.keyboardLayout = layout
			logger.Debug("keyboard layout changed:", layout)
			m.shortcutManager.NotifyLayoutChanged()
		}
	})
	if err != nil {
		logger.Warning("connect CurrentLayout property changed failed:", err)
	}

	m.displayController = NewDisplayController(m.backlightHelper, sessionBus)
	m.kbdLightController = NewKbdLightController(m.backlightHelper)
	m.touchPadController = NewTouchPadController(sessionBus)

	m.initSpecialKeycodeMap()
	m.keyEvent.InitSignalExt(m.systemSigLoop, true)
	_, err = m.keyEvent.ConnectKeyEvent(m.handleSpecialKeycode)
	if err != nil {
		logger.Warning(err)
	}

	return &m, nil
}

// 初始化 NumLock 数字锁定键状态
func (m *Manager) initNumLockState(sysBus *dbus.Conn) {
	// 从 gsettings 读取相关设置
	nlState := NumLockState(m.NumLockState.Get())
	saveStateEnabled := m.gsKeyboard.GetBoolean(gsKeySaveNumLockState)
	if nlState == NumLockUnknown {
		// 判断是否是笔记本, 只根据电池状态，有电池则是笔记本。
		isLaptop := false
		sysPower := power.NewPower(sysBus)
		hasBattery, err := sysPower.HasBattery().Get(0)
		if err != nil {
			logger.Warning("failed to get sysPower HasBattery property:", err)
		} else if hasBattery {
			isLaptop = true
		}

		state := NumLockUnknown
		logger.Debug("isLaptop:", isLaptop)
		if isLaptop {
			// 笔记本，默认关闭。
			state = NumLockOff
		} else {
			// 台式机等，默认开启。
			state = NumLockOn
		}

		if saveStateEnabled {
			// 保存新状态到 gsettings
			m.NumLockState.Set(int32(state))
		}
		err = setNumLockState(m.conn, m.keySymbols, state)
		if err != nil {
			logger.Warning("setNumLockState failed:", err)
		}
	} else if saveStateEnabled {
		err := setNumLockState(m.conn, m.keySymbols, nlState)
		if err != nil {
			logger.Warning("setNumLockState failed:", err)
		}
	}
}

func (m *Manager) handleKeyEventFromLockFront(changKey string) {
	logger.Debugf("Receive LockFront ChangKey Event %s", changKey)
	action := shortcuts.GetAction(changKey)

	// numlock/capslock
	if action.Type == shortcuts.ActionTypeShowNumLockOSD ||
		action.Type == shortcuts.ActionTypeShowCapsLockOSD ||
		action.Type == shortcuts.ActionTypeSystemShutdown {
		if handler := m.handlers[int(action.Type)]; handler != nil {
			handler(nil)
		} else {
			logger.Warning("handler is nil")
		}
	} else {
		cmd, ok := action.Arg.(shortcuts.ActionCmd)
		if !ok {
			logger.Warning(errTypeAssertionFail)
		} else {
			if action.Type == shortcuts.ActionTypeAudioCtrl {
				// audio-mute/audio-lower-volume/audio-raise-volume
				if m.audioController != nil {
					if err := m.audioController.ExecCmd(cmd); err != nil {
						logger.Warning(m.audioController.Name(), "Controller exec cmd err:", err)
					}
				}
			} else if action.Type == shortcuts.ActionTypeDisplayCtrl {
				// mon-brightness-up/mon-brightness-down
				if m.displayController != nil {
					if err := m.displayController.ExecCmd(cmd); err != nil {
						logger.Warning(m.displayController.Name(), "Controller exec cmd err:", err)
					}
				}
			} else if action.Type == shortcuts.ActionTypeTouchpadCtrl {
				// touchpad-toggle/touchpad-on/touchpad-off
				if m.touchPadController != nil {
					if err := m.touchPadController.ExecCmd(cmd); err != nil {
						logger.Warning(m.touchPadController.Name(), "Controller exec cmd err:", err)
					}
				}
			}
		}
	}
}

func (m *Manager) handleKeyEventFromShutdownFront(changKey string) {
	logger.Debugf("handleKeyEvent %s from ShutdownFront", changKey)
	action := shortcuts.GetAction(changKey)
	if action.Type == shortcuts.ActionTypeSystemShutdown {
		if handler := m.handlers[int(action.Type)]; handler != nil {
			handler(nil)
		} else {
			logger.Warning("handler is nil")
		}
	}
}

func (m *Manager) destroy() {
	err := m.service.StopExport(m)
	if err != nil {
		logger.Warning("stop export failed:", err)
	}

	if m.shortcutManager != nil {
		m.shortcutManager.Destroy()
		m.shortcutManager = nil
	}

	// destroy settings
	if m.gsSystem != nil {
		m.gsSystem.Unref()
		m.gsSystem = nil
	}

	if m.gsMediaKey != nil {
		m.gsMediaKey.Unref()
		m.gsMediaKey = nil
	}

	if m.gsGnomeWM != nil {
		m.gsGnomeWM.Unref()
		m.gsGnomeWM = nil
	}

	if m.gsRuleWhiteList != nil {
		m.gsRuleWhiteList.Unref()
		m.gsRuleWhiteList = nil
	}

	if m.audioController != nil {
		m.audioController.Destroy()
		m.audioController = nil
	}

	if m.mediaPlayerController != nil {
		m.mediaPlayerController.Destroy()
		m.mediaPlayerController = nil
	}

	if m.keyboard != nil {
		m.keyboard.RemoveHandler(proxy.RemoveAllHandlers)
		m.keyboard = nil
	}

	if m.keyEvent != nil {
		m.keyEvent.RemoveHandler(proxy.RemoveAllHandlers)
		m.keyEvent = nil
	}

	if m.sessionSigLoop != nil {
		m.sessionSigLoop.Stop()
		m.sessionSigLoop = nil
	}

	if m.systemSigLoop != nil {
		m.systemSigLoop.Stop()
		m.systemSigLoop = nil
	}

	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
}

func (m *Manager) handleKeyEvent(ev *shortcuts.KeyEvent) {
	const minKeyEventInterval = 200 * time.Millisecond
	now := time.Now()
	duration := now.Sub(m.lastKeyEventTime)
	logger.Debug("duration:", duration)
	if 0 < duration && duration < minKeyEventInterval {
		logger.Debug("handleKeyEvent ignore key event")
		return
	}
	m.lastKeyEventTime = now

	logger.Debugf("handleKeyEvent ev: %#v", ev)
	action := ev.Shortcut.GetAction()
	shortcutId := ev.Shortcut.GetId()
	logger.Debugf("shortcut id: %s, type: %v, action: %#v",
		shortcutId, ev.Shortcut.GetType(), action)
	if action == nil {
		logger.Warning("action is nil")
		return
	}

	if m.hasWhitelist {
		found := false
		for _, id := range m.whitelist.Get() {
			if shortcutId == id {
				found = true
				break
			}
		}
		if !found {
			logger.Debugf("screening key event %s", shortcutId)
			return
		}
	}

	if handler := m.handlers[int(action.Type)]; handler != nil {
		handler(ev)
	} else {
		logger.Warning("handler is nil")
	}
}

func (m *Manager) emitShortcutSignal(signalName string, shortcut shortcuts.Shortcut) {
	logger.Debug("emit DBus signal", signalName, shortcut.GetId(), shortcut.GetType())
	err := m.service.Emit(m, signalName, shortcut.GetId(), shortcut.GetType())
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) enableListenGSettingsChanged(val bool) {
	m.enableListenGSettings = val
}

func (m *Manager) listenGSettingsChanged(schema string, settings *gio.Settings, type0 int32) {
	gsettings.ConnectChanged(schema, "*", func(key string) {
		if !m.enableListenGSettings {
			return
		}

		shortcut := m.shortcutManager.GetByIdType(key, type0)
		if shortcut == nil {
			return
		}

		keystrokes := settings.GetStrv(key)
		m.shortcutManager.ModifyShortcutKeystrokes(shortcut, shortcuts.ParseKeystrokes(keystrokes))
		m.emitShortcutSignal(shortcutSignalChanged, shortcut)
	})
}

func (m *Manager) listenSystemEnableChanged() {
	gsettings.ConnectChanged(gsSchemaSystemEnable, "*", func(key string) {
		if !m.enableListenGSettings {
			return
		}

		if m.shortcutManager.CheckSystem(m.gsSystemPlatform, m.gsSystemEnable, key) {
			m.shortcutManager.AddSystemById(m.gsSystem, key)
		} else {
			m.shortcutManager.DelSystemById(key)
		}
	})
}

func (m *Manager) listenSystemPlatformChanged() {
	gsettings.ConnectChanged(gsSchemaSystemPlatform, "*", func(key string) {
		if !m.enableListenGSettings {
			return
		}

		if m.shortcutManager.CheckSystem(m.gsSystemPlatform, m.gsSystemEnable, key) {
			m.shortcutManager.AddSystemById(m.gsSystem, key)
		} else {
			m.shortcutManager.DelSystemById(key)
		}
	})
}

func (m *Manager) execCmd(cmd string, viaStartdde bool) error {
	if cmd == "" {
		logger.Debug("cmd is empty")
		return nil
	}
	if strings.HasPrefix(cmd, "dbus-send ") || !viaStartdde {
		logger.Debug("run cmd:", cmd)
		return exec.Command("/bin/sh", "-c", cmd).Run()
	}

	logger.Debug("startdde run cmd:", cmd)
	return m.startManager.RunCommand(0, "/bin/sh", []string{"-c", cmd})
}

func (m *Manager) runDesktopFile(desktop string) error {
	return m.startManager.LaunchApp(0, desktop, 0, []string{})
}

func (m *Manager) eliminateKeystrokeConflict() {
	for _, ks := range m.shortcutManager.ConflictingKeystrokes {
		shortcut := ks.Shortcut
		logger.Infof("eliminate conflict shortcut: %s keystroke: %s",
			ks.Shortcut.GetUid(), ks)
		err := m.DeleteShortcutKeystroke(shortcut.GetId(), shortcut.GetType(), ks.String())
		if err != nil {
			logger.Warning("delete shortcut keystroke failed:", err)
		}
	}

	m.shortcutManager.ConflictingKeystrokes = nil
	m.shortcutManager.EliminateConflictDone = true
}
