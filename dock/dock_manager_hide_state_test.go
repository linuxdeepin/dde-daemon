/*
 * Copyright (C) 2020 ~ 2021 Deepin Technology Co., Ltd.
 *
 * Author:     weizhixiang <1138871845@qq.com>
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
