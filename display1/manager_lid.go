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
	hasLid, err := sysPower.HasLidSwitch().Get(0)
	if err != nil {
		logger.Warningf("failed to get lid switch info: %v", err)
		return
	}

	if hasLid {
		logger.Info("has lid switch")
		m.handleLidSwitch(sysPower, func(closed bool) {
			if m.builtinMonitor != nil {
				logger.Infof("init lid closed: %v", closed)
				m.builtinMonitor.lidClosed = closed
			}
		})
		sysPower.InitSignalExt(m.sysSigLoop, true)
		sysPower.ConnectLidClosed(func() {
			logger.Warning("lid closed signal")
			m.handleLidSwitch(sysPower, func(closed bool) {
				m.setLidClosed(closed)
			})
		})
		sysPower.ConnectLidOpened(func() {
			logger.Warning("lid open signal")
			m.handleLidSwitch(sysPower, func(closed bool) {
				m.setLidClosed(closed)
			})
		})
	}

}

func (m *Manager) handleLidSwitch(sysPower syspower.Power, handleFunc func(bool)) {
	hasBat, err := sysPower.OnBattery().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	lidClosed, err := sysPower.LidClosed().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	// 如果是开盖，不管合盖配置是什么动作，都需要执行开盖操作
	if !lidClosed {
		handleFunc(lidClosed)
	} else {
		var key string
		if hasBat {
			key = dsettingBatteryLidClosedAction
		} else {
			key = dsettingLinePowerLidClosedAction
		}
		action, err := m.getPowerLidAction(key)
		if err != nil {
			logger.Warning(err)
			return
		}
		if action == powerActionDoNothing || _greeterMode {
			handleFunc(lidClosed)
		}
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
