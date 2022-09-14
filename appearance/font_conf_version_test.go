// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isVersionRight(t *testing.T) {
	assert.True(t, isVersionRight("1.4", "testdata/fontVersionConf"))
}
