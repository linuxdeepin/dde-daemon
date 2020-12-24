package power

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "pkg.deepin.io/lib/gettext"
)

func TestGetNotifyString(t *testing.T) {
	assert.Equal(t, getNotifyString(settingKeyLinePowerLidClosedAction, powerActionShutdown), Tr("When the lid is closed, ")+Tr("your computer will shut down"))
	assert.Equal(t, getNotifyString(settingKeyLinePowerLidClosedAction, powerActionSuspend), Tr("When the lid is closed, ")+Tr("your computer will suspend"))
}
