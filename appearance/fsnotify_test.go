// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_hasEventOccurred(t *testing.T) {
	var shellStr = []string{"/bin/sh", "/bin/bash",
		"/bin/zsh", "/usr/bin/zsh",
		"/usr/bin/fish",
	}

	assert.Equal(t, hasEventOccurred("/usr/bin/sh", shellStr), true)
	assert.Equal(t, hasEventOccurred("/usr/lib/deepin", shellStr), false)
}
