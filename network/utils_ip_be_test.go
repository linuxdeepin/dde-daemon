// +build mips mips64 ppc64 s390x

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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ntohl_be(t *testing.T) {
	tests := []struct {
		n    uint32
		want uint32
	}{
		{
			n:    0x01020304,
			want: 0x01020304,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("0x%x", tt.n), func(t *testing.T) {
			got := ntohl(tt.n)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_htonl_be(t *testing.T) {
	tests := []struct {
		n    uint32
		want uint32
	}{
		{
			n:    0x01020304,
			want: 0x01020304,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("0x%x", tt.n), func(t *testing.T) {
			got := ntohl(tt.n)
			assert.Equal(t, tt.want, got)
		})
	}
}
