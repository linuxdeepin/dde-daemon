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

package keyevent

import "testing"

func Test_handleEvent(t *testing.T) {
	m := Manager{}
	ev := &KeyEvent{
		Keycode: KEY_LEFTCTRL,
	}
	m.handleEvent(ev)

	ev.Keycode = KEY_RIGHTCTRL
	m.handleEvent(ev)

	ev.Keycode = KEY_LEFTALT
	m.handleEvent(ev)

	ev.Keycode = KEY_RIGHTALT
	m.handleEvent(ev)

	ev.Keycode = KEY_LEFTSHIFT
	m.handleEvent(ev)

	ev.Keycode = KEY_RIGHTSHIFT
	m.handleEvent(ev)

	ev.Keycode = KEY_LEFTMETA
	m.handleEvent(ev)

	ev.Keycode = KEY_RIGHTMETA
	m.handleEvent(ev)
}

func Test_SimpleFunc(t *testing.T) {
	m := Manager{}
	m.GetInterfaceName()
}

