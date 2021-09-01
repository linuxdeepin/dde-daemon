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

func Test_intSliceContains(t *testing.T) {
	assert.True(t, intSliceContains([]int{1, 2, 3, 4, 5}, 4))
	assert.False(t, intSliceContains([]int{1, 2, 3, 4, 5}, 6))
}

func Test_intSliceRemove(t *testing.T) {
	testData := []struct {
		arg1 []int
		arg2 int
		ret  []int
	}{
		{
			arg1: []int{1, 2, 3, 4, 5},
			arg2: int(1),
			ret:  []int{2, 3, 4, 5},
		},

		{
			arg1: []int{1, 1, 1, 4, 5},
			arg2: int(1),
			ret:  []int{4, 5},
		},
	}

	for _, data := range testData {
		assert.Equal(t, data.ret, intSliceRemove(data.arg1, data.arg2))
	}
}

func Test_readDirNames(t *testing.T) {
	testData := []struct {
		arg1 string
		ret1 []string
		ret2 error
	}{
		{
			arg1: "testdata",
			ret1: []string{"test.notdesktop", "test.desktop"},
			ret2: nil,
		},
	}

	for _, data := range testData {
		names, err := readDirNames(data.arg1)
		assert.ElementsMatch(t, data.ret1, names)
		assert.Equal(t, data.ret2, err)
	}
}

func Test_desktopExt(t *testing.T) {
	assert.Equal(t, desktopExt, ".desktop")
}

func Test_isDesktopFile(t *testing.T) {
	testData := []struct {
		arg string
		ret bool
	}{
		{
			arg: "testdata/test.notdesktop",
			ret: false,
		},

		{
			arg: "testdata/test.desktop",
			ret: true,
		},
	}

	for _, data := range testData {
		assert.Equal(t, data.ret, isDesktopFile(data.arg))
	}
}
