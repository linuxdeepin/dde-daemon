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

package users

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isStrInArray(t *testing.T) {

	var var1 = []string{"/usr/share", "/ect/passwd", "/etc/cpus"}
	assert.Equal(t, isStrInArray("/etc/cpus", var1), true)
	assert.Equal(t, isStrInArray("/etc/abc", var1), false)
}

func Test_strToInt(t *testing.T) {

	fmt.Println(strToInt("4", -1))

	assert.Equal(t, strToInt("4", -1), 4)
	assert.Equal(t, strToInt("\\4", -1), -1)
	assert.Equal(t, strToInt("-2", -1), -2)

}

func Test_getDeepinReleaseType(t *testing.T) {

	var versions = []string{"Desktop", "Professional",
		"Server", "Personal",
		"",
	}

	assert.Contains(t, versions, systemType())
}
