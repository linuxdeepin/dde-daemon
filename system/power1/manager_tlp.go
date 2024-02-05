// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"strings"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
)


const lowBatteryThreshold = 20.0

func modifyLMTConfig(lines []string, dict map[string]string) ([]string, bool) {
	var changed bool
	for idx := range lines {
		line := lines[idx]
		for key, value := range dict {
			if strings.HasPrefix(line, key) {
				newLine := key + "=" + value
				if line != newLine {
					changed = true
					lines[idx] = newLine
				}
				delete(dict, key)
			}
		}
		if len(dict) == 0 {
			break
		}
	}
	if len(dict) > 0 {
		for key, value := range dict {
			newLine := key + "=" + value
			lines = append(lines, newLine)
		}
		changed = true
	}
	return lines, changed
}


func (m *Manager) writePowerSavingModeEnabledCb(write *dbusutil.PropertyWrite) *dbus.Error {
	logger.Debug("set laptop mode enabled", write.Value)

	m.PropsMu.Lock()
	m.setPropPowerSavingModeAuto(false)
	m.setPropPowerSavingModeAutoWhenBatteryLow(false)
	m.PropsMu.Unlock()


	// TODO: add tlp part


	return nil
}

func (m *Manager) updatePowerSavingMode() { // 根据用户设置以及当前状态,修改节能模式
	if !m.initDone {
		// 初始化未完成时，暂不提供功能
		return
	}
	var enable bool
	var err error
	if !m.IsPowerSaveSupported {
		enable = false
		logger.Debug("IsPowerSaveSupported is false.")
	} else if m.PowerSavingModeAuto && m.PowerSavingModeAutoWhenBatteryLow {
		if m.OnBattery || m.batteryLow {
			enable = true
		} else {
			enable = false
		}
	} else if m.PowerSavingModeAuto && !m.PowerSavingModeAutoWhenBatteryLow {
		if m.OnBattery {
			enable = true
		} else {
			enable = false
		}
	} else if !m.PowerSavingModeAuto && m.PowerSavingModeAutoWhenBatteryLow {
		if m.batteryLow {
			enable = true
		} else {
			enable = false
		}
	} else {
		return // 未开启两个自动节能开关
	}

	if enable {
		logger.Debug("auto switch to powersave mode")
		err = m.doSetMode("powersave")
	} else {
		if m.IsBalanceSupported {
			logger.Debug("auto switch to balance mode")
			err = m.doSetMode("balance")
		}
	}

	if err != nil {
		logger.Warning(err)
	}

	logger.Info("updatePowerSavingMode PowerSavingModeEnabled: ", enable)
	m.PropsMu.Lock()
	m.setPropPowerSavingModeEnabled(enable)
	m.PropsMu.Unlock()
	
	// TODO: add tlp mode part
}
