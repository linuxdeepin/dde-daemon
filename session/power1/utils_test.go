package power1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/linuxdeepin/go-lib/gettext"
)

func TestGetNotifyString(t *testing.T) {
	assert.Equal(t, getNotifyString(settingKeyLinePowerLidClosedAction, powerActionShutdown), Tr("When the lid is closed, ")+Tr("your computer will shut down"))
	assert.Equal(t, getNotifyString(settingKeyLinePowerLidClosedAction, powerActionSuspend), Tr("When the lid is closed, ")+Tr("your computer will suspend"))
}
