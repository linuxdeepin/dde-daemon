// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
