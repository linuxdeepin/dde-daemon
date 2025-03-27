// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

type ConfigV5 map[string]*ScreenConfigV5

type FillModeConfigsV5 struct {
	FillModeMap map[string]string
}

type ScreenConfigV5 struct {
	Mirror  *ModeConfigsV5      `json:",omitempty"`
	Extend  *ModeConfigsV5      `json:",omitempty"`
	OnlyOne *ModeConfigsV5      `json:",omitempty"`
	Single  *SingleModeConfigV5 `json:",omitempty"`
}

type ModeConfigsV5 struct {
	Monitors MonitorConfigsV5
}

type MonitorConfigsV5 []*MonitorConfigV5

type SingleModeConfigV5 struct {
	// 这里其实不能用 Monitors，因为是单数
	Monitor                *MonitorConfigV5 `json:"Monitors"` // 单屏时,该配置文件中色温相关数据未生效;增加json的tag是为了兼容之前配置文件
	ColorTemperatureMode   int32
	ColorTemperatureManual int32
}

type MonitorConfigV5 struct {
	UUID        string
	Name        string
	Enabled     bool
	X           int16
	Y           int16
	Width       uint16
	Height      uint16
	Rotation    uint16
	Reflect     uint16
	RefreshRate float64
	Brightness  float64
	Primary     bool

	ColorTemperatureMode   int32
	ColorTemperatureManual int32
}

type ConfigV6 struct {
	ConfigV5 ConfigV5
	FillMode *FillModeConfigsV5
}

func (cfgV6 *ConfigV6) toSysConfigV1() (sysCfg SysConfig) {
	if cfgV6.FillMode != nil {
		sysCfg.FillModes = cfgV6.FillMode.FillModeMap
	}
	if len(cfgV6.ConfigV5) > 0 {
		sysCfg.Screens = make(map[string]*SysScreenConfig, len(cfgV6.ConfigV5))
		for monitorsId, screenCfgV5 := range cfgV6.ConfigV5 {
			sysScreenCfg := screenCfgV5.toSysScreenConfigV1()
			sysCfg.Screens[monitorsId] = sysScreenCfg
		}
	}
	return
}

func (cfgV6 *ConfigV6) toUserConfigV1() (userCfg UserConfig) {
	if len(cfgV6.ConfigV5) > 0 {
		userCfg.Screens = make(map[string]UserScreenConfig, len(cfgV6.ConfigV5))
		for monitorsId, screenCfgV5 := range cfgV6.ConfigV5 {
			userScreenCfg := screenCfgV5.toUserScreenConfigV1()
			userCfg.Screens[monitorsId] = userScreenCfg
		}
	}
	return
}

func (s *ScreenConfigV5) toUserScreenConfigV1() (userScreenCfg UserScreenConfig) {
	userScreenCfg = make(UserScreenConfig)
	if s.Mirror != nil && len(s.Mirror.Monitors) > 0 {
		userScreenCfg[KeyModeMirror] = s.Mirror.Monitors.toUserMonitorModeConfigV1()
	}

	if s.Extend != nil && len(s.Extend.Monitors) > 0 {
		userScreenCfg[KeyModeExtend] = s.Extend.Monitors.toUserMonitorModeConfigV1()
	}

	if s.OnlyOne != nil && len(s.OnlyOne.Monitors) > 0 {
		for _, monitorCfg := range s.OnlyOne.Monitors {
			mode := monitorCfg.ColorTemperatureMode
			manual := monitorCfg.ColorTemperatureManual
			userScreenCfg[KeyModeOnlyOnePrefix+monitorCfg.UUID] = &UserMonitorModeConfig{
				ColorTemperatureMode:   mode,
				ColorTemperatureManual: manual,
			}
		}
	}

	if s.Single != nil && s.Single.Monitor != nil {
		mode := s.Single.ColorTemperatureMode
		manual := s.Single.ColorTemperatureManual
		userScreenCfg[KeySingle] = &UserMonitorModeConfig{
			ColorTemperatureMode:   mode,
			ColorTemperatureManual: manual,
		}
	}
	return
}

func (s *ScreenConfigV5) toSysScreenConfigV1() (sysScreenCfg *SysScreenConfig) {
	sysScreenCfg = &SysScreenConfig{}

	if s.Mirror != nil && len(s.Mirror.Monitors) > 0 {
		sysScreenCfg.Mirror = s.Mirror.Monitors.toSysMonitorModeConfigV1()
	}

	if s.Extend != nil && len(s.Extend.Monitors) > 0 {
		sysScreenCfg.Extend = s.Extend.Monitors.toSysMonitorModeConfigV1()
	}

	if s.OnlyOne != nil && len(s.OnlyOne.Monitors) > 0 {
		enabledUuid := ""
		sysScreenCfg.OnlyOneMap = make(map[string]*SysMonitorModeConfig, len(s.OnlyOne.Monitors))
		for _, monitorCfg := range s.OnlyOne.Monitors {
			if monitorCfg.Enabled && enabledUuid == "" {
				enabledUuid = monitorCfg.UUID
			}
			mCfg := monitorCfg.toSysMonitorConfigV1()
			mCfg.Enabled = true
			mCfg.Primary = true
			sysScreenCfg.OnlyOneMap[monitorCfg.UUID] = &SysMonitorModeConfig{
				Monitors: SysMonitorConfigs{mCfg},
			}
		}
		sysScreenCfg.OnlyOneUuid = enabledUuid
	}

	if s.Single != nil && s.Single.Monitor != nil {
		sysScreenCfg.Single = &SysMonitorModeConfig{
			Monitors: SysMonitorConfigs{s.Single.Monitor.toSysMonitorConfigV1()},
		}
	}
	return
}

func (cfgs MonitorConfigsV5) toUserMonitorModeConfigV1() *UserMonitorModeConfig {
	var mode = defaultTemperatureMode
	var manual int32 = defaultTemperatureManual
	if len(cfgs) > 0 {
		mode = cfgs[0].ColorTemperatureMode
		manual = cfgs[0].ColorTemperatureManual
	}
	return &UserMonitorModeConfig{
		ColorTemperatureMode:   mode,
		ColorTemperatureManual: manual,
	}
}

func (cfgs MonitorConfigsV5) toSysMonitorModeConfigV1() *SysMonitorModeConfig {
	monitorCfgs := make(SysMonitorConfigs, 0, len(cfgs))
	for _, cfg := range cfgs {
		monitorCfgs = append(monitorCfgs, cfg.toSysMonitorConfigV1())
	}
	return &SysMonitorModeConfig{
		Monitors: monitorCfgs,
	}
}

func (v *MonitorConfigV5) toSysMonitorConfigV1() *SysMonitorConfig {
	return &SysMonitorConfig{
		UUID:        v.UUID,
		Name:        v.Name,
		Enabled:     v.Enabled,
		X:           v.X,
		Y:           v.Y,
		Width:       v.Width,
		Height:      v.Height,
		Rotation:    v.Rotation,
		Reflect:     v.Reflect,
		RefreshRate: v.RefreshRate,
		Brightness:  v.Brightness,
		Primary:     v.Primary,
	}
}

func (s *ScreenConfigV5) getModeConfigs(mode uint8) *ModeConfigsV5 {
	switch mode {
	case DisplayModeMirror:
		if s.Mirror == nil {
			s.Mirror = &ModeConfigsV5{}
		}
		return s.Mirror

	case DisplayModeExtend:
		if s.Extend == nil {
			s.Extend = &ModeConfigsV5{}
		}
		return s.Extend

	case DisplayModeOnlyOne:
		if s.OnlyOne == nil {
			s.OnlyOne = &ModeConfigsV5{}
		}
		return s.OnlyOne
	}

	return nil
}

func getMonitorConfigByUuid(configs []*MonitorConfigV5, uuid string) *MonitorConfigV5 {
	for _, mc := range configs {
		if mc.UUID == uuid {
			return mc
		}
	}
	return nil
}

func getMonitorConfigPrimary(configs []*MonitorConfigV5) *MonitorConfigV5 { //unused
	for _, mc := range configs {
		if mc.Primary {
			return mc
		}
	}
	return &MonitorConfigV5{}
}

func setMonitorConfigsPrimary(configs []*MonitorConfigV5, uuid string) {
	for _, mc := range configs {
		if mc.UUID == uuid {
			mc.Primary = true
		} else {
			mc.Primary = false
		}
	}
}

func (s *ScreenConfigV5) setMonitorConfigs(mode uint8, configs []*MonitorConfigV5) {
	switch mode {
	case DisplayModeMirror:
		if s.Mirror == nil {
			s.Mirror = &ModeConfigsV5{}
		}
		s.Mirror.Monitors = configs

	case DisplayModeExtend:
		if s.Extend == nil {
			s.Extend = &ModeConfigsV5{}
		}
		s.Extend.Monitors = configs

	case DisplayModeOnlyOne:
		s.setMonitorConfigsOnlyOne(configs)
	}
}

func (s *ScreenConfigV5) setModeConfigs(mode uint8, colorTemperatureMode int32, colorTemperatureManual int32, monitorConfig []*MonitorConfigV5) {
	s.setMonitorConfigs(mode, monitorConfig)
	cfg := s.getModeConfigs(mode)
	for _, monitorConfig := range cfg.Monitors {
		if monitorConfig.Enabled {
			monitorConfig.ColorTemperatureMode = colorTemperatureMode
			monitorConfig.ColorTemperatureManual = colorTemperatureManual
		}
	}
}

func (s *ScreenConfigV5) setMonitorConfigsOnlyOne(configs []*MonitorConfigV5) {
	if s.OnlyOne == nil {
		s.OnlyOne = &ModeConfigsV5{}
	}
	oldConfigs := s.OnlyOne.Monitors
	var newConfigs []*MonitorConfigV5
	for _, cfg := range configs {
		if !cfg.Enabled {
			oldCfg := getMonitorConfigByUuid(oldConfigs, cfg.UUID)
			if oldCfg != nil {
				// 不设置 X,Y 是因为它们总是 0
				cfg.Width = oldCfg.Width
				cfg.Height = oldCfg.Height
				cfg.RefreshRate = oldCfg.RefreshRate
				cfg.Rotation = oldCfg.Rotation
				cfg.Reflect = oldCfg.Reflect
			} else {
				continue
			}
		}
		newConfigs = append(newConfigs, cfg)
	}
	s.OnlyOne.Monitors = newConfigs
}
