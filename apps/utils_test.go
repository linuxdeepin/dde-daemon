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

package apps

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getHomeByUid(t *testing.T) {
	uid := os.Getuid()
	home, err := getHomeByUid(uid)
	assert.Nil(t, err)
	assert.Equal(t, home, os.Getenv("HOME"))
}

func Test_isDesktopFile(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "isDesktopFile",
			args: args{
				path: "/usr/share/applications/firefox.desktop",
			},
			want: true,
		},
		{
			name: "isDesktopFile not a desktop",
			args: args{
				path: "/usr/share/applications/firefox.xxx",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDesktopFile(tt.args.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_removeDesktopExt(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{

		{
			name: "removeDesktopFile",
			args: args{
				name: "firefox.desktop",
			},
			want: "firefox",
		},
		{
			name: "removeDesktopFile not a desktop",
			args: args{
				name: "firefox.xxx",
			},
			want: "firefox.xxx",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeDesktopExt(tt.args.name)
			assert.Equal(t, tt.want, got)
		})
	}
}
