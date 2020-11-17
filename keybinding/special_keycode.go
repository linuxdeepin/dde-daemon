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

// 按键码
// https://github.com/torvalds/linux/blob/master/include/uapi/linux/input-event-codes.h
// nolint
const (
	KEY_TOUCHPAD_TOGGLE = 0x212
)

type SpecialKeycodeMapKey struct {
	keycode      uint32 // 按键码
	pressed      bool   // true:按下事件，false松开事件
	ctrlPressed  bool   // ctrl 是否处于按下状态
	shiftPressed bool   // shift 是否处于按下状态
	altPressed   bool   // alt 是否处于按下状态
	superPressed bool   // super 是否处于按下状态
}

// 初始化按键到处理函数的map
func (m *Manager) initSpecialKeycodeMap() {
	m.specialKeycodeBindingList = make(map[SpecialKeycodeMapKey]func())

	key := SpecialKeycodeMapKey{
		KEY_TOUCHPAD_TOGGLE,
		true,
		false,
		false,
		false,
		false,
	}
	m.specialKeycodeBindingList[key] = m.handleTouchpadToggle
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
