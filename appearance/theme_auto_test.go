// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getThemeAutoName(t *testing.T) {
	assert.Equal(t, getThemeAutoName(true), "deepin")
	assert.Equal(t, getThemeAutoName(false), "deepin-dark")
}
