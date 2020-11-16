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

// #include "libinput_bridge.h"
// #cgo pkg-config: libinput glib-2.0
// #cgo LDFLAGS: -ludev -lm
import "C"

// nolint
// 按键状态
const (
	KEY_STATE_RELEASED = 0 // 松开
	KEY_STATE_PRESSED  = 1 // 按下
)

type KeyEvent struct {
	Keycode uint32 // 按键码
	State   uint32 // KEY_STATE_RELEASED,KEY_STATE_PRESSED
}

var eventChanList []chan *KeyEvent

// 开始监控按键
func startKeyEventMonitor() {
	go func() {
		C.loop_startup()
	}()
}

// 停止监控按键事件
func stopKeyEventMonitor() {
	C.loop_stop()
}

// 添加channel用于读取按键事件
func addKeyEventChannel(ch chan *KeyEvent) {
	eventChanList = append(eventChanList, ch)
}

//export pushKeyEvent
func pushKeyEvent(keycode uint32, state uint32) {
	event := &KeyEvent{
		Keycode: keycode,
		State:   state,
	}

	for _, ch := range eventChanList {
		select {
		case ch <- event:
		default:
		}
	}
}
