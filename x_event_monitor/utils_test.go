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

package x_event_monitor

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_hasMotionFlag (t *testing.T) {
	var flag = []int32{0, 1}

	assert.False(t, hasMotionFlag(flag[0]))
	assert.True(t, hasMotionFlag(flag[1]))
}

func Test_hasKeyFlag (t *testing.T) {
	var flag = []int32{1, 4}

	assert.False(t, hasKeyFlag(flag[0]))
	assert.True(t, hasKeyFlag(flag[1]))
}

func Test_hasButtonFlag (t *testing.T) {
	var flag = []int32{1, 2}

	assert.False(t, hasButtonFlag(flag[0]))
	assert.True(t, hasButtonFlag(flag[1]))
}

func Test_isInArea (t *testing.T) {
	var area = coordinateRange {
		X1: 100,
		X2: 200,
		Y1: 100,
		Y2: 200,
	}

	var x = []int32 {99, 101}
	var y = []int32 {99, 101}

	assert.True(t, isInArea(x[1], y[1], area))
	assert.False(t, isInArea(x[0], y[0], area))
}

func Test_isInIdList (t *testing.T) {
	var list = []string {"tongxinruanjian","tongshenruanjian"}
	var md5str = []string {"tongxinruanjian","tongxin"}

	assert.True(t, isInIdList(md5str[0], list))
	assert.False(t, isInIdList(md5str[1], list))
}