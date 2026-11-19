// SPDX-FileCopyrightText: 2025 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"fmt"

	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	syspower "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.power1"
)

const (
	dsettingsAppID                   = "org.deepin.dde.daemon"
	dsettingsPowerName               = "org.deepin.dde.daemon.power"
	dsettingBatteryLidClosedAction   = "batteryLidClosedAction"
	dsettingLinePowerLidClosedAction = "linePowerLidClosedAction"
	powerActionDoNothing             = 5
)

// 初始化只能放在显示模块，为了给greeter-display复用
// greeter-display session/power未启动，因此只能用dconfig
// 合盖操作只能在display模块，greeter界面也需要合盖状态。
func (m *Manager) initLidSwitch() {
	logger.Debug("init lid switch")
	sysPower := syspower.NewPower(m.sysBus)
	sysPower.InitSignalExt(m.sysSigLoop, true)

	hasLid, err := sysPower.HasLidSwitch().Get(0)
	if err != nil {
		logger.Warningf("failed to get lid switch info: %v", err)
		return
	}

	if hasLid {
		logger.Info("has lid switch")
		m.syncLidState()
		sysPower.ConnectLidClosed(func() {
			logger.Warning("lid closed signal")
			m.holdAutoBrightness()
			m.handleLidSwitch(sysPower, func(closed bool) {
				m.setLidClosed(closed)
			})
		})
		sysPower.ConnectLidOpened(func() {
			logger.Warning("lid open signal")
			m.resumeAutoBrightness()
			m.handleLidSwitch(sysPower, func(closed bool) {
				m.setLidClosed(closed)
			})
		})
	}

}

func (m *Manager) syncLidState() bool {
	if m.sysBus == nil || m.getBuiltinMonitor() == nil {
		return false
	}
	closed, err := syspower.NewPower(m.sysBus).LidClosed().Get(0)
	if err != nil {
		logger.Warning("failed to sync lid state:", err)
		return false
	}
	return m.updateLidState(closed)
}

func (m *Manager) updateLidState(closed bool) bool {
	builtin := m.getBuiltinMonitor()
	if builtin == nil {
		return false
	}
	hasExternal := false
	for _, monitor := range m.cloneMonitorMap() {
		if monitor.ID != builtin.ID && monitor.realConnected && len(monitor.Modes) > 0 {
			hasExternal = true
			break
		}
	}
	builtin.PropsMu.Lock()
	defer builtin.PropsMu.Unlock()
	previous := builtin.lidClosed
	builtin.lidClosed = closed && hasExternal
	logger.Infof("sync lid closed: %v", builtin.lidClosed)
	return previous != builtin.lidClosed
}

func (m *Manager) handleLidSwitch(sysPower syspower.Power, handleFunc func(bool)) {
	lidClosed, err := sysPower.LidClosed().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	if !lidClosed || _greeterMode {
		handleFunc(lidClosed)
		return
	}
	hasBat, err := sysPower.OnBattery().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	key := dsettingLinePowerLidClosedAction
	if hasBat {
		key = dsettingBatteryLidClosedAction
	}
	action, err := m.getPowerLidAction(key)
	if err != nil {
		logger.Warning(err)
		return
	}
	if action == powerActionDoNothing {
		handleFunc(lidClosed)
	}
}

func (m *Manager) getPowerLidAction(key string) (int64, error) {
	dsg := configManager.NewConfigManager(m.sysBus)
	powerConfigManagerPath, err := dsg.AcquireManager(0, dsettingsAppID, dsettingsPowerName, "")
	if err != nil {
		return 0, err
	}
	dsPowerConfigManager, err := configManager.NewManager(m.sysBus, powerConfigManagerPath)
	if err != nil {
		return 0, err
	}
	data, err := dsPowerConfigManager.Value(0, key)
	if err != nil {
		return 0, err
	}
	action, ok := data.Value().(int64)
	if ok {
		return action, nil
	} else {
		return 0, fmt.Errorf("get lid action type assert failed")
	}
}

func (m *Manager) setLidClosed(closed bool) error {
	builtMonitor := m.getBuiltinMonitor()
	if builtMonitor == nil {
		logger.Warning("Lid event, but no builtin monitor found")
		return nil
	}
	logger.Infof("handle monitor:%v lid closed: %v", builtMonitor.Name, closed)
	if builtMonitor.lidClosed == closed {
		return nil
	}
	builtMonitor.lidClosed = closed
	m.updateMonitorsId(nil)
	return nil
}
