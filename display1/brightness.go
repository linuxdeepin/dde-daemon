// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
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
						other := monitors.GetByUuidAndName(config.UUID, config.Name)
						if other != nil && other != monitor {
							// 存在其他的名字和UUID都对应配置的显示器，不要改该配置
							continue
						}
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

func (m *Manager) initBrightness() {
	m.Brightness = make(map[string]float64)
	monitors := m.getConnectedMonitors()
	monitorsId := monitors.getMonitorsId()
	configs := m.getSuitableSysMonitorConfigs(m.DisplayMode, monitorsId, monitors)
	for _, config := range configs {
		if config.Enabled {
			m.Brightness[config.Name] = config.Brightness
		}
	}
}

func (m *Manager) getSetterConfig() int {
	// NOTE: 特殊处理龙芯笔记本亮度设置问题
	blDir := "/sys/class/backlight/loongson"
	_, err := os.Stat(blDir)
	if err == nil {
		_, err := os.Stat(filepath.Join(blDir, "device/edid"))
		if err != nil {
			return brightness.BrightnessSetterBacklight
		}
	}

	v, err := m.displayConfigMgr.Value(0, DSettingsKeyBrightnessSetter)
	if err != nil {
		logger.Warning(err)
		return brightness.BrightnessSetterAuto
	}

	return int(v.Value().(int64))
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

func (m *Manager) setMonitorBrightness(monitor *Monitor, brightnessValue float64, forceTransition bool) error {
	logger.Debug("setMonitorBrightness reality value:", brightnessValue)

	// 使用统一过渡管理器
	if m.transitionManager != nil {
		return m.transitionManager.SetBrightness(monitor.Name, brightnessValue, forceTransition)
	}

	// 降级：直接设置亮度
	setter := m.createBrightnessSetter(monitor)
	if setter == nil {
		return fmt.Errorf("failed to create brightness setter for monitor %s", monitor.Name)
	}
	return setter(brightnessValue)
}

func (m *Manager) createBrightnessSetter(monitor *Monitor) func(float64) error {
	isBuiltin := m.isBuiltinMonitor(monitor.Name)
	_uuid := monitor.uuid
	if _useWayland {
		_uuid = monitor.uuidV0
	}

	// 获取当前色温值，用于 gamma 设置路径
	temperature := m.getColorTemperatureValue()

	setter := m.getSetterConfig()

	var setterFunc func(float64) error

	switch setter {
	case brightness.BrightnessSetterBacklight:
		setterFunc = func(brightnessValue float64) error {
			return brightness.SetBacklight(brightnessValue)
		}
	case brightness.BrightnessSetterAuto:
		if isBuiltin && brightness.SupportBacklight() {
			setterFunc = func(brightnessValue float64) error {
				return brightness.SetBacklight(brightnessValue)
			}
		} else {
			setterFunc = func(brightnessValue float64) error {
				return brightness.SetOutputGama(brightnessValue, temperature, monitor.ID, m.xConn, _uuid)
			}
		}
	default: // BrightnessSetterGamma
		setterFunc = func(brightnessValue float64) error {
			return brightness.SetOutputGama(brightnessValue, temperature, monitor.ID, m.xConn, _uuid)
		}
	}

	return setterFunc
}

// setColorTemperature 设置色温（通过 gamma）
func (m *Manager) setColorTemperature(monitor *Monitor, brightnessVal float64) error {
	temperature := m.getColorTemperatureValue()
	logger.Debug("setColorTemperature", monitor.Name, temperature)

	isBuiltin := m.isBuiltinMonitor(monitor.Name)
	_uuid := monitor.uuid
	if _useWayland {
		_uuid = monitor.uuidV0
	}

	// 内建显示器使用背光时，色温通过 gamma 设置（亮度为1）
	if isBuiltin && brightness.SupportBacklight() {
		brightnessVal = 1
	}

	return brightness.SetOutputGama(brightnessVal, temperature, monitor.ID, m.xConn, _uuid)
}

func (m *Manager) setBrightness(name string, value float64) error {
	logger.Debug("Starting brightness setting", name, value)
	monitors := m.getConnectedMonitors()
	monitor := monitors.GetByName(name)
	if monitor == nil {
		logger.Debug("Monitor not found:", name)
		return InvalidOutputNameError{Name: name}
	}

	monitor.PropsMu.RLock()
	enabled := monitor.Enabled
	monitor.PropsMu.RUnlock()

	value = math.Round(value*1000) / 1000 // 通过该方法，用来对亮度值(亮度值范围为0-1)四舍五入保留小数点后三位有效数字
	if enabled {
		// 保持最小亮度，不能全黑
		if value <= 0.1 {
			value = 0.1
		} else if value > 1 {
			value = 1
		}

		err := m.setMonitorBrightness(monitor, value, false)
		if err != nil {
			logger.Warningf("failed to set brightness for %s: %v", name, err)
			return err
		}
	}

	monitor.setPropBrightnessWithLock(value)

	logger.Debug("end set brightness", name, value)

	return nil
}

func (m *Manager) setBrightnessAndSync(name string, value float64) error {
	err := m.setBrightness(name, value)
	if err == nil {
		m.syncPropBrightness()
	}
	return err
}

// setBrightnessWithTransition 使用渐变效果设置亮度（强制启用渐变）
// 亮度属性会在渐变过程中通过 onStepFunc 回调逐步更新，而非立即跳到目标值
func (m *Manager) setBrightnessWithTransition(name string, value float64) error {
	logger.Debug("Starting brightness setting with transition", name, value)
	monitors := m.getConnectedMonitors()
	monitor := monitors.GetByName(name)
	if monitor == nil {
		logger.Debug("Monitor not found:", name)
		return InvalidOutputNameError{Name: name}
	}

	monitor.PropsMu.RLock()
	enabled := monitor.Enabled
	monitor.PropsMu.RUnlock()

	value = math.Round(value*1000) / 1000
	if enabled {
		if value <= 0.1 {
			value = 0.1
		} else if value > 1 {
			value = 1
		}

		err := m.setMonitorBrightness(monitor, value, true)
		if err != nil {
			logger.Warningf("failed to set brightness for %s: %v", name, err)
			return err
		}
	}

	logger.Debug("end set brightness with transition", name, value)

	return nil
}

// getDefaultMonitorBrightness 获取默认显示器亮度（带 fallback 逻辑）
func (m *Manager) getDefaultMonitorBrightness(name string) float64 {
	if v, ok := m.Brightness[name]; ok {
		return v
	}
	if v, ok := m.Brightness["default"]; ok {
		return v
	}
	return 1
}
