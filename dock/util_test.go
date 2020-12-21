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

package dock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_uniqStrSlice(t *testing.T) {
	slice := []string{"a", "b", "c", "c", "b", "a", "c"}
	slice = uniqStrSlice(slice)
	assert.Equal(t, len(slice), 3)
	assert.Equal(t, slice[0], "a")
	assert.Equal(t, slice[1], "b")
	assert.Equal(t, slice[2], "c")
}

func Test_strSliceEqual(t *testing.T) {
	sa := []string{"a", "b", "c"}
	sb := []string{"a", "b", "c", "d"}
	sc := sa[:]
	assert.False(t, strSliceEqual(sa, sb))
	assert.True(t, strSliceEqual(sa, sc))
}

func Test_strSliceContains(t *testing.T) {
	slice := []string{"a", "b", "c"}
	assert.True(t, strSliceContains(slice, "a"))
	assert.True(t, strSliceContains(slice, "b"))
	assert.True(t, strSliceContains(slice, "c"))
	assert.False(t, strSliceContains(slice, "d"))
	assert.False(t, strSliceContains(slice, "e"))

}
