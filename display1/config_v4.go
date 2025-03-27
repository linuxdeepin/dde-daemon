// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"encoding/json"
	"os"
)

type ConfigV4 map[string]*ScreenConfigV4

type ScreenConfigV4 struct {
	Custom  []*CustomModeConfig `json:",omitempty"`
	Mirror  *MirrorModeConfig   `json:",omitempty"`
	Extend  *ExtendModeConfig   `json:",omitempty"`
	OnlyOne *OnlyOneModeConfig  `json:",omitempty"`
	Single  *MonitorConfigV5    `json:",omitempty"`
}

type CustomModeConfig struct {
	Name     string
	Monitors []*MonitorConfigV5
}

type MirrorModeConfig struct {
	Monitors []*MonitorConfigV5
}

type ExtendModeConfig struct {
	Monitors []*MonitorConfigV5
}

type OnlyOneModeConfig struct {
	Monitors []*MonitorConfigV5
}

func loadConfigV4(filename string) (ConfigV4, error) {
	// #nosec G304
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var c ConfigV4
	err = json.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// 需要 Manager 的 Brightness，gsColorTemperatureMode, gsColorTemperatureManual
// 还会影响 Manager 的 DisplayMode
func (c ConfigV4) toConfig(m *Manager) ConfigV5 {
	newConfig := make(ConfigV5)

	for id, sc := range c {
		cfgKey := parseConfigKey(id)
		jId := cfgKey.getJoinedId()
		// 单屏幕，可设置分辨率
		if len(cfgKey.idFields) == 1 {
			//配置文件中保存的可能为空值
			if sc.Single != nil {
				//把亮度,色温写入配置文件
				sc.Single.Brightness = m.Brightness[sc.Single.Name]
				newConfig[jId] = &ScreenConfigV5{
					Mirror:  nil,
					Extend:  nil,
					OnlyOne: nil,
					Single: &SingleModeConfigV5{
						Monitor:                sc.Single,
						ColorTemperatureMode:   m.gsColorTemperatureMode,
						ColorTemperatureManual: m.gsColorTemperatureManual,
					},
				}
			}
		} else {
			screenCfg := newConfig[id]
			if screenCfg == nil {
				screenCfg = &ScreenConfigV5{}
				newConfig[id] = screenCfg
			}
			sc.toModeConfigs(screenCfg, m)
		}
	}
	return newConfig
}

func (sc *ScreenConfigV4) toModeConfigs(screenCfg *ScreenConfigV5, m *Manager) {
	if sc.OnlyOne != nil {
		result := make([]*MonitorConfigV5, 0, len(sc.OnlyOne.Monitors))
		for _, monitor := range sc.OnlyOne.Monitors {
			monitor.Brightness = m.Brightness[monitor.Name]
			result = append(result, monitor)
		}
		screenCfg.setModeConfigs(DisplayModeOnlyOne, m.gsColorTemperatureMode, m.gsColorTemperatureManual, result)
	}

	//默认自定义数据,自定义没数据就用复制 扩展的数据
	if len(sc.Custom) != 0 {
		// 直接取最后一个
		custom := sc.Custom[len(sc.Custom)-1]
		result := make([]*MonitorConfigV5, 0, len(custom.Monitors))
		for _, monitor := range custom.Monitors {
			monitor.Brightness = m.Brightness[monitor.Name]
			result = append(result, monitor)
		}

		if result[0].X == result[1].X {
			screenCfg.setModeConfigs(DisplayModeMirror, m.gsColorTemperatureMode, m.gsColorTemperatureManual, result)
			//如果升级之前是自定义模式.重新判断是拆分/合并模式
			if m.DisplayMode == DisplayModeCustom {
				m.setDisplayMode(DisplayModeMirror)
			}
		} else {
			screenCfg.setModeConfigs(DisplayModeExtend, m.gsColorTemperatureMode, m.gsColorTemperatureManual, result)
			//如果升级之前是自定义模式.重新判断是拆分/合并模式
			if m.DisplayMode == DisplayModeCustom {
				m.setDisplayMode(DisplayModeExtend)
			}
		}
		return
	} else {
		if m.DisplayMode == DisplayModeCustom {
			m.setDisplayMode(DisplayModeMirror)
		}
	}

	if sc.Mirror != nil {
		result := make([]*MonitorConfigV5, 0, len(sc.Mirror.Monitors))
		for _, monitor := range sc.Mirror.Monitors {
			monitor.Brightness = m.Brightness[monitor.Name]
			result = append(result, monitor)
		}
		screenCfg.setModeConfigs(DisplayModeMirror, m.gsColorTemperatureMode, m.gsColorTemperatureManual, result)
	}

	if sc.Extend != nil {
		result := make([]*MonitorConfigV5, 0, len(sc.Extend.Monitors))
		for _, monitor := range sc.Extend.Monitors {
			monitor.Brightness = m.Brightness[monitor.Name]
			result = append(result, monitor)
		}
		screenCfg.setModeConfigs(DisplayModeExtend, m.gsColorTemperatureMode, m.gsColorTemperatureManual, result)
	}
}
