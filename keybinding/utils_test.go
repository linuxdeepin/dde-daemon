// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package keybinding

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_shouldUseDDEKwin(t *testing.T) {
	_, err := exec.LookPath("deepin-kwin_x11")
	exist1 := err == nil
	exist2 := shouldUseDDEKwin()
	assert.Equal(t, exist1, exist2)
}
