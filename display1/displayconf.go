// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	sysConfigVersion  = "1.0"
	userConfigVersion = "1.0"
)

type SysRootConfig struct {
	mu       sync.Mutex
	Version  string
	Config   SysConfig
	UpdateAt string
}

func (c *SysRootConfig) copyFrom(newSysConfig *SysRootConfig) {
	c.mu.Lock()

	c.Version = newSysConfig.Version
	c.Config = newSysConfig.Config
	c.UpdateAt = newSysConfig.UpdateAt

	c.mu.Unlock()
}

// SysConfig v1
type SysConfig struct {
	DisplayMode byte
	Screens     map[string]*SysScreenConfig
	//ScaleFactors map[string]float64 // 缩放比例, 方案变更，缩放放到dconfig中：org.deepin.XSettings的individual-scaling
	FillModes map[string]string // key 是特殊的 fillMode Key
	Cache     SysCache
}

type SysCache struct {
	BuiltinMonitor string
	ConnectTime    map[string]time.Time
}

// UserConfig v1
type UserConfig struct {
	Version string
	Screens map[string]UserScreenConfig
}

func (cfg *UserConfig) fix() {
	for _, screenConfig := range cfg.Screens {
		screenConfig.fix()
	}
}

type UserScreenConfig map[string]*UserMonitorModeConfig

const (
	KeyModeMirror        = "Mirror"
	KeyModeExtend        = "Extend"
	KeyModeOnlyOnePrefix = "OnlyOne-"
	KeySingle            = "Single"
)

type UserMonitorModeConfig struct {
	ColorTemperatureMode   int32
	ColorTemperatureManual int32
	ColorTemperatureModeOn int32 //记录用户开启色温时的模式
	// 以后如果有必要
	// Monitors UserMonitorConfigs
}

func (c *UserMonitorModeConfig) fix() {
	if !isValidColorTempMode(c.ColorTemperatureMode) {
		c.ColorTemperatureMode = defaultTemperatureMode
	}
	if !isValidColorTempValue(c.ColorTemperatureManual) {
		c.ColorTemperatureManual = defaultTemperatureManual
	}
	if c.ColorTemperatureMode != ColorTemperatureModeNone {
		c.ColorTemperatureModeOn = c.ColorTemperatureMode
	}
}

func (c *UserMonitorModeConfig) clone() *UserMonitorModeConfig {
	if c == nil {
		return nil
	}
	cfgCp := *c
	return &cfgCp
}

func getDefaultUserMonitorModeConfig() *UserMonitorModeConfig {
	return &UserMonitorModeConfig{
		ColorTemperatureMode:   defaultTemperatureMode,
		ColorTemperatureManual: defaultTemperatureManual,
		// TODO: 如果配置缺失，需要一个色温使能的模式
		ColorTemperatureModeOn: ColorTemperatureModeAuto,
	}
}

func (m *Manager) getUserScreenConfig(monitorsId monitorsId) UserScreenConfig {
	m.userCfgMu.Lock()
	defer m.userCfgMu.Unlock()

	screens := m.userConfig.Screens
	screenCfg := screens[monitorsId.v1]
	if screenCfg != nil {
		return screenCfg.clone()
	}

	return UserScreenConfig{}
}

func (m *Manager) setUserScreenConfig(monitorsId monitorsId, screenCfg UserScreenConfig) {
	m.userCfgMu.Lock()
	defer m.userCfgMu.Unlock()

	screens := m.userConfig.Screens
	if screens == nil {
		screens = make(map[string]UserScreenConfig)
		m.userConfig.Screens = screens
	}
	screens[monitorsId.v1] = screenCfg.clone()
}

// getSysScreenConfig 根据 monitorsId 参数返回不同的屏幕配置，不同 monitorsId 则屏幕配置不同。
// monitorsId 代表了已连接了哪些显示器。
func (m *Manager) getSysScreenConfig(monitorsId monitorsId) *SysScreenConfig {
	m.sysConfig.mu.Lock()
	defer m.sysConfig.mu.Unlock()

	screens := m.sysConfig.Config.Screens
	screenCfg := screens[monitorsId.v1]
	if screenCfg != nil {
		return screenCfg.clone()
	}

	return &SysScreenConfig{}
}

func (m *Manager) updateConfigUuid(monitors Monitors) {
	m.updateSysConfigUuid(monitors)
	m.updateUserConfigUuid(monitors)
}

func (m *Manager) updateUserConfigUuid(monitors Monitors) {
	m.userCfgMu.Lock()
	defer m.userCfgMu.Unlock()

	needSave := m.userConfig.updateUuid(monitors)
	if needSave {
		err := m.saveUserConfigNoLock()
		if err != nil {
			logger.Warning("save user config failed:", err)
		}
	}
}

func (cfg *UserConfig) updateUuid(monitors Monitors) (changed bool) {
	screens := cfg.Screens
	// screensAdditional 待插入 screens 的键值
	var screensAdditional map[string]UserScreenConfig
	for monitorsId, screenCfg := range screens {
		monitorUuids := strings.Split(monitorsId, monitorsIdDelimiter)

		var partMonitors Monitors
		for _, uuid := range monitorUuids {
			monitor := monitors.GetByUuid(uuid)
			if monitor != nil {
				partMonitors = append(partMonitors, monitor)
			}
		}

		// 如果 monitorsId 中所有显示器的 uuid 都能在 monitors 中找到，
		// 则可以进行对 screens 中这个 monitorsId 和 screenCfg 的替换。
		if len(monitorUuids) == len(partMonitors) {
			newMonitorsId := partMonitors.getMonitorsId()
			screenCfgChanged := false
			screenCfg, screenCfgChanged = screenCfg.updateUuid(monitors)
			if newMonitorsId.v1 != monitorsId || screenCfgChanged {
				if screensAdditional == nil {
					screensAdditional = make(map[string]UserScreenConfig)
				}
				screensAdditional[newMonitorsId.v1] = screenCfg
				delete(screens, monitorsId)
				changed = true
			}
		}
	}

	for key, screenCfg := range screensAdditional {
		screens[key] = screenCfg
	}
	return
}

func (m *Manager) updateSysConfigUuid(monitors Monitors) {
	m.sysConfig.mu.Lock()
	defer m.sysConfig.mu.Unlock()

	needSave := m.sysConfig.Config.updateUuid(monitors)
	if needSave {
		err := m.saveSysConfigNoLock("update uuid")
		if err != nil {
			logger.Warning("save sys config failed:", err)
		}
	}
}

func (m *Manager) setSysScreenConfig(monitorsId monitorsId, screenCfg *SysScreenConfig) {
	m.sysConfig.mu.Lock()
	defer m.sysConfig.mu.Unlock()

	screens := m.sysConfig.Config.Screens
	if screens == nil {
		screens = make(map[string]*SysScreenConfig)
		m.sysConfig.Config.Screens = screens
	}
	screens[monitorsId.v1] = screenCfg.clone()
}

func (cfg *SysConfig) getMonitorConfigs(monitorsId monitorsId, displayMode byte, single bool) SysMonitorConfigs {
	if cfg == nil {
		return nil
	}
	screens := cfg.Screens
	sc := screens[monitorsId.v1]
	if sc == nil {
		return nil
	}

	if single {
		return sc.getSingleMonitorConfigs()
	}
	return sc.getMonitorConfigs(displayMode, sc.OnlyOneUuid)
}

func (cfg *SysConfig) updateUuid(monitors Monitors) (changed bool) {
	// 更新 screens 中的 uuid
	screens := cfg.Screens
	// screensAdditional 待插入 screens 的键值
	var screensAdditional map[string]*SysScreenConfig
	for monitorsId, screenCfg := range screens {
		monitorUuids := strings.Split(monitorsId, monitorsIdDelimiter)

		var partMonitors Monitors
		for _, uuid := range monitorUuids {
			monitor := monitors.GetByUuid(uuid)
			if monitor != nil {
				partMonitors = append(partMonitors, monitor)
			}
		}

		// 如果 monitorsId 中所有显示器的 uuid 都能在 monitors 中找到，
		// 则可以进行对 screens 中这个 monitorsId 和 screenCfg 的替换。
		if len(monitorUuids) == len(partMonitors) {
			newMonitorsId := partMonitors.getMonitorsId()
			screenCfgChanged := false
			screenCfg, screenCfgChanged = screenCfg.updateUuid(monitors)
			if newMonitorsId.v1 != monitorsId || screenCfgChanged {
				if screensAdditional == nil {
					screensAdditional = make(map[string]*SysScreenConfig)
				}
				screensAdditional[newMonitorsId.v1] = screenCfg
				delete(screens, monitorsId)
				changed = true
			}
		}
	}

	for key, screenCfg := range screensAdditional {
		screens[key] = screenCfg
	}

	// 更新 fillModes 中的 uuid
	fillModes := cfg.FillModes
	var fillModesAdditional map[string]string

	for key, value := range fillModes {
		// key 的格式是 uuid:(width)x(height)
		fields := strings.SplitN(key, fillModeKeyDelimiter, 2)
		if len(fields) != 2 {
			// 删除无效的键
			changed = true
			delete(fillModes, key)
			continue
		}
		uuid := fields[0]
		uuidChanged := false
		uuid, uuidChanged = updateUuid(uuid, monitors)
		if uuidChanged {
			if fillModesAdditional == nil {
				fillModesAdditional = make(map[string]string)
			}
			newKey := uuid + fillModeKeyDelimiter + fields[1]
			fillModesAdditional[newKey] = value
			delete(fillModes, key)
			changed = true
		}
	}

	for key, value := range fillModesAdditional {
		fillModes[key] = value
	}
	return
}

func (usc UserScreenConfig) getMonitorModeConfig(mode byte, uuid string) (cfg *UserMonitorModeConfig) {
	switch mode {
	case DisplayModeMirror:
		return usc[KeyModeMirror]
	case DisplayModeExtend:
		return usc[KeyModeExtend]
	case DisplayModeOnlyOne:
		return usc[KeyModeOnlyOnePrefix+uuid]
	}
	return nil
}

func (usc UserScreenConfig) setMonitorModeConfig(mode byte, uuid string, cfg *UserMonitorModeConfig) {
	switch mode {
	case DisplayModeMirror:
		usc[KeyModeMirror] = cfg
	case DisplayModeExtend:
		usc[KeyModeExtend] = cfg
	case DisplayModeOnlyOne:
		usc[KeyModeOnlyOnePrefix+uuid] = cfg
	}
}

func (usc UserScreenConfig) fix() {
	for _, config := range usc {
		config.fix()
	}
}

func (usc UserScreenConfig) clone() UserScreenConfig {
	if usc == nil {
		return nil
	}
	result := make(UserScreenConfig, len(usc))
	for key, config := range usc {
		result[key] = config.clone()
	}
	return result
}

func (usc UserScreenConfig) updateUuid(monitors Monitors) (outCfg UserScreenConfig, changed bool) {
	if usc == nil {
		return nil, false
	}
	outCfg = make(UserScreenConfig, len(usc))
	for key, config := range usc {
		if strings.HasPrefix(key, KeyModeOnlyOnePrefix) {
			uuid := key[len(KeyModeOnlyOnePrefix):]
			uuidChanged := false
			uuid, uuidChanged = updateUuid(uuid, monitors)
			changed = changed || uuidChanged
			key = KeyModeOnlyOnePrefix + uuid
		}
		outCfg[key] = config
	}
	if !changed {
		outCfg = usc
	}
	return
}

// SysScreenConfig 系统级屏幕配置
// NOTE: Single 可以看作是特殊的显示模式，和 Mirror,Extend 等模式共用 SysMonitorModeConfig 结构可以保持设计上的统一，不必在乎里面有 Monitors
type SysScreenConfig struct {
	Mirror      *SysMonitorModeConfig            `json:",omitempty"`
	Extend      *SysMonitorModeConfig            `json:",omitempty"`
	Single      *SysMonitorModeConfig            `json:",omitempty"`
	OnlyOneMap  map[string]*SysMonitorModeConfig `json:",omitempty"`
	OnlyOneUuid string                           `json:",omitempty"`
}

func isCurrentVersionUuid(uuid string) bool {
	return strings.HasSuffix(uuid, "|v1")
}

func (c *SysScreenConfig) updateUuid(monitors Monitors) (result *SysScreenConfig, overallChanged bool) {
	if c == nil {
		return nil, false
	}

	result = &SysScreenConfig{}

	if c.Mirror != nil {
		changed := false
		result.Mirror, changed = c.Mirror.updateUuid(monitors)
		overallChanged = overallChanged || changed
	}
	if c.Extend != nil {
		changed := false
		result.Extend, changed = c.Extend.updateUuid(monitors)
		overallChanged = overallChanged || changed
	}
	if c.Single != nil {
		changed := false
		result.Single, changed = c.Single.updateUuid(monitors)
		overallChanged = overallChanged || changed
	}
	if len(c.OnlyOneMap) > 0 {
		result.OnlyOneMap = make(map[string]*SysMonitorModeConfig, len(c.OnlyOneMap))
		for uuid, config := range c.OnlyOneMap {
			changed := false
			uuid, changed = updateUuid(uuid, monitors)
			overallChanged = overallChanged || changed

			result.OnlyOneMap[uuid], changed = config.updateUuid(monitors)
			overallChanged = overallChanged || changed
		}
	}

	changed := false
	result.OnlyOneUuid, changed = updateUuid(c.OnlyOneUuid, monitors)
	overallChanged = overallChanged || changed

	return result, overallChanged
}

func updateUuid(uuid string, monitors Monitors) (newUuid string, change bool) {
	if isCurrentVersionUuid(uuid) {
		return uuid, false
	}
	monitor := monitors.GetByUuid(uuid)
	if monitor != nil {
		return monitor.uuid, true
	}
	// 放弃修改
	return uuid, false
}

// 更新配置中的所有 uuid 的版本
func (c *SysMonitorModeConfig) updateUuid(monitors Monitors) (*SysMonitorModeConfig, bool) {
	if c == nil {
		return nil, false
	}
	monitorConfigs := c.Monitors
	hasChanged := false
	outMonitorCfgs := make(SysMonitorConfigs, 0, len(monitorConfigs))
	for _, monitorCfg := range monitorConfigs {
		if !isCurrentVersionUuid(monitorCfg.UUID) {
			monitor := monitors.GetByUuid(monitorCfg.UUID)
			if monitor != nil {
				outMonitorCfg := *monitorCfg
				// 更新 uuid
				outMonitorCfg.UUID = monitor.uuid
				hasChanged = true
				outMonitorCfgs = append(outMonitorCfgs, &outMonitorCfg)
			}
		} else {
			outMonitorCfgs = append(outMonitorCfgs, monitorCfg)
		}
	}
	result := SysMonitorModeConfig{}
	if hasChanged && len(outMonitorCfgs) == len(monitorConfigs) {
		result.Monitors = outMonitorCfgs
	} else {
		// 放弃更新 uuid
		result.Monitors = monitorConfigs
	}
	return &result, hasChanged
}

func (c *SysScreenConfig) clone() *SysScreenConfig {
	if c == nil {
		return nil
	}
	result := &SysScreenConfig{
		Mirror:      c.Mirror.clone(),
		Extend:      c.Extend.clone(),
		Single:      c.Single.clone(),
		OnlyOneUuid: c.OnlyOneUuid,
	}
	if len(c.OnlyOneMap) > 0 {
		result.OnlyOneMap = make(map[string]*SysMonitorModeConfig, len(c.OnlyOneMap))
		for uuid, config := range c.OnlyOneMap {
			result.OnlyOneMap[uuid] = config.clone()
		}
	}
	return result
}

func (c *SysScreenConfig) fix() {
	if c.Mirror != nil {
		c.Mirror.fix()
	}
	if c.Extend != nil {
		c.Extend.fix()
	}
	if c.Single != nil {
		c.Single.fix()
	}
	for uuid, config := range c.OnlyOneMap {
		if config != nil {
			config.fix()
		} else {
			delete(c.OnlyOneMap, uuid)
		}
	}
}

type SysMonitorModeConfig struct {
	Monitors SysMonitorConfigs
}

func (c *SysMonitorModeConfig) fix() {
	for _, monitor := range c.Monitors {
		monitor.fix()
	}
}

func (c *SysMonitorModeConfig) clone() *SysMonitorModeConfig {
	if c == nil {
		return nil
	}
	return &SysMonitorModeConfig{
		Monitors: c.Monitors.clone(),
	}
}

type SysMonitorConfig struct {
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
}

func (c *SysMonitorConfig) fix() {
	// c.Enable 为 false，但 c.Primary 为 true 的，错误情况
	if !c.Enabled && c.Primary {
		c.Primary = false
	}
	if !isValidBrightness(c.Brightness) {
		c.Brightness = 1
	}
}

func (c *SysMonitorConfig) modify(changes monitorChanges) {
	for name, value := range changes {
		var ok bool
		switch name {
		case monitorPropX:
			c.X, ok = value.(int16)
		case monitorPropY:
			c.Y, ok = value.(int16)
		case monitorPropWidth:
			c.Width, ok = value.(uint16)
		case monitorPropHeight:
			c.Height, ok = value.(uint16)
		case monitorPropRotation:
			c.Rotation, ok = value.(uint16)
		case monitorPropRefreshRate:
			c.RefreshRate, ok = value.(float64)
		case monitorPropEnabled:
			c.Enabled, ok = value.(bool)
		default:
			ok = true
			logger.Warningf("invalid monitor property name, uuid: %v, name: %v", c.UUID, name)
		}

		if !ok {
			logger.Warningf("failed to set value, uuid: %v, name: %v, value: %v", c.UUID, name, value)
		} else {
			logger.Debugf("config %v set value %v = %v", c.UUID, name, value)
		}
	}
}

func isValidBrightness(value float64) bool {
	// 不含 0
	return value > 0 && value <= 1
}

type SysMonitorConfigs []*SysMonitorConfig

func (cfgs SysMonitorConfigs) clone() SysMonitorConfigs {
	if cfgs == nil {
		return nil
	}
	result := make(SysMonitorConfigs, len(cfgs))
	for i, config := range cfgs {
		configCp := *config
		result[i] = &configCp
	}
	return result
}

func (s *SysScreenConfig) getSingleMonitorConfigs() SysMonitorConfigs {
	if s == nil || s.Single == nil {
		return nil
	}
	return s.Single.Monitors
}

func (s *SysScreenConfig) getMonitorConfigs(mode uint8, uuid string) SysMonitorConfigs {
	if s == nil {
		return nil
	}
	switch mode {
	case DisplayModeMirror:
		if s.Mirror == nil {
			return nil
		}
		return s.Mirror.Monitors

	case DisplayModeExtend:
		if s.Extend == nil {
			return nil
		}
		return s.Extend.Monitors

	case DisplayModeOnlyOne:
		if uuid == "" {
			return nil
		}
		config := s.OnlyOneMap[uuid]
		if config != nil {
			return config.Monitors
		}
	}

	return nil
}

func (s *SysScreenConfig) setSingleMonitorConfigs(configs SysMonitorConfigs) {
	if s.Single == nil {
		s.Single = &SysMonitorModeConfig{}
	}
	s.Single.Monitors = configs
}

func (s *SysScreenConfig) setMonitorConfigs(mode uint8, uuid string, configs SysMonitorConfigs) {
	switch mode {
	case DisplayModeMirror:
		if s.Mirror == nil {
			s.Mirror = &SysMonitorModeConfig{}
		}
		s.Mirror.Monitors = configs

	case DisplayModeExtend:
		if s.Extend == nil {
			s.Extend = &SysMonitorModeConfig{}
		}
		s.Extend.Monitors = configs

	case DisplayModeOnlyOne:
		s.setMonitorConfigsOnlyOne(uuid, configs)
	}
}

func (s *SysScreenConfig) setMonitorConfigsOnlyOne(uuid string, configs SysMonitorConfigs) {
	if uuid == "" {
		return
	}
	if s.OnlyOneMap == nil {
		s.OnlyOneMap = make(map[string]*SysMonitorModeConfig)
	}

	if s.OnlyOneMap[uuid] == nil {
		s.OnlyOneMap[uuid] = &SysMonitorModeConfig{}
	}

	if len(configs) > 1 {
		// 去除非使能的 monitor
		var tmpCfg *SysMonitorConfig
		for _, config := range configs {
			if config.Enabled {
				tmpCfg = config
				break
			}
		}
		if tmpCfg != nil {
			configs = SysMonitorConfigs{tmpCfg}
		}
	}

	s.OnlyOneMap[uuid].Monitors = configs
}

func (cfgs SysMonitorConfigs) getByUuid(uuid string) *SysMonitorConfig {
	for _, mc := range cfgs {
		if uuid == mc.UUID {
			return mc
		}
	}
	return nil
}

func (cfgs SysMonitorConfigs) setPrimary(uuid string) {
	for _, mc := range cfgs {
		if mc.UUID == uuid {
			mc.Primary = true
		} else {
			mc.Primary = false
		}
	}
}

// cfgs 和 otherCfgs 之间是否仅亮度不同
// 前置条件：cfgs 和 otherCfgs 不相同
func (cfgs SysMonitorConfigs) onlyBrNotEq(otherCfgs SysMonitorConfigs) bool {
	if len(cfgs) != len(otherCfgs) {
		return false
	}
	// 除了亮度设置为 0， 其他字段都复制
	partCpCfgs := func(cfgs SysMonitorConfigs) SysMonitorConfigs {
		copyCfgs := make(SysMonitorConfigs, len(cfgs))
		for i, cfg := range cfgs {
			cpCfg := &SysMonitorConfig{}
			*cpCfg = *cfg
			cpCfg.Brightness = 0
			copyCfgs[i] = cpCfg
		}
		return copyCfgs
	}

	c1 := partCpCfgs(cfgs)
	c2 := partCpCfgs(otherCfgs)
	// 把亮度都安全的设置为0, 如果 c1 和 c2 是相同的，则可以说明是仅亮度不同。
	if reflect.DeepEqual(c1, c2) {
		return true
	}
	return false
}

func (cfgs SysMonitorConfigs) sort() {
	sort.Slice(cfgs, func(i, j int) bool {
		return strings.Compare(cfgs[i].UUID, cfgs[j].UUID) < 0
	})
}
