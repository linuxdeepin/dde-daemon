/*
 * Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:     quezhiyong <quezhiyong@uniontech.com>
 *
 * Maintainer: quezhiyong <quezhiyong@uniontech.com>
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

package trayicon

import (
	"fmt"
	"testing"

	dbus "github.com/godbus/dbus"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
	"github.com/stretchr/testify/assert"
)

func TestStatusNotifierWatcher_GetInterfaceName(t *testing.T) {
	s := StatusNotifierWatcher{}
	assert.Equal(t, snwDBusServiceName, s.GetInterfaceName())
}

func TestStatusNotifierWatcher_isDBusServiceRegistered(t *testing.T) {
	type args struct {
		serviceName string
	}
	type test struct {
		name string
		snw  *StatusNotifierWatcher
		args args
		want bool
	}
	tests := []test{
		func() test {
			serviceName := "xxx"

			m := new(ofdbus.MockDBus)
			m.MockInterfaceDbusIfc.On("GetNameOwner", dbus.Flags(0), serviceName).Return("aaa", nil)

			return test{
				name: "isDBusServiceRegistered succ",
				snw: &StatusNotifierWatcher{
					dbusDaemon: m,
				},
				args: args{
					serviceName: serviceName,
				},
				want: true,
			}
		}(),
		func() test {
			serviceName := "xxx"

			m := new(ofdbus.MockDBus)
			m.MockInterfaceDbusIfc.On("GetNameOwner", dbus.Flags(0), serviceName).Return("", nil)

			return test{
				name: "isDBusServiceRegistered empty",
				snw: &StatusNotifierWatcher{
					dbusDaemon: m,
				},
				args: args{
					serviceName: serviceName,
				},
				want: false,
			}
		}(),
		func() test {
			serviceName := "xxx"

			m := new(ofdbus.MockDBus)
			m.MockInterfaceDbusIfc.On("GetNameOwner", dbus.Flags(0), serviceName).Return("", fmt.Errorf("xxx"))

			return test{
				name: "isDBusServiceRegistered error",
				snw: &StatusNotifierWatcher{
					dbusDaemon: m,
				},
				args: args{
					serviceName: serviceName,
				},
				want: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.snw.isDBusServiceRegistered(tt.args.serviceName)
			assert.Equal(t, tt.want, got)
		})
	}
}
