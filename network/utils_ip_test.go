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

package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ipToUint32(t *testing.T) {
	tests := []struct {
		ip   string
		want uint32
	}{
		{
			ip:   "1.2.3.4",
			want: 0x01020304,
		},
		{
			ip:   "255.255.255.255",
			want: 0xffffffff,
		},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := ipToUint32(tt.ip)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_uint32ToIP(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		l    uint32
		want string
	}{
		{
			l:    0x01020304,
			want: "1.2.3.4",
		},
		{
			l:    0xffffffff,
			want: "255.255.255.255",
		},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := uint32ToIP(tt.l)
			assert.Equal(t, tt.want, got)
		})
	}
}
