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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusNotifierWatcher_GetInterfaceName(t *testing.T) {
	s := StatusNotifierWatcher{}
	assert.Equal(t, snwDBusServiceName, s.GetInterfaceName())
}

/*
func TestStatusNotifierWatcher_isDBusServiceRegistered(t *testing.T) {
	type args struct {
		serviceName string
	}
	tests := []struct {
		name string
		snw  *StatusNotifierWatcher
		args args
		want bool
	}{
		{
			name: "isDBusServiceRegistered",
			snw: &StatusNotifierWatcher{
				dbusDaemon: &ofdbus.MockDBus{},
			},
			args: args{},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.snw.isDBusServiceRegistered(tt.args.serviceName)
			assert.Equal(t, tt.want, got)
		})
	}
}
*/
