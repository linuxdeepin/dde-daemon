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

package keybinding

import (
	"io/ioutil"
	"strings"

	"github.com/godbus/dbus"
	power "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.power"
)

// 按键码
// https://github.com/torvalds/linux/blob/master/include/uapi/linux/input-event-codes.h
// nolint
const (
	KEY_TOUCHPAD_TOGGLE = 0x212
	KEY_TOUCHPAD_ON     = 0x213
	KEY_TOUCHPAD_OFF    = 0x214
	KEY_POWER           = 116
	KEY_FN_ESC          = 0x1d1
	KEY_MICMUTE         = 248
	KEY_SETUP           = 141
	KEY_SCREENLOCK      = 152
	KEY_CYCLEWINDOWS    = 154
	KEY_MODE            = 0x175
	KEY_KBDILLUMTOGGLE  = 228
)

type SpecialKeycodeMapKey struct {
	keycode      uint32 // 按键码
	pressed      bool   // true:按下事件，false松开事件
	ctrlPressed  bool   // ctrl 是否处于按下状态
	shiftPressed bool   // shift 是否处于按下状态
	altPressed   bool   // alt 是否处于按下状态
	superPressed bool   // super 是否处于按下状态
}

// 按键修饰
const (
	MODIFY_NONE uint32 = 0
	MODIFY_CTRL uint32 = 1 << iota
	MODIFY_SHIFT
	MODIFY_ALT
	MODIFY_SUPER
)

// 通过按键码和状态创建map的key
func createSpecialKeycodeIndex(keycode uint32, pressed bool, modify uint32) SpecialKeycodeMapKey {
	return SpecialKeycodeMapKey{
		keycode,
		pressed,
		(modify & MODIFY_CTRL) != 0,
		(modify & MODIFY_SHIFT) != 0,
		(modify & MODIFY_ALT) != 0,
		(modify & MODIFY_SUPER) != 0,
	}
}

// 初始化按键到处理函数的map
func (m *Manager) initSpecialKeycodeMap() {
	m.specialKeycodeBindingList = make(map[SpecialKeycodeMapKey]func())

	// 触摸板开关键
	key := createSpecialKeycodeIndex(KEY_TOUCHPAD_TOGGLE, true, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handleTouchpadToggle

	// 触摸板开键
	key = createSpecialKeycodeIndex(KEY_TOUCHPAD_ON, true, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handleTouchpadOn

	// 触摸板关键
	key = createSpecialKeycodeIndex(KEY_TOUCHPAD_OFF, true, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handleTouchpadOff

	// 电源键，松开时触发
	key = createSpecialKeycodeIndex(KEY_POWER, false, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handlePower

	// FnLock
	key = createSpecialKeycodeIndex(KEY_FN_ESC, false, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handleFnLock

	// 开关麦克风
	key = createSpecialKeycodeIndex(KEY_MICMUTE, false, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handleMicMute

	// 打开控制中心
	key = createSpecialKeycodeIndex(KEY_SETUP, false, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handleOpenControlCenter

	// 锁屏
	key = createSpecialKeycodeIndex(KEY_SCREENLOCK, false, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handleScreenlock

	// 打开多任务视图
	key = createSpecialKeycodeIndex(KEY_CYCLEWINDOWS, false, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handleWorkspace

	// 切换性能模式
	key = createSpecialKeycodeIndex(KEY_MODE, false, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handleSwitchPowerMode

	// 切换键盘背光模式
	key = createSpecialKeycodeIndex(KEY_KBDILLUMTOGGLE, false, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handleKbdLight
}

// 处理函数的总入口
func (m *Manager) handleSpecialKeycode(keycode uint32,
	pressed bool,
	ctrlPressed bool,
	shiftPressed bool,
	altPressed bool,
	superPressed bool) {

	key := SpecialKeycodeMapKey{
		keycode,
		pressed,
		ctrlPressed,
		shiftPressed,
		altPressed,
		superPressed,
	}

	handler, ok := m.specialKeycodeBindingList[key]
	if ok {
		handler()
	}
}

// 开关FnLock
func (m *Manager) handleFnLock() {
	showOSD("FnToggle")
}

// 开关麦克风
func (m *Manager) handleMicMute() {
	source, err := m.audioController.getDefaultSource()
	if err != nil {
		logger.Warning(err)
		return
	}

	mute, err := source.Mute().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	mute = !mute
	err = source.SetMute(0, mute)
	if err != nil {
		logger.Warning(err)
		return
	}

	var osd string
	if mute {
		osd = "AudioMicMuteOn"
	} else {
		osd = "AudioMicMuteOff"
	}
	showOSD(osd)
}

// 打开控制中心
func (m *Manager) handleOpenControlCenter() {
	cmd := "dbus-send --session --dest=com.deepin.dde.ControlCenter --print-reply /com/deepin/dde/ControlCenter com.deepin.dde.ControlCenter.Show"
	m.execCmd(cmd, false)
}

// 锁屏
func (m *Manager) handleScreenlock() {
	m.lockFront.Show(0)
}

// 打开任务视图
func (m *Manager) handleWorkspace() {
	m.wm.ShowWorkspace(0)
}

// 切换触摸板状态
func (m *Manager) handleTouchpadToggle() {
	showOSD("TouchpadToggle")
}

func (m *Manager) handleTouchpadOn() {
	showOSD("TouchpadOn")
}

func (m *Manager) handleTouchpadOff() {
	showOSD("TouchpadOff")
}

// 切换性能模式
func (m *Manager) handleSwitchPowerMode() {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning("connect to system bus failed:", err)
		return
	}

	pwr := power.NewPower(systemBus)
	mode, err := pwr.Mode().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	isHighPerformanceSupported, err := pwr.IsHighPerformanceSupported().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	targetMode := ""
	//平衡 balance, 节能 powersave, 高性能 performance
	if isHighPerformanceSupported {
		if mode == "balance" {
			targetMode = "powersave"
		} else if mode == "powersave" {
			targetMode = "performance"
		} else if mode == "performance" {
			targetMode = "balance"
		}
	} else {
		if mode == "balance" {
			targetMode = "powersave"
		} else if mode == "powersave" {
			targetMode = "balance"
		}
	}

	if targetMode == "" {
		return
	}
	err = pwr.SetMode(0, targetMode)

	logger.Infof("[handleSwitchPowerMode] from %s to %s", mode, targetMode)
	if err != nil {
		logger.Warning(err)
	} else {
		showOSD(targetMode)
	}
}

// 电源键的处理
func (m *Manager) handlePower() {
	var powerPressAction int32

	systemBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning("connect to system bus failed:", err)
		return
	}
	systemPower := power.NewPower(systemBus)
	onBattery, err := systemPower.OnBattery().Get(0)
	if err != nil {
		logger.Warning(err)
	}

	screenBlackLock := m.gsPower.GetBoolean("screen-black-lock")

	if onBattery {
		powerPressAction = m.gsPower.GetEnum("battery-press-power-button")
	} else {
		powerPressAction = m.gsPower.GetEnum("line-power-press-power-button")
	}
	switch powerPressAction {
	case powerActionShutdown:
		m.systemShutdown()
	case powerActionSuspend:
		m.systemSuspendByFront()
	case powerActionHibernate:
		m.systemHibernateByFront()
	case powerActionTurnOffScreen:
		if screenBlackLock {
			systemLock()
		}
		m.systemTurnOffScreen()
	case powerActionShowUI:
		cmd := "dde-lock -t"
		go func() {
			locked, err := m.sessionManager.Locked().Get(0)
			if err != nil {
				logger.Warning("sessionManager get locked error:", err)
			}
			if !locked {
				err = m.execCmd(cmd, false)
				if err != nil {
					logger.Warning("execCmd error:", err)
				}
			}
		}()
	}
}

// 响应键盘背光模式切换
func (m *Manager) handleKbdLight() {
	data, err := ioutil.ReadFile("/sys/class/backlight/n70z/n70z_kbbacklight")
	if err != nil {
		logger.Warning(err)
		return
	}

	mode := strings.TrimSpace(string(data))
	switch mode {
	case "0":
		showOSD("KbLightClose")
	case "1":
		showOSD("KbLightAuto")
	case "2":
		showOSD("KbLightLow")
	case "3":
		showOSD("KbLightHigh")
	}
	logger.Debugf("switch kbd light mode to %v", mode)
}
