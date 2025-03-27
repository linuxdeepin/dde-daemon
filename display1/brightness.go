// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/linuxdeepin/dde-daemon/display1/brightness"
)

type InvalidOutputNameError struct {
	Name string
}

func (err InvalidOutputNameError) Error() string {
	return fmt.Sprintf("invalid output name %q", err.Name)
}

func (m *Manager) saveBrightnessInCfg(valueMap map[string]float64) error {
	if len(valueMap) == 0 {
		return nil
	}
	changed := false
	m.modifySuitableSysMonitorConfigs(func(configs SysMonitorConfigs) SysMonitorConfigs {
		for _, config := range configs {
			v, ok := valueMap[config.Name]
			if ok {
				config.Brightness = v
			} else {
				// 存在当从wayland切换到x11后，在wayland中设置过显示配置，此时配置文件中Name与切换到x11之后中的Name不匹配
				// 因此当失败时，在通过uuid查找一次，把Name改写，亮度不变
				monitors := m.getConnectedMonitors()
				for name := range valueMap {
					monitor := monitors.GetByName(name)
					if monitor == nil {
						logger.Warning("call GetByName failed: ", name)
						continue
					}

					if config.UUID == monitor.uuid {
						config.Name = name
						config.Brightness = v
					}
				}
			}
			changed = true
		}
		return configs
	})

	if !changed {
		return nil
	}

	err := m.saveSysConfig("brightness changed")
	return err
}

func (m *Manager) changeBrightness(raised bool) error {
	var step = 0.05
	if m.MaxBacklightBrightness < 100 && m.MaxBacklightBrightness != 0 {
		step = 1 / float64(m.MaxBacklightBrightness)
	}
	if !raised {
		step = -step
	}

	monitors := m.getConnectedMonitors()

	successMap := make(map[string]float64)
	for _, monitor := range monitors {
		// 如果此显示器不支持亮度调节，则退出
		if ok, err := m.CanSetBrightness(monitor.Name); !ok {
			logger.Warning("call CanSetBrightness failed: ", err)
			continue
		}

		v, ok := m.Brightness[monitor.Name]
		if !ok {
			v = 1.0
		}

		var br float64
		br = v + step
		if br > 1.0 {
			br = 1.0
		}
		if br < 0.0 {
			br = 0.0
		}
		logger.Debug("[changeBrightness] will set to:", monitor.Name, br)
		err := m.setBrightnessAndSync(monitor.Name, br)
		if err != nil {
			logger.Warning(err)
			continue
		}
		successMap[monitor.Name] = br
	}
	err := m.saveBrightnessInCfg(successMap)
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

func (m *Manager) getSavedBrightnessTable() (map[string]float64, error) {
	value := m.settings.GetString(gsKeyBrightness)
	if value == "" {
		return nil, nil
	}
	var result map[string]float64
	err := json.Unmarshal([]byte(value), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *Manager) initBrightness() {
	brightnessTable, err := m.getSavedBrightnessTable()
	if err != nil {
		logger.Warning(err)
	}
	m.Brightness = brightnessTable
}

func (m *Manager) getBrightnessSetter() string {
	// NOTE: 特殊处理龙芯笔记本亮度设置问题
	blDir := "/sys/class/backlight/loongson"
	_, err := os.Stat(blDir)
	if err == nil {
		_, err := os.Stat(filepath.Join(blDir, "device/edid"))
		if err != nil {
			return "backlight"
		}
	}

	return m.settings.GetString(gsKeySetter)
}

// see also: gnome-desktop/libgnome-desktop/gnome-rr.c
//
//	'_gnome_rr_output_name_is_builtin_display'
func (m *Manager) isBuiltinMonitor(name string) bool {
	name = strings.ToLower(name)
	switch {
	case strings.HasPrefix(name, "vga"):
		return false
	case strings.HasPrefix(name, "hdmi"):
		return false

	case strings.HasPrefix(name, "dvi"):
		return true
	case strings.HasPrefix(name, "lvds"):
		// Most drivers use an "LVDS" prefix
		return true
	case strings.HasPrefix(name, "lcd"):
		// fglrx uses "LCD" in some versions
		return true
	case strings.HasPrefix(name, "edp"):
		// eDP is for internal built-in panel connections
		return true
	case strings.HasPrefix(name, "dsi"):
		return true
	case name == "default":
		return true
	}
	return false
}

func (m *Manager) setMonitorBrightness(monitor *Monitor, brightnessValue float64, temperature int) error {
	if !isValidColorTempValue(int32(temperature)) {
		temperature = defaultTemperatureManual
	}

	isBuiltin := m.isBuiltinMonitor(monitor.Name)
	err := brightness.Set(brightnessValue, temperature, m.getBrightnessSetter(), isBuiltin,
		monitor.ID, m.xConn)
	return err
}

func (m *Manager) setBrightnessAux(fake bool, name string, value float64) error {
	monitors := m.getConnectedMonitors()
	monitor := monitors.GetByName(name)
	if monitor == nil {
		return InvalidOutputNameError{Name: name}
	}

	monitor.PropsMu.RLock()
	enabled := monitor.Enabled
	monitor.PropsMu.RUnlock()

	value = math.Round(value*1000) / 1000 // 通过该方法，用来对亮度值(亮度值范围为0-1)四舍五入保留小数点后三位有效数字
	if !fake && enabled {
		temperature := m.getColorTemperatureValue()
		// 保持最小亮度，不能全黑
		if value <= 0.1 {
			value = 0.1
		}
		err := m.setMonitorBrightness(monitor, value, temperature)
		if err != nil {
			logger.Warningf("failed to set brightness for %s: %v", name, err)
			return err
		}
	}

	monitor.setPropBrightnessWithLock(value)

	return nil
}

func (m *Manager) setBrightness(name string, value float64) error {
	return m.setBrightnessAux(false, name, value)
}

func (m *Manager) setBrightnessAndSync(name string, value float64) error {
	err := m.setBrightness(name, value)
	if err == nil {
		m.syncPropBrightness()
	}
	return err
}
