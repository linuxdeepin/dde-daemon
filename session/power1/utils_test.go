// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/linuxdeepin/go-lib/gettext"
)

func TestGetNotifyString(t *testing.T) {
	assert.Equal(t, getNotifyString(settingKeyLinePowerLidClosedAction, powerActionShutdown), Tr("When the lid is closed, ")+Tr("your computer will shut down"))
	assert.Equal(t, getNotifyString(settingKeyLinePowerLidClosedAction, powerActionSuspend), Tr("When the lid is closed, ")+Tr("your computer will suspend"))
}
