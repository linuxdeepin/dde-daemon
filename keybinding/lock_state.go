/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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
	"errors"
	"fmt"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/test"
	"github.com/linuxdeepin/go-x11-client/util/keysyms"
	"pkg.deepin.io/dde/daemon/keybinding/shortcuts"
)

// NumLockState 数字锁定键状态
type NumLockState uint

const (
	NumLockOff     NumLockState = iota // 关闭，不能输入数字，是方向键
	NumLockOn                          // 开启，能输入数字
	NumLockUnknown                     // 未知，出错
)

func (s NumLockState) String() string {
	switch s {
	case NumLockOff:
		return "off"
	case NumLockOn:
		return "on"
	case NumLockUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("invalid-num-lock-state(%d)", s)
	}
}

// CapsLockState 大小写锁定键状态
type CapsLockState uint

const (
	CapsLockOff     CapsLockState = iota // 关闭，默认输入小写字母
	CapsLockOn                           // 开启，默认输入大写字母
	CapsLockUnknown                      // 未知，出错
)

// 查询 NumLock 数字锁定键状态
func queryNumLockState(conn *x.Conn) (NumLockState, error) {
	rootWin := conn.GetDefaultScreen().Root
	queryPointerReply, err := x.QueryPointer(conn, rootWin).Reply(conn)
	if err != nil {
		return NumLockUnknown, err
	}
	logger.Debugf("query pointer reply %#v", queryPointerReply)
	on := queryPointerReply.Mask&x.ModMask2 != 0
	if on {
		return NumLockOn, nil
	} else {
		return NumLockOff, nil
	}
}

// 查询 CapsLock 大小写锁定键状态
func queryCapsLockState(conn *x.Conn) (CapsLockState, error) {
	rootWin := conn.GetDefaultScreen().Root
	queryPointerReply, err := x.QueryPointer(conn, rootWin).Reply(conn)
	if err != nil {
		return CapsLockUnknown, err
	}
	logger.Debugf("query pointer reply %#v", queryPointerReply)
	on := queryPointerReply.Mask&x.ModMaskLock != 0
	if on {
		return CapsLockOn, nil
	} else {
		return CapsLockOff, nil
	}
}

// 设置 NumLock 数字锁定键状态
func setNumLockState(conn *x.Conn, keySymbols *keysyms.KeySymbols, state NumLockState) error {
	logger.Debug("setNumLockState", state)
	if !(state == NumLockOff || state == NumLockOn) {
		return errors.New("invalid num lock state")
	}

	state0, err := queryNumLockState(conn)
	if err != nil {
		return err
	}

	if state0 != state {
		return changeNumLockState(conn, keySymbols)
	}
	return nil
}

// 设置 CapsLock 大小写锁定键状态
func setCapsLockState(conn *x.Conn, keySymbols *keysyms.KeySymbols, state CapsLockState) error {
	if !(state == CapsLockOff || state == CapsLockOn) {
		return errors.New("invalid caps lock state")
	}

	state0, err := queryCapsLockState(conn)
	if err != nil {
		return err
	}

	if state0 != state {
		return changeCapsLockState(conn, keySymbols)
	}
	return nil
}

// 修改 NumLock 数字锁定键状态，模拟一次相应的键按下和释放。
func changeNumLockState(conn *x.Conn, keySymbols *keysyms.KeySymbols) (err error) {
	// get Num_Lock keycode
	numLockKeycode, err := shortcuts.GetKeyFirstCode(keySymbols, "Num_Lock")
	if err != nil {
		return err
	}
	logger.Debug("numLockKeycode is", numLockKeycode)
	return simulatePressReleaseKey(conn, numLockKeycode)
}

// 修改 CapsLock 大小写锁定键状态，模拟一次相应的键按下释放。
func changeCapsLockState(conn *x.Conn, keySymbols *keysyms.KeySymbols) (err error) {
	// get Caps_Lock keycode
	capsLockKeycode, err := shortcuts.GetKeyFirstCode(keySymbols, "Caps_Lock")
	if err != nil {
		return err
	}
	logger.Debug("capsLockKeycode is", capsLockKeycode)
	return simulatePressReleaseKey(conn, capsLockKeycode)
}

// 模拟键的按下和释放
func simulatePressReleaseKey(conn *x.Conn, code x.Keycode) error {
	rootWin := conn.GetDefaultScreen().Root
	// fake key press
	err := test.FakeInputChecked(conn, x.KeyPressEventCode, byte(code), x.TimeCurrentTime, rootWin, 0, 0, 0).Check(conn)
	if err != nil {
		return err
	}
	// fake key release
	err = test.FakeInputChecked(conn, x.KeyReleaseEventCode, byte(code), x.TimeCurrentTime, rootWin, 0, 0, 0).Check(conn)
	if err != nil {
		return err
	}
	return nil
}
