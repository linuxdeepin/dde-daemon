// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_diffSortedWindowSlice(t *testing.T) {
	a := windowSlice{1, 2, 3, 4}
	b := windowSlice{1, 3, 5, 6, 7}
	add, remove := diffSortedWindowSlice(a, b)

	assert.Equal(t, len(add), 3)
	assert.Equal(t, int(add[0]), 5)
	assert.Equal(t, int(add[1]), 6)
	assert.Equal(t, int(add[2]), 7)

	assert.Equal(t, len(remove), 2)
	assert.Equal(t, int(remove[0]), 2)
	assert.Equal(t, int(remove[1]), 4)
}
