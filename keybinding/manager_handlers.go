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
	"fmt"
	"os"
	"time"

	dbus "github.com/godbus/dbus"
	sys_network "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.network"
	power "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.power"
	. "pkg.deepin.io/dde/daemon/keybinding/shortcuts"
	"pkg.deepin.io/dde/api/powersupply/battery"
)

const (
	padEnv                                 = "Deepin-tablet"
	SCREENSHOT_CHORD_DEBOUNCE_DELAY_MILLIS = 300 * time.Millisecond

	POWER_RELEASE_SHORT = 0 // 电源键短按松开
	POWER_PRESS         = 1 // 电源键按下
	POWER_RELEASE_LONG  = 2 // 电源键长按松开

	cmdScreenShot = "dbus-send --print-reply --dest=com.deepin.Screenshot /com/deepin/Screenshot com.deepin.Screenshot.StartScreenshot"
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
				logger.Warning("execCmd error:", err)
			}
		}()
	}

	m.handlers[ActionTypeShowNumLockOSD] = func(ev *KeyEvent) {
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

	m.handlers[ActionTypeShowCapsLockOSD] = func(ev *KeyEvent) {
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

	m.handlers[ActionTypeAudioCtrl] = m.buildHandlerFromController(m.audioController)
	m.handlers[ActionTypeMediaPlayerCtrl] = m.buildHandlerFromController(m.mediaPlayerController)
	m.handlers[ActionTypeDisplayCtrl] = m.buildHandlerFromController(m.displayController)
	m.handlers[ActionTypeKbdLightCtrl] = m.buildHandlerFromController(m.kbdLightController)
	m.handlers[ActionTypeTouchpadCtrl] = m.buildHandlerFromController(m.touchPadController)
	m.handlers[ActionTypeToggleWireless] = func(ev *KeyEvent) {
		if m.gsMediaKey.GetBoolean(gsKeyUpperLayerWLAN) {
			sysBus, err := dbus.SystemBus()
			if err != nil {
				logger.Warning(err)
				return
			}
			sysNetwork := sys_network.NewNetwork(sysBus)
			enabled, err := sysNetwork.ToggleWirelessEnabled(0)
			if err != nil {
				logger.Warning("failed to toggle wireless enabled:", err)
				return
			}
			if enabled {
				showOSD("WLANOn")
			} else {
				showOSD("WLANOff")
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
	}

	m.handlers[ActionTypeSystemShutdown] = func(ev *KeyEvent) { // 电源键按下的handler
		// 平板环境下，由于电源键有长按，短按两种操作，此时对电源键的处理不走这个流程
		if os.Getenv("XDG_CURRENT_DESKTOP") == padEnv {
			return
		}

		var powerPressAction int32

		systemBus, _ := dbus.SystemBus()
		systemPower := power.NewPower(systemBus)
		var onBattery bool
		if os.Getenv("XDG_CURRENT_DESKTOP") == padEnv {
			batteryStatus, err := systemPower.BatteryStatus().Get(0)
			if err != nil {
				logger.Error(err)
			}
			onBattery = batteryStatus != uint32(battery.StatusCharging) && batteryStatus != uint32(battery.StatusFullCharging)
		} else {
			isOnBattery, err := systemPower.OnBattery().Get(0)
			if err != nil {
				logger.Error(err)
			}
			onBattery = isOnBattery
		}

		screenBlackLock := m.gsPower.GetBoolean("screen-black-lock")
		sleepLock := m.gsPower.GetBoolean("sleep-lock") // NOTE : 实际上是待机，不是休眠

		if onBattery {
			powerPressAction = m.gsPower.GetEnum("battery-press-power-button")
		} else {
			powerPressAction = m.gsPower.GetEnum("line-power-press-power-button")
		}
		switch powerPressAction {
		case powerActionShutdown:
			m.systemShutdown()
		case powerActionSuspend:
			if sleepLock {
				systemLock()
			}
			m.systemSuspend()
		case powerActionHibernate:
			m.systemHibernate()
		case powerActionTurnOffScreen:
			if screenBlackLock {
				systemLock()
			}
			m.systemTurnOffScreen()
		case powerActionShowUI:
			cmd := "dde-shutdown -s"
			go func() {
				locked, err := m.sessionManager.Locked().Get(0)
				if err != nil {
					logger.Warning("sessionManager get locked error:", err)
				}
				if locked {
					return
				}
				err = m.execCmd(cmd, false)
				if err != nil {
					logger.Warning("execCmd error:", err)
				}
			}()
		}
	}

	m.handlers[ActionTypeSystemSuspend] = func(ev *KeyEvent) {
		m.systemSuspend()
	}

	m.handlers[ActionTypeSystemLogOff] = func(ev *KeyEvent) {
		m.systemLogout()
	}

	m.handlers[ActionTypeSystemAway] = func(ev *KeyEvent) {
		m.systemAway()
	}

	// handle Switch Kbd Layout
	m.handlers[ActionTypeSwitchKbdLayout] = func(ev *KeyEvent) {
		logger.Debug("Switch Kbd Layout state", m.switchKbdLayoutState)
		flags := m.ShortcutSwitchLayout.Get()
		action := ev.Shortcut.GetAction()
		arg, ok := action.Arg.(uint32)
		if !ok {
			logger.Warning(ErrTypeAssertionFail)
			return
		}

		if arg&flags == 0 {
			return
		}

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
	}

	m.handlers[ActionTypeShowControlCenter] = func(ev *KeyEvent) {
		err := m.execCmd("dbus-send --session --dest=com.deepin.dde.ControlCenter  --print-reply /com/deepin/dde/ControlCenter com.deepin.dde.ControlCenter.Show",
			false)
		if err != nil {
			logger.Warning("failed to show control center:", err)
		}
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

func (m *Manager) handlePowerActionCode(actionCode int32) {
	logger.Info("handlePowerActionCode", actionCode)
	if actionCode == POWER_RELEASE_SHORT {
		// 长按电源结束时发出的POWER_RELEASE_SHORT事件.过滤掉
		if m.interceptPowerReleaseEvent {
			return
		}

		if !m.powerKeyTriggered {
			m.powerKeyTriggered = true
			m.powerKeyConsumedByScreenshotChord = false
			m.powerKeyTime = time.Now()
			m.interceptScreenshotChord()

			time.AfterFunc(350*time.Millisecond, func() {
				if !m.powerKeyConsumedByScreenshotChord {
					m.handleWakeUpScreen(m.gsPower.GetBoolean("wakeupscreen"))
					m.gsPower.SetBoolean("wakeupscreen", !m.gsPower.GetBoolean("wakeupscreen"))
				}
				m.resetScreenShotComboFlags()
			})
		}
	} else if actionCode == POWER_RELEASE_LONG {
		// 长按电源时, 内核上传的事件流程为:1 -> 2 -> 0
		// 短按电源时, 内核上传的事件流程为:1 -> 0
		// 为了防止长按结束时会处理POWER_RELEASE_SHORT事件, 需要将interceptPowerReleaseEvent置为true来过滤掉此次的POWER_RELEASE_SHORT事件
		m.interceptPowerReleaseEvent = true
		cmd := "due-shell -s"
		go func() {
			// TODO: 需要判断当前是否已经在锁屏界面
			err := m.execCmd(cmd, false)
			if err != nil {
				logger.Warning("execCmd error:", err)
			}
		}()
	} else if actionCode == POWER_PRESS {
		// 每次接收到电源press事件时,都重新初始化一次interceptPowerReleaseEvent
		m.interceptPowerReleaseEvent = false
	} else {
		logger.Warning("bad power action code")
	}
}

func (m *Manager) handleTouchInput() {
	m.gsPower.SetBoolean("wakeupscreen", false)
}

func (m *Manager) interceptScreenshotChord() {
	// TODO： 目前volumeUpKeyTriggered的值一直是false。之后可能会用到，先保留。
	if m.volumeDownKeyTriggered && m.powerKeyTriggered && !m.volumeUpKeyTriggered {
		now := time.Now()
		// 第一个键按下300ms内，另一个键按下，被认为是同时按下，触发截屏操作
		if now.Sub(m.volumeDownKeyTime) <= SCREENSHOT_CHORD_DEBOUNCE_DELAY_MILLIS && now.Sub(m.powerKeyTime) <= SCREENSHOT_CHORD_DEBOUNCE_DELAY_MILLIS {
			m.volumeDownKeyConsumedByScreenshotChord = true
			m.powerKeyConsumedByScreenshotChord = true
			err := m.execCmd(cmdScreenShot, false)
			if err != nil {
				logger.Warning("failed to exec screenShot:", err)
			}
		}
	}
}

type Controller interface {
	ExecCmd(cmd ActionCmd) error
	Name() string
}

func (m *Manager) buildHandlerFromController(c Controller) KeyEventFunc {
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

		// 响应音量-键时, 需要判断是否是截屏组合键逻辑
		if name == "Audio" && AudioSinkVolumeDown == ActionCmd(cmd) {
			if !m.volumeDownKeyTriggered {
				m.volumeDownKeyTriggered = true
				m.volumeDownKeyConsumedByScreenshotChord = false
				m.volumeDownKeyTime = time.Now()
				m.interceptScreenshotChord()

				time.AfterFunc(350*time.Millisecond, func() {
					if !m.volumeDownKeyConsumedByScreenshotChord {
						if err := c.ExecCmd(cmd); err != nil {
							logger.Warning(name, "Controller exec cmd err:", err)
						}
					}
					m.resetScreenShotComboFlags()
				})
			}
		} else {
			if err := c.ExecCmd(cmd); err != nil {
				logger.Warning(name, "Controller exec cmd err:", err)
			}
		}
	}
}

// 复位跟["电源"+"音量-"]截屏组合键相关的标志变量
func (m *Manager) resetScreenShotComboFlags() {
	m.volumeDownKeyTriggered = false
	m.volumeUpKeyTriggered = false
	m.powerKeyTriggered = false
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
