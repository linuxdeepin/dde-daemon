// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
