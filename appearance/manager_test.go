// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_genMonitorKeyString(t *testing.T) {
	assert.Equal(t, genMonitorKeyString("abc", 123), "abc&&123")
	assert.Equal(t, genMonitorKeyString("abc", "def"), "abc&&def")
}
