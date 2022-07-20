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

package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isVolumeValid(t *testing.T) {
	assert.True(t, isVolumeValid(0))
	assert.False(t, isVolumeValid(-1))
}
func Test_floatPrecision(t *testing.T) {
	assert.Equal(t, floatPrecision(3.1415926), 3.14)
	assert.Equal(t, floatPrecision(2.718281828), 2.72)
}

func Test_toJSON(t *testing.T) {
	str1 := make(map[string]string)
	str1["name"] = "uniontech"
	str1["addr"] = "wuhan"
	ret := toJSON(str1)
	assert.Equal(t, ret, `{"addr":"wuhan","name":"uniontech"}`)
}
