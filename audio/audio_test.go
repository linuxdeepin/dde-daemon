package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"

	dbus "github.com/godbus/dbus"
	"pkg.deepin.io/lib/pulse"
)

func Test_objectPathSliceEqual(t *testing.T) {
	var str = []dbus.ObjectPath{"/com/deepin/daemon/Bluetooth", "/com/deepin/daemon/Audio"}
	var str1 = []dbus.ObjectPath{"/com/deepin/daemon/Bluetooth", "/com/deepin/daemon/Audio"}
	var str2 = []dbus.ObjectPath{"/com/deepin/daemon/Bluetooth", "/com/deepin/daemon/Audio", "/"}
	var str3 = []dbus.ObjectPath{"/com/deepin/daemon/Bluetooth", "/com/deepin/daemon/Accounts"}

	assert.Equal(t, objectPathSliceEqual(str, str1), true)
	assert.Equal(t, objectPathSliceEqual(str, str2), false)
	assert.Equal(t, objectPathSliceEqual(str, str3), false)
}

func Test_isPortExists(t *testing.T) {
	var tests = []pulse.PortInfo{
		{"MG-T1S", "audio", 1, pulse.AvailableTypeUnknow},
		{"MG-T1S", "audio", 2, pulse.AvailableTypeNo},
		{"MG-T1S", "audio", 3, pulse.AvailableTypeYes},
		{"", "audio", 4, pulse.AvailableTypeUnknow},
		{"", "audio", 5, pulse.AvailableTypeNo},
		{"", "audio", 6, pulse.AvailableTypeYes},
		{"MG-T1S", "audio", 7, pulse.AvailableTypeUnknow},
		{"MG-T1S", "audio", 8, pulse.AvailableTypeNo},
		{"MG-T1S", "audio", 9, pulse.AvailableTypeYes},
	}
	assert.Equal(t, isPortExists("MG-T1S", tests), true)
	assert.Equal(t, isPortExists("", tests), true)
	assert.Equal(t, isPortExists("MG", tests), false)
}

func Test_getBestPort(t *testing.T) {
	var tests = []pulse.PortInfo{
		{"MG-T1S", "audio", 1, pulse.AvailableTypeUnknow},
		{"MG-T1S", "audio", 2, pulse.AvailableTypeNo},
		{"MG-T1S", "audio", 3, pulse.AvailableTypeYes},
		{"", "audio", 4, pulse.AvailableTypeUnknow},
		{"", "audio", 5, pulse.AvailableTypeNo},
		{"", "audio", 6, pulse.AvailableTypeYes},
		{"MG-T1S", "audio", 7, pulse.AvailableTypeUnknow},
		{"MG-T1S", "audio", 8, pulse.AvailableTypeNo},
		{"MG-T1S", "audio", 9, pulse.AvailableTypeYes},
	}
	var ret = pulse.PortInfo{"MG-T1S", "audio", 9, pulse.AvailableTypeYes}
	ret1 := getBestPort(tests)
	assert.Equal(t, ret1, ret)
}
