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

package gesture

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isInWindowBlacklist(t *testing.T) {
	slice := []string{"window1", "window2", "window3"}
	assert.True(t, isInWindowBlacklist("window1", slice))
	assert.True(t, isInWindowBlacklist("window2", slice))
	assert.True(t, isInWindowBlacklist("window3", slice))
	assert.False(t,isInWindowBlacklist("window4", slice))
}
