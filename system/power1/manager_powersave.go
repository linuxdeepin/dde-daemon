// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/common/systemdunit"
)

type DSPCMode string

const (
	DSPCPerformance DSPCMode = "performance"
	DSPCBalance     DSPCMode = "balance"
	DSPCSaving      DSPCMode = "saving"
	DSPCLowBattery  DSPCMode = "lowbat"
)

type powerConfig struct {
	DSPCConfig DSPCMode

	PowerSavingModeEnabled bool
}

// TODO dconfig 或其他配置存储
var _powerConfigMap = map[string]*powerConfig{
	ddePerformance: {
		DSPCConfig:             DSPCPerformance,
		PowerSavingModeEnabled: false,
	},
	ddeBalance: {
		DSPCConfig:             DSPCBalance,
		PowerSavingModeEnabled: false,
	},
	ddePowerSave: {
		DSPCConfig:             DSPCSaving,
		PowerSavingModeEnabled: true,
	},
	ddeLowBattery: {
		DSPCConfig:             DSPCLowBattery,
		PowerSavingModeEnabled: true,
	},
}

func (m *Manager) setDSPCState(state DSPCMode) {
	conn, err := dbus.SystemBus()
	if err != nil {
		logger.Warning("Failed to connect to system bus:", err)
		return
	}

	unitName := "dde-system-power-control.service"
	unitInfo := systemdunit.TransientUnit{
		Dbus:        conn,
		UnitName:    unitName,
		Type:        "oneshot",
		Description: "Transient Unit set deepin system power control",
		Environment: []string{},
		Commands:    []string{"/usr/sbin/deepin-system-power-control", "set", string(state)},
	}
	err = unitInfo.StartTransientUnit()
	if err != nil {
		logger.Warningf("failed create unit: %v, err: %v", unitName, err)
		return
	}
	if !unitInfo.WaitforFinish(m.systemSigLoop) {
		logger.Warningf("%v run failed", unitName)
		return
	}
}

// 关联电量、电源连接状态、低电量节能开关、使用电池节能开关四项状态的变动，修改系统的功耗模式
func (m *Manager) updatePowerMode(init bool) {
	logger.Info("start updatePowerMode")
	if !m.initDone {
		// 初始化未完成时，暂不提供功能
		return
	}
	var enablePowerSave bool
	var enableLowPower bool
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()
	if m.PowerSavingModeAuto && m.OnBattery {
		enablePowerSave = true
	}

	if m.PowerSavingModeAutoWhenBatteryLow && m.batteryLow {
		enableLowPower = true
	}
	logger.Infof("PowerSavingModeAuto: %v\n OnBattery:%v \n PowerSavingModeAutoWhenBatteryLow:%v \n batteryLow:%v \n",
		m.PowerSavingModeAuto, m.OnBattery, m.PowerSavingModeAutoWhenBatteryLow, m.batteryLow)
	logger.Infof("lastMode: %v", m.lastMode)
	if !m.PowerSavingModeAuto && !m.PowerSavingModeAutoWhenBatteryLow && !init {
		return
	}
	mode := m.lastMode
	if init {
		mode = m.Mode
	}
	if enablePowerSave {
		mode = ddePowerSave
	}
	if enableLowPower {
		mode = ddeLowBattery
	}
	m.doSetMode(mode)
}

func (m *Manager) updatePowerSavingState(state bool) {
	if m.setPropPowerSavingModeAutoWhenBatteryLow(state) {
		_ = m.setDsgData(dsettingsPowerSavingModeAutoWhenBatteryLow, state, m.dsgPower)
	}

	if m.setPropPowerSavingModeAuto(state) {
		_ = m.setDsgData(dsettingsPowerSavingModeAuto, state, m.dsgPower)
	}
}
