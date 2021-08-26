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

package network

import (
	"testing"

	dbus "github.com/godbus/dbus"
	"github.com/stretchr/testify/assert"
)

func Test_IsDeviceEnabled(t *testing.T) {
	n := Network{
		config: &Config{
			VpnEnabled: false,
			Devices: map[string]*DeviceConfig{
				"test": {
					Enabled: true,
				},
			},
		},
		devices: map[dbus.ObjectPath]*device{
			"1": {
				iface: "iFace",
				type0: 0,
			},
		},
	}

	enable, err := n.IsDeviceEnabled("iFace")
	assert.True(t, enable)
	assert.Nil(t, err)

	enable, err = n.IsDeviceEnabled("iFace1")
	assert.False(t, enable)
	assert.NotNil(t, err)
}

func Test_isDeviceEnabled(t *testing.T) {
	n := Network{
		config: &Config{
			VpnEnabled: false,
			Devices: map[string]*DeviceConfig{
				"test": {
					Enabled: true,
				},
			},
		},
		devices: map[dbus.ObjectPath]*device{
			"1": {
				iface: "iFace",
				type0: 0,
			},
		},
	}

	enable, err := n.isDeviceEnabled("iFace")
	assert.True(t, enable)
	assert.Nil(t, err)

	enable, err = n.isDeviceEnabled("iFace1")
	assert.False(t, enable)
	assert.NotNil(t, err)
}

func Test_findDevice(t *testing.T) {
	n := Network{}
	n.findDevice("/org/freedesktop/NetworkManager")

	n.findDevice("test")
}

func Test_getDeviceByIface(t *testing.T) {
	n := Network{}
	device1 := n.getDeviceByIface("test")
	assert.Nil(t, device1)

	n = Network{
		devices: map[dbus.ObjectPath]*device{
			"1": {
				iface: "test",
				type0: 0,
			},
		},
	}

	device1 = n.getDeviceByIface("test")
	assert.Equal(t, uint32(0), device1.type0)
}

func Test_saveConfig_Network(t *testing.T) {
	n := Network{}
	n.saveConfig()
}
