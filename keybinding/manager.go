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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	backlight "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.helper.backlight"
	inputdevices "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.inputdevices"
	kwayland "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.kwayland"
	ses_network "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.network"
	lockfront "github.com/linuxdeepin/go-dbus-factory/com.deepin.dde.lockfront"
	sessionmanager "github.com/linuxdeepin/go-dbus-factory/com.deepin.sessionmanager"
	power "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.power"
	wm "github.com/linuxdeepin/go-dbus-factory/com.deepin.wm"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/keysyms"
	"pkg.deepin.io/dde/daemon/keybinding/shortcuts"
	gio "pkg.deepin.io/gir/gio-2.0"
	dbus "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/gsprop"
	"pkg.deepin.io/lib/dbusutil/proxy"
	"pkg.deepin.io/lib/gsettings"
	"pkg.deepin.io/lib/xdg/basedir"
)

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

	gsSchemaSystem       = "com.deepin.dde.keybinding.system"
	gsSchemaMediaKey     = "com.deepin.dde.keybinding.mediakey"
	gsSchemaGnomeWM      = "com.deepin.wrap.gnome.desktop.wm.keybindings"
	gsSchemaSessionPower = "com.deepin.dde.power"

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

	gsKeyboard *gio.Settings
	gsSystem   *gio.Settings
	gsMediaKey *gio.Settings
	gsGnomeWM  *gio.Settings
	gsPower    *gio.Settings

	enableListenGSettings   bool
	clickNum                uint32
	delayNetworkStateChange bool
	dpmsIsOff               bool
	shortcutIsPressed       bool
	shortcutCmd             string
	shortcutKey             string
	shortcutKeyCmd          string

	customShortcutManager *shortcuts.CustomShortcutManager

	lockFront        *lockfront.LockFront
	sessionSigLoop   *dbusutil.SignalLoop
	systemSigLoop    *dbusutil.SignalLoop
	startManager     *sessionmanager.StartManager
	sessionManager   *sessionmanager.SessionManager
	backlightHelper  *backlight.Backlight
	keyboard         *inputdevices.Keyboard
	keyboardLayout   string
	wm               *wm.Wm
	waylandOutputMgr *kwayland.OutputManagement
	login1Manager    *login1.Manager

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

	methods *struct {
		AddCustomShortcut         func() `in:"name,action,keystroke" out:"id,type"`
		AddShortcutKeystroke      func() `in:"id,type,keystroke"`
		ClearShortcutKeystrokes   func() `in:"id,type"`
		DeleteCustomShortcut      func() `in:"id"`
		DeleteShortcutKeystroke   func() `in:"id,type,keystroke"`
		GetShortcut               func() `in:"id,type" out:"shortcut"`
		ListAllShortcuts          func() `out:"shortcuts"`
		ListShortcutsByType       func() `in:"type" out:"shortcuts"`
		SearchShortcuts           func() `in:"query" out:"shortcuts"`
		LookupConflictingShortcut func() `in:"keystroke" out:"shortcut"`
		ModifyCustomShortcut      func() `in:"id,name,cmd,keystroke"`
		SetNumLockState           func() `in:"state"`
		GetCapsLockState          func() `out:"state"`
		SetCapsLockState          func() `in:"state"`

		// deprecated
		Add            func() `in:"name,action,keystroke" out:"ret0,ret1"`
		Query          func() `in:"id,type" out:"shortcut"`
		List           func() `out:"shortcuts"`
		Delete         func() `in:"id,type"`
		Disable        func() `in:"id,type"`
		CheckAvaliable func() `in:"keystroke" out:"available,shortcut"`
		ModifiedAccel  func() `in:"id,type,keystroke,add" out:"ret0,ret1"`
	}
}

// SKLState Switch keyboard Layout state
type SKLState uint

const (
	SKLStateNone SKLState = iota
	SKLStateWait
	SKLStateOSDShown
)

func (m *Manager) systemConn() *dbus.Conn {
	return m.systemSigLoop.Conn()
}

func (m *Manager) sessionConn() *dbus.Conn {
	return m.sessionSigLoop.Conn()
}

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

	m.waylandOutputMgr = kwayland.NewOutputManagement(sessionBus)
	m.sessionSigLoop = dbusutil.NewSignalLoop(sessionBus, 10)
	m.systemSigLoop = dbusutil.NewSignalLoop(sysBus, 10)
	m.login1Manager = login1.NewManager(sysBus)

	m.gsKeyboard = gio.NewSettings(gsSchemaKeyboard)
	m.NumLockState.Bind(m.gsKeyboard, gsKeyNumLockState)
	m.ShortcutSwitchLayout.Bind(m.gsKeyboard, gsKeyShortcutSwitchLayout)
	m.sessionSigLoop.Start()
	m.systemSigLoop.Start()

	if m.gsKeyboard.GetBoolean(gsKeySaveNumLockState) {
		nlState := NumLockState(m.NumLockState.Get())
		if nlState == NumLockUnknown {
			state, err := queryNumLockState(m.conn)
			if err != nil {
				logger.Warning("queryNumLockState failed:", err)
			} else {
				m.NumLockState.Set(int32(state))
			}
		} else {
			if len(os.Getenv("WAYLAND_DISPLAY")) != 0 {
				err := setNumLockWl(m.waylandOutputMgr, m.conn, nlState)
				if err != nil {
					logger.Warning("setNumLockWl failed:", err)
				}
			} else {
				err := setNumLockState(m.conn, m.keySymbols, nlState)
				if err != nil {
					logger.Warning("setNumLockState failed:", err)
				}
			}
		}
	}

	return &m, nil
}

func (m *Manager) init() {
	sessionBus := m.service.Conn()
	sysBus, _ := dbus.SystemBus()
	m.delayNetworkStateChange = true
	m.dpmsIsOff = false
	m.shortcutIsPressed = false

	// init settings
	m.gsSystem = gio.NewSettings(gsSchemaSystem)
	m.gsMediaKey = gio.NewSettings(gsSchemaMediaKey)
	m.gsPower = gio.NewSettings(gsSchemaSessionPower)
	m.wm = wm.NewWm(sessionBus)

	m.shortcutManager = shortcuts.NewShortcutManager(m.conn, m.keySymbols, m.handleKeyEvent)
	// when session is locked, we need handle some keyboard function event
	m.lockFront = lockfront.NewLockFront(sessionBus)
	m.lockFront.InitSignalExt(m.sessionSigLoop, true)
	m.lockFront.ConnectChangKey(func(changKey string) {
		m.handleKeyEventFromLockFront(changKey)
	})

	if shouldUseDDEKwin() {
		m.shortcutManager.AddSpecialToKwin(m.wm)
		m.shortcutManager.AddSystemToKwin(m.gsSystem, m.wm)
		m.shortcutManager.AddMediaToKwin(m.gsMediaKey, m.wm)
		m.shortcutManager.AddKWin(m.wm)
	} else {
		m.shortcutManager.AddSpecial()
		m.shortcutManager.AddSystem(m.gsSystem, m.wm)
		m.shortcutManager.AddMedia(m.gsMediaKey)
		m.gsGnomeWM = gio.NewSettings(gsSchemaGnomeWM)
		m.shortcutManager.AddWM(m.gsGnomeWM)
	}
	// init custom shortcuts
	customConfigFilePath := filepath.Join(basedir.GetUserConfigDir(), customConfigFile)
	m.customShortcutManager = shortcuts.NewCustomShortcutManager(customConfigFilePath)
	m.shortcutManager.AddCustom(m.customShortcutManager)

	// init controllers
	m.backlightHelper = backlight.NewBacklight(sysBus)
	m.audioController = NewAudioController(sessionBus, m.backlightHelper)
	m.mediaPlayerController = NewMediaPlayerController(m.systemSigLoop, sessionBus)
	m.displayController = NewDisplayController(m.backlightHelper, sessionBus)
	m.kbdLightController = NewKbdLightController(m.backlightHelper)
	m.touchPadController = NewTouchPadController(sessionBus)

	m.startManager = sessionmanager.NewStartManager(sessionBus)
	m.sessionManager = sessionmanager.NewSessionManager(sessionBus)
	m.keyboard = inputdevices.NewKeyboard(sessionBus)
	m.keyboard.InitSignalExt(m.sessionSigLoop, true)
	m.keyboard.CurrentLayout().ConnectChanged(func(hasValue bool, layout string) {
		if !hasValue {
			return
		}
		if m.keyboardLayout != layout {
			m.keyboardLayout = layout
			logger.Debug("keyboard layout changed:", layout)
			m.shortcutManager.NotifyLayoutChanged()
		}
	})
	m.initHandlers()
	m.clickNum = 0
	go m.ListenGlobalAccelRelease(sessionBus)
	go m.ListenGlobalAccel(sessionBus)
	go m.ListenKeyboardEvent(sysBus)
	go m.DealWithShortcutEvent()
}

var kwinSysActionCmdMap = map[string]string{
	"Launcher":              "launcher",               //Super_L Super_R
	"Terminal":              "terminal",               //<Control><Alt>T
	"Terminal Quake Window": "terminal-quake",         //
	"Lock screen":           "lock-screen",            //super+l
	"Shutdown interface":    "logout",                 //ctrl+alt+del
	"File manager":          "file-manager",           //super+e
	"Screenshot":            "screenshot",             //ctrl+alt+a
	"Full screenshot":       "screenshot-fullscreen",  //print
	"Window screenshot":     "screenshot-window",      //alt+print
	"Delay screenshot":      "screenshot-delayed",     //ctrl+print
	"Disable Touchpad":      "disable-touchpad",       //
	"Switch window effects": "wm-switcher",            //alt+tab
	"turn-off-screen":       "Fast Screen Off",        //<Shift><Super>L
	"Deepin Picker":         "color-picker",           //ctrl+alt+v
	"System Monitor":        "system-monitor",         //ctrl+alt+escape
	"Screen Recorder":       "deepin-screen-recorder", // deepin-screen-recorder ctrl+alt+r
	"Desktop AI Assistant":  "ai-assistant",           // ai-assistant [<Super>Q]q
	"Text to Speech":        "text-to-speech",
	"Speech to Text":        "speech-to-text",
	"Clipboard":             "clipboard",
	"Translation":           "translation",
	"Show/Hide the dock":    "show-dock",

	// cmd
	"Calculator": "calculator", // XF86Calculator
	"Search":     "search",     // XF86Search
}

var waylandMediaIdMap = map[string]string{
	"Messenger":         "messenger",           // XF86Messenger
	"Save":              "save",                // XF86Save
	"New":               "new",                 // XF86New
	"WakeUp":            "wake-up",             // XF86WakeUp
	"audio-rewind":      "AudioRewind",         // XF86AudioRewind
	"VolumeMute":        "audio-mute",          // XF86AudioMute  "AudioMute":
	"MonBrightnessUp":   "mon-brightness-up",   // XF86MonBrightnessUp
	"WLAN":              "wlan",                // XF86WLAN
	"AudioMedia":        "audio-media",         // XF86AudioMedia
	"reply":             "Reply",               // XF86Reply
	"favorites":         "Favorites",           // XF86Favorites
	"AudioPlay":         "audio-play",          // XF86AudioPlay
	"AudioMicMute":      "audio-mic-mute",      // XF86AudioMicMute
	"AudioPause":        "audio-pause",         // XF86AudioPause
	"AudioStop":         "audio-stop",          // XF86AudioStop
	"PowerOff":          "power-off",           // XF86PowerOff
	"documents":         "Documents",           // XF86Documents
	"game":              "Game",                // XF86Game
	"AudioRecord":       "audio-record",        // XF86AudioRecord
	"Display":           "display",             // XF86Display
	"reload":            "Reload",              // XF86Reload
	"explorer":          "Explorer",            // XF86Explorer
	"calendar":          "Calendar",            // XF86Calendar
	"forward":           "Forward",             // XF86Forward
	"cut":               "Cut",                 // XF86Cut
	"MonBrightnessDown": "mon-brightness-down", // XF86MonBrightnessDown
	"Copy":              "copy",                // XF86Copy
	"Tools":             "tools",               // XF86Tools
	"VolumeUp":          "audio-raise-volume",  // XF86AudioRaiseVolume "AudioRaiseVolume":  "audio-raise-volume",
	"close":             "Close",               // XF86Close
	"WWW":               "www",                 // XF86WWW
	"HomePage":          "home-page",           // XF86HomePage
	"sleep":             "Sleep",               // XF86Sleep
	"VolumeDown":        "audio-lower-volume",  // XF86AudioLowerVolume  "AudioLowerVolume":  "audio-lower-volume",
	"AudioPrev":         "audio-prev",          // XF86AudioPrev
	"AudioNext":         "audio-next",          // XF86AudioNext
	"Paste":             "paste",               // XF86Paste
	"open":              "Open",                // XF86Open
	"send":              "Send",                // XF86Send
	"my-computer":       "MyComputer",          // XF86MyComputer
	"Mail":              "mail",                // XF86Mail
	"adjust-brightness": "BrightnessAdjust",    // XF86BrightnessAdjust
	"LogOff":            "log-off",             // XF86LogOff
	"pictures":          "Pictures",            // XF86Pictures
	"Terminal":          "terminal",            // XF86Terminal
	"video":             "Video",               // XF86Video
	"Music":             "music",               // XF86Music
	"app-left":          "ApplicationLeft",     // XF86ApplicationLeft
	"app-right":         "ApplicationRight",    // XF86ApplicationRight
	"meeting":           "Meeting",             // XF86Meeting
	"Switch monitors":   "switch-monitors",
	"Numlock":           "numlock",
	"Capslock":          "capslock",
	"Switch kbd layout": "switch-kbd-layout",
}

func (m *Manager) ListenGlobalAccel(sessionBus *dbus.Conn) error {
	err := sessionBus.Object("org.kde.kglobalaccel",
		"/component/kwin").AddMatchSignal("org.kde.kglobalaccel.Component", "globalShortcutPressed").Err
	if err != nil {
		logger.Warning(err)
		return err
	}

	m.sessionSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "org.kde.kglobalaccel.Component.globalShortcutPressed",
	}, func(sig *dbus.Signal) {
		if len(sig.Body) > 1 {
			m.shortcutKey = sig.Body[0].(string)
			m.shortcutKeyCmd = sig.Body[1].(string)
			ok := strings.Compare(string("kwin"), m.shortcutKey)
			if ok == 0 {
				logger.Debug("[test global key] get accel sig.Body[1]", sig.Body[1], m.shortcutIsPressed)
				m.shortcutCmd = shortcuts.GetSystemActionCmd(kwinSysActionCmdMap[m.shortcutKeyCmd])
				//+ 根据产品需求，目前只开发F1,F2,F5，F6快捷键的长按效果
				if m.shortcutKeyCmd == "MonBrightnessDown" || m.shortcutKeyCmd == "MonBrightnessUp" ||
				   m.shortcutKeyCmd == "VolumeDown" || m.shortcutKeyCmd == "VolumeUp" {
					m.shortcutIsPressed = true
				} else {
					m.shortcutIsPressed = false
					if m.shortcutCmd == "" {
						m.handleKeyEventByWayland(waylandMediaIdMap[m.shortcutKeyCmd])
					} else {
						m.execCmd(m.shortcutCmd, true)
					}
				}
			}
		}
	})

	//+ 监听锁屏信号
	err = sessionBus.Object("com.deepin.dde.lockFront",
		"/com/deepin/dde/lockFront").AddMatchSignal("com.deepin.dde.lockFront", "Visible").Err
	if err != nil {
		logger.Warning(err)
		return err
	}
	m.sessionSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "com.deepin.dde.lockFront.Visible",
	}, func(sig *dbus.Signal) {
		if len(sig.Body) > 0 {
			isLocked := sig.Body[0].(bool)
			if isLocked {
				m.shortcutIsPressed = false

				sessionBus, err := dbus.SessionBus()
				if err != nil {
					return
				}
				obj := sessionBus.Object("org.kde.kglobalaccel", "/kglobalaccel")
				err = obj.Call("org.kde.KGlobalAccel.blockGlobalShortcuts", 0, true).Err
				if err != nil {
					return
				} else {
					var stringList = [...]string{"PowerOff", "CapsLock", "MonBrightnessDown", "MonBrightnessUp",
					"VolumeMute", "VolumeDown", "VolumeUp", "AudioMicMute", "WLAN", "Tools", "Full screenshot",
					"PowerOff"}
					for _, str := range stringList {
						err = obj.Call("org.kde.KGlobalAccel.setActiveByUniqueName", 0, str, true).Err
						if err != nil {
							return
						}
						time.Sleep(10 * time.Millisecond)
					}
				}
			} else {
				obj := sessionBus.Object("org.kde.kglobalaccel", "/kglobalaccel")
				err = obj.Call("org.kde.KGlobalAccel.blockGlobalShortcuts", 0, false).Err
				if err != nil {
					return
				}
			}
		}
	})

	//+ 监控鼠标移动事件
	err = sessionBus.Object("com.deepin.daemon.KWayland",
		"/com/deepin/daemon/KWayland/Output").AddMatchSignal("com.deepin.daemon.KWayland.Output", "CursorMove").Err
	if err != nil {
		logger.Warning(err)
		return err
	}
	m.sessionSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "com.deepin.daemon.KWayland.Output.CursorMove",
	}, func(sig *dbus.Signal) {
		if len(sig.Body) > 1 {
			if m.dpmsIsOff {
				err := exec.Command("dde_wldpms", "-s", "On").Run()
				if err != nil {
					logger.Warningf("failed to exec dde_wldpms: %s", err)
				} else {
					m.dpmsIsOff = false
				}
			}
		}
	})
	return nil
}

func (m *Manager) ListenKeyboardEvent(systemBus *dbus.Conn) error {
	err := systemBus.Object("com.deepin.daemon.Gesture",
		"/com/deepin/daemon/Gesture").AddMatchSignal("com.deepin.daemon.Gesture", "KeyboardEvent").Err
	if err != nil {
		logger.Warning(err)
		return err
	}
	m.systemSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "com.deepin.daemon.Gesture.KeyboardEvent",
	}, func(sig *dbus.Signal) {
		if len(sig.Body) > 1 {
			key := sig.Body[0].(uint32)
			value := sig.Body[1].(uint32)
			//+ 短按电源键同时出发kwin快捷键逻辑和libinput逻辑有冲突，先屏蔽
			if m.dpmsIsOff && value == 1 && key != 116 {
				logger.Debug("Keyboard:", key, value)
				err := exec.Command("dde_wldpms", "-s", "On").Run()
				if err != nil {
					logger.Warningf("failed to exec dde_wldpms: %s", err)
				} else {
					m.dpmsIsOff = false
				}
			}
		}
	})
	return nil
}

func (m *Manager) ListenGlobalAccelRelease(sessionBus *dbus.Conn) error {
	err := sessionBus.Object("org.kde.kglobalaccel",
		"/component/kwin").AddMatchSignal("org.kde.kglobalaccel.Component", "globalShortcutReleased").Err
	if err != nil {
		logger.Warning(err)
		return err
	}

	m.sessionSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "org.kde.kglobalaccel.Component.globalShortcutReleased",
	}, func(sig *dbus.Signal) {
		if len(sig.Body) > 1 {
			m.shortcutIsPressed = false
		}
	})
	return nil
}

func (m *Manager) DealWithShortcutEvent() {
	for {
		if m.shortcutIsPressed {
			for {
				if m.shortcutCmd == "" {
					m.handleKeyEventByWayland(waylandMediaIdMap[m.shortcutKeyCmd])
				} else {
					m.execCmd(m.shortcutCmd, true)
				}
				if !m.shortcutIsPressed {
					break
				}

				time.Sleep(200 * time.Millisecond)

				if !m.shortcutIsPressed {
					break
				}
			}
		}
		runtime.Gosched()
	}
}

func (m *Manager) handleKeyEventFromLockFront(changKey string) {
	logger.Debugf("Receive LockFront ChangKey Event %s", changKey)
	action := shortcuts.GetAction(changKey)

	// numlock/capslock
	if action.Type == shortcuts.ActionTypeShowNumLockOSD ||
		action.Type == shortcuts.ActionTypeShowCapsLockOSD {
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

func (m *Manager) handleKeyEventByWayland(changKey string) {
	action := shortcuts.GetAction(changKey)
	// numlock/capslock
	if action.Type == shortcuts.ActionTypeSystemShutdown {
		var powerPressAction int32
		systemBus, _ := dbus.SystemBus()
		systemPower := power.NewPower(systemBus)
		onBattery, err := systemPower.OnBattery().Get(0)
		if err != nil {
			logger.Error(err)
		}
		if onBattery {
			powerPressAction = m.gsPower.GetEnum("battery-press-power-button")
		} else {
			powerPressAction = m.gsPower.GetEnum("line-power-press-power-button")
		}
		logger.Debug("powerPressAction:", powerPressAction)
		switch powerPressAction {
		case powerActionShutdown:
			m.systemShutdown()
		case powerActionSuspend:
			systemSuspend()
		case powerActionHibernate:
			m.systemHibernate()
		case powerActionTurnOffScreen:
			m.systemTurnOffScreen()
		case powerActionShowUI:
			cmd := "dde-shutdown"
			go func() {
				err := m.execCmd(cmd, false)
				if err != nil {
					logger.Warning("execCmd error:", err)
				}
			}()
		}
	} else if action.Type == shortcuts.ActionTypeShowControlCenter {
		err := m.execCmd("dbus-send --session --dest=com.deepin.dde.ControlCenter  --print-reply /com/deepin/dde/ControlCenter com.deepin.dde.ControlCenter.Show",
			false)
		if err != nil {
			logger.Warning("failed to show control center:", err)
		}

	} else if action.Type == shortcuts.ActionTypeToggleWireless {
		if m.gsMediaKey.GetBoolean(gsKeyUpperLayerWLAN) {
			sessionBus, err := dbus.SessionBus()
			if err != nil {
				logger.Warning(err)
				return
			}
			obj := ses_network.NewNetwork(sessionBus)
			connWifi, err := obj.Devices().Get(0)
			wifiPre := "\"wireless\":[{\"Path\":\"/org/freedesktop/NetworkManager/Devices/"
			i := strings.Index(connWifi, wifiPre)
			if i < 0 {
				return
			}

			lenwifi := "\"wireless\":[{\"Path\":\""
			devpath := connWifi[i+len(lenwifi) : i+len(wifiPre)+1]
			enabled := false
			if ret, err := obj.IsDeviceEnabled(0, dbus.ObjectPath(devpath)); ret {
				enabled = true
				if err != nil {
					logger.Warning(err)
					return
				}
			}

			if false == m.delayNetworkStateChange {
				return
			}
			m.delayNetworkStateChange = false
			time.Sleep(500 * time.Millisecond) //+ 与前端的延时对应
			m.delayNetworkStateChange = true

			if enabled {
				//add to avoid conflict with contorl-center
				time.Sleep(500 * time.Millisecond)
				obj.EnableDevice(0, dbus.ObjectPath(devpath), false)
				showOSD("WLANOff")
			} else {
				obj.EnableDevice(0, dbus.ObjectPath(devpath), true)
				showOSD("WLANOn")
			}
		} else {
			state, err := getRfkillWlanState()
			if err != nil {
				logger.Warning(err)
				return
			}
			if state == 0 {
				showOSD("WLANOff")
			} else {
				showOSD("WLANOn")
			}
		}

	} else if action.Type == shortcuts.ActionTypeShowNumLockOSD {
		var state NumLockState
		if len(os.Getenv("WAYLAND_DISPLAY")) != 0 {
			systemBus, err := dbus.SystemBus()
			if err != nil {
				return
			}
			systemdObj := systemBus.Object("com.deepin.system.Evdev", "/com/deepin/system/Evdev")
			var ret int32
			time.Sleep(200 * time.Millisecond) //+ 添加200ms延时，保证在dde-system-daemon中先获取状态；
			err = systemdObj.Call("com.deepin.system.Evdev.GetNumLockState", 0).Store(&ret)
			if err != nil {
				logger.Warning(err)
				return
			}

			if 0 == ret {
				state = NumLockOff
			} else {
				state = NumLockOn
			}
		} else {
			var err error
			state, err = queryNumLockState(m.conn)
			if err != nil {
				logger.Warning(err)
				return
			}
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
	} else if action.Type == shortcuts.ActionTypeShowCapsLockOSD {
		if !m.shouldShowCapsLockOSD() {
			return
		}

		var state CapsLockState
		if len(os.Getenv("WAYLAND_DISPLAY")) != 0 {
			systemBus, err := dbus.SystemBus()
			if err != nil {
				return
			}
			time.Sleep(200 * time.Millisecond) //+ 添加200ms延时，保证在dde-system-daemon中先获取状态；
			systemdObj := systemBus.Object("com.deepin.system.Evdev", "/com/deepin/system/Evdev")
			var ret int32
			err = systemdObj.Call("com.deepin.system.Evdev.GetCapsLockState", 0).Store(&ret)
			if err != nil {
				logger.Warning(err)
				return
			}

			if 0 == ret {
				state = CapsLockOff
			} else {
				state = CapsLockOn
			}
		} else {
			state, err := queryCapsLockState(m.conn)
			if err != nil {
				logger.Warning(err)
				return
			}
			logger.Debug("caps:", state)
		}

		switch state {
		case CapsLockOff:
			showOSD("CapsLockOff")
		case CapsLockOn:
			showOSD("CapsLockOn")
		}
	} else if action.Type == shortcuts.ActionTypeSwitchKbdLayout {
		switch m.switchKbdLayoutState {
		case SKLStateNone:
			m.switchKbdLayoutState = SKLStateWait
			go m.sklWait()

		case SKLStateWait:
			m.switchKbdLayoutState = SKLStateOSDShown
			m.terminateSKLWait()
			showOSD("SwitchLayout")

		case SKLStateOSDShown:
			showOSD("SwitchLayout")
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
			} else if action.Type == shortcuts.ActionTypeSystemShutdown {

			} else if action.Type == shortcuts.ActionTypeMediaPlayerCtrl {
				//增蓝牙耳机快捷键的处理
				if cmd == shortcuts.MediaPlayerPlay {
					m.clickNum = m.clickNum + 1
					if m.clickNum == 1 {
						time.AfterFunc(time.Millisecond*600, func() {
							m.playMeadiaByHeadphone()
						})
					}
				} else {
					if m.mediaPlayerController != nil {
						err := m.mediaPlayerController.ExecCmd(cmd)
						if err != nil {
							logger.Warning(m.mediaPlayerController.Name(), "Controller exec cmd err:", err)
						}
					}
				}

			}
		}
	}
}

func getMediaPlayAction(num uint32) shortcuts.ActionCmd {
	var cmd shortcuts.ActionCmd = shortcuts.MediaPlayerPlay
	if num == 2 {
		cmd = shortcuts.MediaPlayerNext
	} else if num == 3 {
		cmd = shortcuts.MediaPlayerPrevious
	} else {
		cmd = shortcuts.MediaPlayerPlay
	}
	return cmd
}

func (m *Manager) playMeadiaByHeadphone() {
	cmd := getMediaPlayAction(m.clickNum)
	m.clickNum = 0
	if m.mediaPlayerController != nil {
		err := m.mediaPlayerController.ExecCmd(cmd)
		if err != nil {
			logger.Warning(m.mediaPlayerController.Name(), "Controller exec cmd err:", err)
		}
	}
	return
}

func (m *Manager) destroy() {
	m.service.StopExport(m)

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
	logger.Debugf("shortcut id: %s, type: %v, action: %#v",
		ev.Shortcut.GetId(), ev.Shortcut.GetType(), action)
	if action == nil {
		logger.Warning("action is nil")
		return
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
		m.DeleteShortcutKeystroke(shortcut.GetId(), shortcut.GetType(), ks.String())
	}

	m.shortcutManager.ConflictingKeystrokes = nil
	m.shortcutManager.EliminateConflictDone = true
}
