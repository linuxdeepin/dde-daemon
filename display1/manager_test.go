// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"fmt"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type UnitTestSuite struct {
	suite.Suite
	m *Manager
}

func (s *UnitTestSuite) SetupSuite() {
	var err error
	s.m = &Manager{}
	s.m.service, err = dbusutil.NewSessionService()
	if err != nil {
		s.T().Skip(fmt.Sprintf("failed to get service: %v", err))
	}

	s.m.sysBus, err = dbus.SystemBus()
	if err != nil {
		s.T().Skip(fmt.Sprintf("failed to get service: %v", err))
	}
}

func (s *UnitTestSuite) Test_initScreenRotation() {
	s.m.initScreenRotation()
}

func TestUnitTestSuite(t *testing.T) {
	suite.Run(t, new(UnitTestSuite))
}

func newMirrorTestManager() *Manager {
	m := &Manager{}
	m.sysConfig.Config.Screens = make(map[string]*SysScreenConfig)
	return m
}

func newMirrorTestMonitor(id uint32, name, uuid string) *Monitor {
	mode := ModeInfo{Width: 1920, Height: 1080, Rate: 60}
	return &Monitor{
		ID:        id,
		Name:      name,
		uuid:      uuid,
		Connected: true,
		BestMode:  mode,
		Modes:     []ModeInfo{mode},
	}
}

func Test_getInheritedPrimaryMonitor(t *testing.T) {
	edp := newMirrorTestMonitor(1, "eDP-1", "uuid-edp")
	hdmi := newMirrorTestMonitor(2, "HDMI-0", "uuid-hdmi")
	monitors := Monitors{edp, hdmi}
	m := newMirrorTestManager()

	m.Primary = "HDMI-0"
	assert.Equal(t, hdmi, m.getInheritedPrimaryMonitor(monitors))

	// 当前主屏未知时不作继承
	m.Primary = ""
	assert.Nil(t, m.getInheritedPrimaryMonitor(monitors))

	// randr 1.2 以下的老回退路径会把主屏名设成 Default
	m.Primary = "Default"
	assert.Nil(t, m.getInheritedPrimaryMonitor(monitors))
}

func Test_getMirrorConfigs(t *testing.T) {
	edp := newMirrorTestMonitor(1, "eDP-1", "uuid-edp")
	hdmi := newMirrorTestMonitor(2, "HDMI-0", "uuid-hdmi")
	monitors := Monitors{edp, hdmi}
	id := monitorsId{v1: "uuid-edp,uuid-hdmi"}

	// 首次生成复制配置：主屏沿用当前主屏，而不是内置屏优先的默认规则
	m := newMirrorTestManager()
	m.builtinMonitor = edp
	m.Primary = "HDMI-0"
	configs, needSaveCfg, err := m.getMirrorConfigs(id, monitors)
	assert.NoError(t, err)
	assert.True(t, needSaveCfg)
	assert.True(t, configs.getByUuid("uuid-hdmi").Primary)
	assert.False(t, configs.getByUuid("uuid-edp").Primary)

	// 没有可继承的主屏时，回落到默认规则（内置屏）
	m.Primary = ""
	configs, needSaveCfg, err = m.getMirrorConfigs(id, monitors)
	assert.NoError(t, err)
	assert.True(t, needSaveCfg)
	assert.True(t, configs.getByUuid("uuid-edp").Primary)
	assert.False(t, configs.getByUuid("uuid-hdmi").Primary)

	// 已保存的复制配置：主屏标记要刷新成当前主屏
	saveMirrorConfigs := func(primary, disabledName string) {
		configs := SysMonitorConfigs{
			{Name: "eDP-1", UUID: "uuid-edp", Enabled: true},
			{Name: "HDMI-0", UUID: "uuid-hdmi", Enabled: true},
		}
		for _, cfg := range configs {
			cfg.Primary = cfg.Name == primary
			if cfg.Name == disabledName {
				cfg.Enabled = false
			}
		}
		m.sysConfig.Config.Screens[id.v1] = &SysScreenConfig{
			Mirror: &SysMonitorModeConfig{Monitors: configs},
		}
	}

	saveMirrorConfigs("eDP-1", "")
	m.Primary = "HDMI-0"
	configs, needSaveCfg, err = m.getMirrorConfigs(id, monitors)
	assert.NoError(t, err)
	assert.True(t, needSaveCfg)
	assert.True(t, configs.getByUuid("uuid-hdmi").Primary)
	assert.False(t, configs.getByUuid("uuid-edp").Primary)

	// 主屏已经是当前主屏：不需要再存盘
	saveMirrorConfigs("HDMI-0", "")
	configs, needSaveCfg, err = m.getMirrorConfigs(id, monitors)
	assert.NoError(t, err)
	assert.False(t, needSaveCfg)
	assert.True(t, configs.getByUuid("uuid-hdmi").Primary)

	// 没有可继承的主屏：保留已保存的主屏
	saveMirrorConfigs("eDP-1", "")
	m.Primary = ""
	configs, needSaveCfg, err = m.getMirrorConfigs(id, monitors)
	assert.NoError(t, err)
	assert.False(t, needSaveCfg)
	assert.True(t, configs.getByUuid("uuid-edp").Primary)

	// 要继承的主屏在复制配置里未启用：保留已保存的主屏
	saveMirrorConfigs("eDP-1", "HDMI-0")
	m.Primary = "HDMI-0"
	configs, needSaveCfg, err = m.getMirrorConfigs(id, monitors)
	assert.NoError(t, err)
	assert.False(t, needSaveCfg)
	assert.True(t, configs.getByUuid("uuid-edp").Primary)
}
