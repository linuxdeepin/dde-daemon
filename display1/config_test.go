// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	configPath_v3  = "./testdata/display_v3.json"
	configPath_v4  = "./testdata/display_v4.json"
	configPath_v5  = "./testdata/display_v5.json"
	configPath_tmp = "./testdata/display_tmp.json"
)

func TestConfig(t *testing.T) {
	_, err := loadConfigV3D3(configPath_v3)
	require.Nil(t, err)

	_, err = loadConfigV4(configPath_v4)
	require.Nil(t, err)

	config, err := loadConfigV5V6(configPath_v5)
	require.Nil(t, err)

	screenConfig := config.ConfigV5["HDMI-1bc06f293ee6bfb16fd813648741f8ac3,eDP-12fd580d2dc41168dce2efb1bf19adb54"]
	require.NotNil(t, screenConfig)

	modeConfig := screenConfig.getModeConfigs(DisplayModeExtend)
	require.NotNil(t, modeConfig)

	monitors := modeConfig.Monitors
	require.NotEqual(t, len(monitors), 0)

	monitorConfig := getMonitorConfigByUuid(monitors, "eDP-12fd580d2dc41168dce2efb1bf19adb54")
	require.NotNil(t, monitorConfig)

	primaryMonitor := getMonitorConfigPrimary(monitors)
	require.Equal(t, primaryMonitor.UUID, monitorConfig.UUID)

	setMonitorConfigsPrimary(monitors, "HDMI-1bc06f293ee6bfb16fd813648741f8ac3")
	primaryMonitor = getMonitorConfigPrimary(monitors)
	require.Equal(t, primaryMonitor.UUID, "HDMI-1bc06f293ee6bfb16fd813648741f8ac3")

	_, err = loadConfigV5V6(configPath_v5)
	require.Nil(t, err)
}
