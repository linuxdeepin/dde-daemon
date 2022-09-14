// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_hasIntersection(t *testing.T) {
	rect1 := Rect{0, 0, 100, 100}
	rect2 := Rect{0, 0, 50, 50}
	rect3 := Rect{1, 1, 30, 30}
	rect4 := Rect{32, 1, 15, 20}
	rect5 := Rect{32, 22, 15, 15}

	assert.True(t, hasIntersection(&rect2, &rect1))
	assert.True(t, hasIntersection(&rect3, &rect1))
	assert.True(t, hasIntersection(&rect3, &rect2))
	assert.True(t, hasIntersection(&rect4, &rect1))
	assert.True(t, hasIntersection(&rect4, &rect2))
	assert.False(t, hasIntersection(&rect4, &rect3))
	assert.True(t, hasIntersection(&rect5, &rect1))
	assert.True(t, hasIntersection(&rect5, &rect2))
	assert.False(t, hasIntersection(&rect5, &rect3))
	assert.False(t, hasIntersection(&rect5, &rect4))
}
