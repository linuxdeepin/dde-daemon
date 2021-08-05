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

package inputdevices

import (
	"testing"

	"github.com/godbus/dbus"
	"github.com/stretchr/testify/assert"
)

func Test_touchscreenSliceEqual(t *testing.T) {
	v1 := []dbus.ObjectPath{
		"/com/deepin/ABRecovery",
		"/com/deepin/anything",
	}

	v2 := []dbus.ObjectPath{
		"/com/deepin/api/Device",
	}
	ok := touchscreenSliceEqual(v1, v2)
	assert.False(t, ok)

	v3 := []dbus.ObjectPath{
		"/com/deepin/api/Device",
		"/com/deepin/anything",
	}
	ok = touchscreenSliceEqual(v1, v3)
	assert.False(t, ok)

	ok = touchscreenSliceEqual(v1, v1)
	assert.True(t, ok)
}

func Test_getIndexByDevNode(t *testing.T) {
	m := InputDevices{}
	m.touchscreens = map[dbus.ObjectPath]*Touchscreen{
		"": {
			DevNode: "/com/deepin/ABRecovery",
		},
	}
	i := m.getIndexByDevNode("")
	assert.Equal(t, -1, i)

	m.touchscreens = map[dbus.ObjectPath]*Touchscreen{
		"/com/deepin/ABRecovery": {
			DevNode: "/com/deepin/ABRecovery",
		},
	}
	i = m.getIndexByDevNode("/com/deepin/ABRecovery")
	assert.Equal(t, -1, i)

	m.Touchscreens = []dbus.ObjectPath{
		"/com/deepin/ABRecovery",
	}
	i = m.getIndexByDevNode("/com/deepin/ABRecovery")
	assert.Equal(t, 0, i)
}

func Test_SimpleFunc(t *testing.T) {
	m := InputDevices{}
	m.init()
	m.GetInterfaceName()
	newInputDevices()
}
