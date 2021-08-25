/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
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
	"time"

	dbus "github.com/godbus/dbus"
	"github.com/stretchr/testify/assert"
	"pkg.deepin.io/dde/daemon/network/nm"
)

func Test_notifyManager(t *testing.T) {
	initNotifyManager()

	disableNotify()

	assert.Equal(t, false, notifyEnabled)
	enableNotify()
	assert.Equal(t, false, notifyEnabled)

	time.Sleep(10 * time.Second)
	assert.Equal(t, true, notifyEnabled)
}

func Test_notifyMsgQueue(t *testing.T) {
	queue := newNotifyMsgQueue()

	tests := []*notifyMsg{
		{
			icon:    "test1",
			summary: "test1",
			body:    "test1",
		},
		{
			icon:    "test2",
			summary: "test2",
			body:    "test2",
		},
		{
			icon:    "test3",
			summary: "test3",
			body:    "test3",
		},
	}

	queue.enqueue(tests[0])
	queue.enqueue(tests[1])
	queue.dequeue()
	queue.enqueue(tests[2])

	entrys := queue.toList()
	assert.Equal(t, 2, entrys.Len())
	assert.Equal(t, tests[1], entrys.Front().Value)
	assert.Equal(t, tests[2], entrys.Back().Value)
}

func Test_NotifyManager(t *testing.T) {
	initNotifyManager()
	disableNotify()
	globalSessionActive = true

	globalNotifyManager.addMsg(&notifyMsg{
		icon:    "test",
		summary: "test",
		body:    "test",
	})

	notifyAirplanModeEnabled()
	notifyWiredCableUnplugged()
	notifyApModeNotSupport()
	notifyWirelessHardSwitchOff()
	notifyProxyEnabled()
	notifyProxyDisabled()
	notifyVpnConnected("test")
	notifyVpnDisconnected("test")
	notifyVpnFailed("test", 0)

	get := globalNotifyManager.count()
	assert.Equal(t, true, get > 0)

	time.Sleep(30 * time.Second)
	get = globalNotifyManager.count()
	assert.Equal(t, 0, get)
}

func Test_getMobileConnectedNotifyIcon(t *testing.T) {
	tests := []struct {
		arg  string
		want string
	}{
		{moblieNetworkType4G, notifyIconMobile4gConnected},
		{moblieNetworkType3G, notifyIconMobile3gConnected},
		{moblieNetworkType2G, notifyIconMobile2gConnected},
		{moblieNetworkTypeUnknown, notifyIconMobileUnknownConnected},
		{"test", notifyIconMobileUnknownConnected},
	}

	for _, e := range tests {
		get := getMobileConnectedNotifyIcon(e.arg)
		assert.Equal(t, e.want, get)
	}
}

func Test_getMobileDisconnectedNotifyIcon(t *testing.T) {
	tests := []struct {
		arg  string
		want string
	}{
		{moblieNetworkType4G, notifyIconMobile4gDisconnected},
		{moblieNetworkType3G, notifyIconMobile3gDisconnected},
		{moblieNetworkType2G, notifyIconMobile2gDisconnected},
		{moblieNetworkTypeUnknown, notifyIconMobileUnknownDisconnected},
		{"test", notifyIconMobileUnknownDisconnected},
	}

	for _, e := range tests {
		get := getMobileDisconnectedNotifyIcon(e.arg)
		assert.Equal(t, e.want, get)
	}
}

func Test_generalGetNotifyConnectedIcon(t *testing.T) {
	tests := []struct {
		arg1 uint32
		arg2 dbus.ObjectPath
		want string
	}{
		{nm.NM_DEVICE_TYPE_ETHERNET, "", notifyIconWiredConnected},
		{nm.NM_DEVICE_TYPE_WIFI, "", notifyIconWirelessConnected},
		{nm.NM_DEVICE_TYPE_GENERIC, "", notifyIconNetworkConnected},
	}

	for _, e := range tests {
		get := generalGetNotifyConnectedIcon(e.arg1, e.arg2)
		assert.Equal(t, e.want, get)
	}
}

func Test_generalGetNotifyDisconnectedIcon(t *testing.T) {
	tests := []struct {
		arg1 uint32
		arg2 dbus.ObjectPath
		want string
	}{
		{nm.NM_DEVICE_TYPE_ETHERNET, "", notifyIconWiredDisconnected},
		{nm.NM_DEVICE_TYPE_WIFI, "", notifyIconWirelessDisconnected},
		{nm.NM_DEVICE_TYPE_GENERIC, "", notifyIconNetworkDisconnected},
	}

	for _, e := range tests {
		get := generalGetNotifyDisconnectedIcon(e.arg1, e.arg2)
		assert.Equal(t, e.want, get)
	}
}

func Test_notifyDeviceRemoved(t *testing.T) {
	manager = &Manager{}
	manager.devices = map[string][]*device{}
	manager.devices["test"] = []*device{
		{
			Path:      "test",
			nmDevType: nm.NM_DEVICE_TYPE_ETHERNET,
		},
	}
	initNotifyManager()
	disableNotify()
	globalSessionActive = true
	notifyDeviceRemoved("test")

	time.Sleep(3 * time.Second)
	get := globalNotifyManager.count()
	assert.Equal(t, 0, get)
}
