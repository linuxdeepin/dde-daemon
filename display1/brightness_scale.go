// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"math"

	syspower "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.power1"
)

const minBrightness = 0.1

// initBrightnessScale 初始化亮度缩放功能：读取当前节能状态、记录缩放系数、监听变化。
//
// 采用"存显示值、切换时换算"模型：配置中保存的是屏幕实际显示的亮度值，
// 缩放仅在节能开关或降低比例变化的那一刻一次性换算，不在唤醒/刷新时反复施加。
//
// 启动时不换算已保存的显示值（缩放系数不持久化）：配置里的值就是上次关机时
// 的实际显示值，直接沿用即可，只记录当前系数供自动亮度按其缩放推荐值。
// 若启动时节能状态相对上次关机发生了变化，亮度会在下一次节能开关/比例变化时
// 重新同步。
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

	// 启动时不换算已保存的显示值；仅自动亮度需要按当前系数重新缩放推荐值。
	if m.autoBrightnessManager != nil && m.autoBrightnessManager.IsRunning() {
		m.autoBrightnessManager.applyRecommendedBrightness()
	}

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

// storeBrightnessScale 在锁内提交缩放系数。
func (m *Manager) storeBrightnessScale(newScale float64) {
	m.brightnessScaleMu.Lock()
	m.brightnessScale = newScale
	m.brightnessScaleMu.Unlock()
}

// setBrightnessScale 更新缩放系数，并把当前显示亮度从旧系数换算到新系数。
//
// 关键：m.brightnessScale 必须与配置中显示值所处的缩放基准保持一致。
// 因此系数只在硬件写入且配置落盘都成功后才提交为 newScale；若任一步失败，
// 保持旧系数不变，使下一次变化仍从正确的基准（oldScale）折算并可重试。
func (m *Manager) setBrightnessScale(newScale float64) {
	oldScale := m.getBrightnessScale()
	if oldScale == newScale {
		return
	}
	logger.Infof("brightness scale changing: %.4f -> %.4f", oldScale, newScale)
	m.applyBrightnessScale(oldScale, newScale)
}

// applyBrightnessScale 在缩放系数变化时把显示亮度从 oldScale 换算到 newScale。
// 自动亮度运行时用推荐值按新系数重新计算目标；否则把配置中保存的显示值换算后
// 写入硬件并落盘。仅在应用完全成功后才提交新系数。
func (m *Manager) applyBrightnessScale(oldScale, newScale float64) {
	// 自动亮度正在运行：推荐值是实时重算的，先提交系数再按新系数缩放推荐值
	if m.autoBrightnessManager != nil && m.autoBrightnessManager.IsRunning() {
		m.storeBrightnessScale(newScale)
		m.autoBrightnessManager.applyRecommendedBrightness()
		return
	}

	// 自动亮度未运行：把配置中保存的显示值从旧系数换算到新系数
	monitors := m.getConnectedMonitors()
	monitorsId := monitors.getMonitorsId()
	configs := m.getSuitableSysMonitorConfigs(m.DisplayMode, monitorsId, monitors)
	updates := make(map[string]float64)
	applied := true
	for _, config := range configs {
		if !config.Enabled {
			continue
		}
		newDisplayed := rescaleBrightness(config.Brightness, oldScale, newScale)
		if err := m.setBrightness(config.Name, newDisplayed); err != nil {
			// 硬件写入失败时显示值没有变化，不改写配置，也不提交新系数
			logger.Warning(err)
			applied = false
			continue
		}
		if newDisplayed != config.Brightness {
			updates[config.Name] = newDisplayed
		}
	}
	// 配置中保存的始终是屏幕实际显示值，换算后落盘，唤醒/刷新时直接恢复。
	if len(updates) > 0 {
		if err := m.saveBrightnessInCfg(updates); err != nil {
			logger.Warning(err)
			applied = false
		}
	}
	// 只有硬件写入与配置落盘都成功，才提交新系数；否则保持旧系数，
	// 保证 m.brightnessScale 与配置显示值的基准一致，下一次变化可从正确基准重试。
	if applied {
		m.storeBrightnessScale(newScale)
	} else {
		logger.Warningf("brightness scale apply incomplete, keep old scale %.4f for next retry", oldScale)
	}
	m.syncPropBrightness()
}

// rescaleBrightness 把屏幕当前显示亮度从旧缩放系数换算到新缩放系数。
// 先按旧系数还原为"逻辑亮度"（displayed/oldScale，上限 100%），再乘以新系数，
// 结果钳制到 [minBrightness, 1.0]。
func rescaleBrightness(displayed, oldScale, newScale float64) float64 {
	displayed = math.Round(displayed*1000) / 1000
	if oldScale <= 0 {
		// 旧系数为 0 时不可逆（任意逻辑值都映射到最低亮度），保持原值
		return displayed
	}
	logical := displayed / oldScale
	if logical > 1.0 {
		logical = 1.0
	}
	v := logical * newScale
	if v < minBrightness {
		v = minBrightness
	}
	if v > 1.0 {
		v = 1.0
	}
	return math.Round(v*1000) / 1000
}

// scaleBrightness 将自动亮度推荐值（逻辑值）乘以缩放系数，保证最低 minBrightness、最高 1.0。
// 仅用于自动亮度：推荐值是实时重新计算的，不作为显示基准落盘，不存在往返丢失问题。
// 原始值 <= minBrightness 时不缩放，直接返回 minBrightness。
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
