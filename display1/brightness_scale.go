// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"math"

	syspower "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.power1"
)

const minBrightness = 0.1

// initBrightnessScale 初始化亮度缩放功能：读取初始值、应用、监听变化。
func (m *Manager) initBrightnessScale() {
	if m.sysBus == nil {
		return
	}

	// 缓存 Power1 客户端，复用而非每次重建
	power := syspower.NewPower(m.sysBus)
	power.InitSignalExt(m.sysSigLoop, true)
	m.sysPower = power

	// 读取初始值
	enabled, err := power.PowerSavingModeEnabled().Get(0)
	if err != nil {
		logger.Warning("failed to get PowerSavingModeEnabled:", err)
		enabled = false
	}
	dropPercent, err := power.PowerSavingModeBrightnessDropPercent().Get(0)
	if err != nil {
		logger.Warning("failed to get PowerSavingModeBrightnessDropPercent:", err)
		dropPercent = 0
	}
	scale := calcBrightnessScale(enabled, dropPercent)
	m.brightnessScaleMu.Lock()
	m.brightnessScale = scale
	m.brightnessScaleMu.Unlock()
	logger.Infof("init brightness scale: %.4f (enabled=%v drop=%d%%)", scale, enabled, dropPercent)

	// 启动时节能可能已开启，需要立即应用
	m.applyBrightnessScale()

	// 监听属性变化
	err = power.PowerSavingModeEnabled().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}
		m.onPowerSavingModeChanged()
	})
	if err != nil {
		logger.Warning("failed to connect PowerSavingModeEnabled changed:", err)
	}

	err = power.PowerSavingModeBrightnessDropPercent().ConnectChanged(func(hasValue bool, value uint32) {
		if !hasValue {
			return
		}
		m.onPowerSavingModeChanged()
	})
	if err != nil {
		logger.Warning("failed to connect PowerSavingModeBrightnessDropPercent changed:", err)
	}
}

// calcBrightnessScale 从节能开关和降低百分比计算缩放系数。
func calcBrightnessScale(enabled bool, dropPercent uint32) float64 {
	if !enabled {
		return 1.0
	}
	drop := float64(dropPercent)
	if drop > 100 {
		drop = 100
	}
	scale := 1.0 - drop/100.0
	if scale < 0 {
		scale = 0
	}
	return scale
}

// onPowerSavingModeChanged 在节能属性变化时重新读取并应用 scale。
func (m *Manager) onPowerSavingModeChanged() {
	power := m.sysPower
	if power == nil {
		logger.Warning("sysPower not initialized")
		return
	}
	enabled, err := power.PowerSavingModeEnabled().Get(0)
	if err != nil {
		logger.Warning("failed to get PowerSavingModeEnabled:", err)
		return
	}
	dropPercent, err := power.PowerSavingModeBrightnessDropPercent().Get(0)
	if err != nil {
		logger.Warning("failed to get PowerSavingModeBrightnessDropPercent:", err)
		return
	}
	scale := calcBrightnessScale(enabled, dropPercent)
	m.setBrightnessScale(scale)
}

// getBrightnessScale 返回当前亮度缩放系数。
func (m *Manager) getBrightnessScale() float64 {
	m.brightnessScaleMu.RLock()
	defer m.brightnessScaleMu.RUnlock()
	return m.brightnessScale
}

// setBrightnessScale 更新缩放系数并重新应用亮度。
func (m *Manager) setBrightnessScale(scale float64) {
	m.brightnessScaleMu.Lock()
	old := m.brightnessScale
	m.brightnessScale = scale
	m.brightnessScaleMu.Unlock()
	if old == scale {
		return
	}
	logger.Infof("brightness scale changed: %.4f -> %.4f", old, scale)
	m.applyBrightnessScale()
}

// applyBrightnessScale 在缩放系数变化后重新应用亮度。
// 自动亮度运行时通过 transition.Update 重定向目标；否则从配置取原始值重写。
func (m *Manager) applyBrightnessScale() {
	scale := m.getBrightnessScale()

	// 自动亮度正在运行：用推荐值重新计算目标
	if m.autoBrightnessManager != nil && m.autoBrightnessManager.IsRunning() {
		m.autoBrightnessManager.applyRecommendedBrightness()
		return
	}

	// 自动亮度未运行：从配置取原始值，乘以 scale 写硬件
	monitors := m.getConnectedMonitors()
	monitorsId := monitors.getMonitorsId()
	configs := m.getSuitableSysMonitorConfigs(m.DisplayMode, monitorsId, monitors)
	for _, config := range configs {
		if config.Enabled {
			effective := scaleBrightness(config.Brightness, scale)
			err := m.setBrightness(config.Name, effective)
			if err != nil {
				logger.Warning(err)
			}
		}
	}
	m.syncPropBrightness()
}

// scaleBrightness 将原始亮度乘以缩放系数，保证最低 0.1、最高 1.0。
// 原始值 <= 0.1 时不缩放，直接返回 0.1。
func scaleBrightness(base, scale float64) float64 {
	base = math.Round(base*1000) / 1000
	if base <= minBrightness {
		return minBrightness
	}
	v := base * scale
	if v < minBrightness {
		v = minBrightness
	}
	if v > 1.0 {
		v = 1.0
	}
	return math.Round(v*1000) / 1000
}
