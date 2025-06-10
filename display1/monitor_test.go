// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"testing"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/stretchr/testify/assert"
)

func Test_getFirstModeBySize(t *testing.T) {
	modes := []ModeInfo{{84, "", 1920, 1080, 60.0}, {85, "", 1920, 1080, 50.0},
		{95, "", 1600, 1200, 60.0}}
	assert.Equal(t, getFirstModeBySize(modes, 1920, 1080), ModeInfo{84, "", 1920, 1080, 60.0})
	assert.Equal(t, getFirstModeBySize(modes, 1600, 1200), ModeInfo{95, "", 1600, 1200, 60.0})
	assert.Equal(t, getFirstModeBySize(modes, 1280, 740), ModeInfo{})
}

func Test_getFirstModeBySizeRate(t *testing.T) {
	modes := []ModeInfo{{84, "", 1920, 1080, 60.0}, {85, "", 1920, 1080, 50.0},
		{95, "", 1600, 1200, 60.0}}
	assert.Equal(t, getFirstModeBySizeRate(modes, 1920, 1080, 59.999), ModeInfo{84, "", 1920, 1080, 60.0})
	assert.Equal(t, getFirstModeBySizeRate(modes, 1920, 1080, 49.999), ModeInfo{85, "", 1920, 1080, 50.0})
	assert.Equal(t, getFirstModeBySizeRate(modes, 1600, 1200, 59), ModeInfo{})
	assert.Equal(t, getFirstModeBySizeRate(modes, 1280, 740, 60), ModeInfo{})

}

func Test_getRandrStatusStr(t *testing.T) {
	var status = []uint8{0, 1, 2, 3, 4}
	var statusstr = []string{"success", "invalid config time", "invalid time", "failed", "unknown status 4"}
	assert.Equal(t, getRandrStatusStr(status[0]), statusstr[0])
	assert.Equal(t, getRandrStatusStr(status[1]), statusstr[1])
	assert.Equal(t, getRandrStatusStr(status[2]), statusstr[2])
	assert.Equal(t, getRandrStatusStr(status[3]), statusstr[3])
	assert.Equal(t, getRandrStatusStr(status[4]), statusstr[4])
}

//func Test_setXrandrScalingMode(t *testing.T) {
//	var fillMode = "None"
//	m := Monitor{}
//	err := m.setScalingMode(fillMode)
//	assert.NotNil(t, err)
//}

func Test_generateFillModeKey(t *testing.T) {
	m := Monitor{}
	m.uuid = "VGA-16813e57c97781347115de0e64f8277ec"
	m.Height = 1080
	m.Width = 1920
	assert.Equal(t, "VGA-16813e57c97781347115de0e64f8277ec:1920x1080",
		m.generateFillModeKey())

	m.Rotation = 2
	assert.Equal(t, "VGA-16813e57c97781347115de0e64f8277ec:1080x1920",
		m.generateFillModeKey())
}

func Test_setCurrentFillMode(t *testing.T) {
	m := Monitor{}
	write := &dbusutil.PropertyWrite{
		Value: "None",
	}

	m.m = &Manager{}
	err := m.setCurrentFillMode(write)
	assert.NotNil(t, err)
}
