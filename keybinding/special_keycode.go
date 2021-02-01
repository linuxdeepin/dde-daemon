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
	"github.com/godbus/dbus"
	power "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.power"
)

// 按键码
// https://github.com/torvalds/linux/blob/master/include/uapi/linux/input-event-codes.h
// nolint
const (
	KEY_TOUCHPAD_TOGGLE = 0x212
	KEY_POWER           = 116
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

	// 电源键，松开时触发
	key = createSpecialKeycodeIndex(KEY_POWER, false, MODIFY_NONE)
	m.specialKeycodeBindingList[key] = m.handlePower
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

// 切换触摸板状态
func (m *Manager) handleTouchpadToggle() {
	showOSD("TouchpadToggle")
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
			if !locked {
				err = m.execCmd(cmd, false)
				if err != nil {
					logger.Warning("execCmd error:", err)
				}
			}
		}()
	}
}
