// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWarnLevelConfig(t *testing.T) {
	conf := &warnLevelConfig{
		UsePercentageForPolicy: true,

		LowTime:      1200,
		DangerTime:   900,
		CriticalTime: 600,
		ActionTime:   300,

		LowPowerNotifyThreshold: 20,
		remindPercentage:        25,
		LowPercentage:           20,
		DangerPercentage:        15,
		CriticalPercentage:      10,
		ActionPercentage:        5,
	}
	assert.True(t, conf.isValid())
	conf.LowTime = 599
	assert.False(t, conf.isValid())

	conf.LowTime = 1200
	conf.LowPercentage = 9
	assert.False(t, conf.isValid())
}

func Test_getWarnLevel(t *testing.T) {
	config := &warnLevelConfig{
		UsePercentageForPolicy: true,

		LowTime:      1200,
		DangerTime:   900,
		CriticalTime: 600,
		ActionTime:   300,

		LowPowerNotifyThreshold: 20,
		remindPercentage:        25,
		LowPercentage:           20,
		DangerPercentage:        15,
		CriticalPercentage:      10,
		ActionPercentage:        5,
	}

	onBattery := false
	assert.Equal(t, getWarnLevel(config, onBattery, 1.0, 0), WarnLevelNone)

	onBattery = true
	config.UsePercentageForPolicy = true

	assert.Equal(t, getWarnLevel(config, onBattery, 0.0, 0), WarnLevelNone)
	assert.Equal(t, getWarnLevel(config, onBattery, 1.1, 0), WarnLevelAction)
	assert.Equal(t, getWarnLevel(config, onBattery, 5.0, 0), WarnLevelAction)
	assert.Equal(t, getWarnLevel(config, onBattery, 5.1, 0), WarnLevelCritical)
	assert.Equal(t, getWarnLevel(config, onBattery, 10.0, 0), WarnLevelCritical)
	assert.Equal(t, getWarnLevel(config, onBattery, 10.1, 0), WarnLevelDanger)
	assert.Equal(t, getWarnLevel(config, onBattery, 15.0, 0), WarnLevelDanger)
	assert.Equal(t, getWarnLevel(config, onBattery, 15.1, 0), WarnLevelLow)
	assert.Equal(t, getWarnLevel(config, onBattery, 20.0, 0), WarnLevelLow)
	assert.Equal(t, getWarnLevel(config, onBattery, 20.1, 0), WarnLevelNone)
	assert.Equal(t, getWarnLevel(config, onBattery, 50.0, 0), WarnLevelNone)

	config.UsePercentageForPolicy = false
	// use time to empty
	assert.Equal(t, getWarnLevel(config, onBattery, 0, 0), WarnLevelNone)
	assert.Equal(t, getWarnLevel(config, onBattery, 0, 61), WarnLevelAction)
	assert.Equal(t, getWarnLevel(config, onBattery, 0, 300), WarnLevelAction)
	assert.Equal(t, getWarnLevel(config, onBattery, 0, 301), WarnLevelCritical)
	assert.Equal(t, getWarnLevel(config, onBattery, 0, 600), WarnLevelCritical)
	assert.Equal(t, getWarnLevel(config, onBattery, 0, 601), WarnLevelDanger)
	assert.Equal(t, getWarnLevel(config, onBattery, 0, 900), WarnLevelDanger)
	assert.Equal(t, getWarnLevel(config, onBattery, 0, 901), WarnLevelLow)
	assert.Equal(t, getWarnLevel(config, onBattery, 0, 1200), WarnLevelLow)
	assert.Equal(t, getWarnLevel(config, onBattery, 0, 12001), WarnLevelNone)
}

func TestManagerSystemApplicationsSnapshotReturnsCopy(t *testing.T) {
	m := &Manager{
		systemApplicationsMap: map[string]string{"deepin-terminal.desktop": "/usr/share/applications/deepin-terminal.desktop"},
	}

	snapshot := m.systemApplicationsSnapshot()
	snapshot["browser.desktop"] = "/usr/share/applications/browser.desktop"

	_, exists := m.systemApplicationsMap["browser.desktop"]
	assert.False(t, exists)
	assert.Equal(t, "/usr/share/applications/deepin-terminal.desktop", snapshot["deepin-terminal.desktop"])
}

func TestManagerReplaceSystemApplicationsReplacesContents(t *testing.T) {
	m := &Manager{
		systemApplicationsMap: map[string]string{"old.desktop": "old.desktop"},
	}
	psp := &powerSavePlan{manager: m}

	m.replaceSystemApplications([]string{"/usr/share/applications/deepin-terminal.desktop", "browser.desktop"}, psp.getDesktopName)

	assert.Equal(t, map[string]string{
		"deepin-terminal.desktop": "/usr/share/applications/deepin-terminal.desktop",
		"browser.desktop":         "browser.desktop",
	}, m.systemApplicationsSnapshot())
}

func TestManagerShortIdleBlacklistSnapshotReturnsCopy(t *testing.T) {
	m := &Manager{
		shortIdleBlackListApplicationsMap: map[string]string{"deepin-movie.desktop": "/usr/share/applications/deepin-movie.desktop"},
	}

	snapshot := m.shortIdleBlacklistApplicationsSnapshot()
	snapshot["browser.desktop"] = "/usr/share/applications/browser.desktop"

	_, exists := m.shortIdleBlackListApplicationsMap["browser.desktop"]
	assert.False(t, exists)
	assert.Equal(t, "/usr/share/applications/deepin-movie.desktop", snapshot["deepin-movie.desktop"])
}

func TestManagerReplaceShortIdleBlacklistApplicationsReplacesContents(t *testing.T) {
	m := &Manager{
		shortIdleBlackListApplicationsMap: map[string]string{"old.desktop": "old.desktop"},
	}
	psp := &powerSavePlan{manager: m}

	m.replaceShortIdleBlacklistApplications([]string{"/usr/share/applications/deepin-movie.desktop", "browser.desktop"}, psp.getDesktopName)

	assert.Equal(t, map[string]string{
		"deepin-movie.desktop": "/usr/share/applications/deepin-movie.desktop",
		"browser.desktop":      "browser.desktop",
	}, m.shortIdleBlacklistApplicationsSnapshot())
}

func TestManagerSystemServicesSnapshotReturnsCopy(t *testing.T) {
	m := &Manager{
		systemServicesMap: map[string]string{"dde-session-daemon.service": "dde-session-daemon.service"},
	}

	snapshot := m.systemServicesSnapshot()
	snapshot["custom.service"] = "custom.service"

	_, exists := m.systemServicesMap["custom.service"]
	assert.False(t, exists)
	assert.Equal(t, "dde-session-daemon.service", snapshot["dde-session-daemon.service"])
}

func TestManagerReplaceSystemServicesReplacesContents(t *testing.T) {
	m := &Manager{
		systemServicesMap: map[string]string{"old.service": "old.service"},
	}

	m.replaceSystemServices([]string{"dde-session-daemon.service", "NetworkManager.service"})

	assert.Equal(t, map[string]string{
		"dde-session-daemon.service": "dde-session-daemon.service",
		"NetworkManager.service":     "NetworkManager.service",
	}, m.systemServicesSnapshot())
}

func TestMetaTasksMin(t *testing.T) {
	tasks := metaTasks{
		metaTask{
			name:  "n1",
			delay: 10,
		},
		metaTask{
			name:  "n2",
			delay: 30,
		},
		metaTask{
			name:  "n3",
			delay: 20,
		},
	}
	assert.Equal(t, tasks.min(), int32(10))

	tasks = metaTasks{}
	assert.Equal(t, tasks.min(), int32(0))

	tasks = metaTasks{
		metaTask{
			name:  "n1",
			delay: 10,
		},
	}
	assert.Equal(t, tasks.min(), int32(10))
}
