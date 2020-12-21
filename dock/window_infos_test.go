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

func Test_windowInfosTypeEqual(t *testing.T) {
	wa := windowInfosType{
		0: {"a", false},
		1: {"b", false},
		2: {"c", true},
	}
	wb := windowInfosType{
		2: {"c", true},
		1: {"b", false},
		0: {"a", false},
	}
	assert.True(t, wa.Equal(wb))

	wc := windowInfosType{
		1: {"b", false},
		2: {"c", false},
	}
	assert.False(t, wc.Equal(wa))

	wd := windowInfosType{
		0: {"aa", false},
		1: {"b", false},
		2: {"c", false},
	}
	assert.False(t, wd.Equal(wa))

	we := windowInfosType{
		0: {"a", false},
		1: {"b", false},
		3: {"c", false},
	}
	assert.False(t, we.Equal(wa))

	wf := windowInfosType{
		0: {"a", false},
		1: {"b", false},
		2: {"c", false},
	}
	assert.False(t, wf.Equal(wa))
}
