/*
 * Copyright (C) 2016 ~ 2017 Deepin Technology Co., Ltd.
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

package lastore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Hook up gocheck into the "go test" runner.

func TestStrSliceSetEqual(t *testing.T) {
	assert.Equal(t, strSliceSetEqual([]string{}, []string{}), true)
	assert.Equal(t, strSliceSetEqual([]string{"a"}, []string{"a"}), true)
	assert.Equal(t, strSliceSetEqual([]string{"a", "b"}, []string{"a"}), false)
	assert.Equal(t, strSliceSetEqual([]string{"a", "b", "d"}, []string{"b", "d", "a"}), true)
}
