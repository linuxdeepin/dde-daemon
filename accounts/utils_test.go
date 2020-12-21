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

package accounts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var str = []string{"/bin/sh", "/bin/bash",
	"/bin/zsh", "/usr/bin/zsh",
	"/usr/bin/fish",
}

func Test_GetLocaleFromFile(t *testing.T) {
	assert.Equal(t, getLocaleFromFile("testdata/locale"), "zh_CN.UTF-8")
}

func Test_SystemLayout(t *testing.T) {
	layout, err := getSystemLayout("testdata/keyboard_us")
	assert.Nil(t, err)
	assert.Equal(t, layout, "us;")
	layout, _ = getSystemLayout("testdata/keyboard_us_chr")
	assert.Equal(t, layout, "us;chr")
}

func TestAvailableShells(t *testing.T) {
	var ret = []string{"/bin/sh", "/bin/bash",
		"/bin/zsh", "/usr/bin/zsh",
		"/usr/bin/fish",
	}
	shells := getAvailableShells("testdata/shells")
	assert.ElementsMatch(t, shells, ret)
}

func TestIsStrInArray(t *testing.T) {
	ret := isStrInArray("testdata/shells", str)
	assert.Equal(t, ret, false)
	ret = isStrInArray("/bin/sh", str)
	assert.Equal(t, ret, true)
}

func TestIsStrvEqual(t *testing.T) {
	var str1 = []string{"/bin/sh", "/bin/bash",
		"/bin/zsh", "/usr/bin/zsh",
		"/usr/bin/fish",
	}
	var str2 = []string{"/bin/sh", "/bin/bash"}
	ret := isStrvEqual(str, str1)
	assert.Equal(t, ret, true)
	ret = isStrvEqual(str, str2)
	assert.Equal(t, ret, false)
}

func TestGetValueFromLine(t *testing.T) {
	ret := getValueFromLine("testdata/shells", "/")
	assert.Equal(t, ret, "shells")
}
