// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"time"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/soundutils"
	. "github.com/linuxdeepin/go-lib/gettext"
)

// nolint
const (
	suspendStateUnknown = iota + 1
	suspendStateLidOpen
	suspendStateFinish
	suspendStateWakeup
	suspendStatePrepare
	suspendStateLidClose
	suspendStateButtonClick
)

func (m *Manager) setPrepareSuspend(v int) {
	m.prepareSuspendLocker.Lock()
	m.prepareSuspend = v
	m.prepareSuspendLocker.Unlock()
}

func (m *Manager) shouldIgnoreIdleOn() bool {
	m.prepareSuspendLocker.Lock()
	v := (m.prepareSuspend > suspendStateFinish)
	m.prepareSuspendLocker.Unlock()
	return v
}

func (m *Manager) shouldIgnoreIdleOff() bool {
	m.prepareSuspendLocker.Lock()
	v := (m.prepareSuspend >= suspendStatePrepare)
	m.prepareSuspendLocker.Unlock()
	return v
}

// 处理有线电源插入拔出事件
func (m *Manager) initOnBatteryChangedHandler() {
	power := m.helper.Power
	err := power.OnBattery().ConnectChanged(func(hasValue bool, onBattery bool) {
		if !hasValue {
			return
		}
		logger.Debug("property OnBattery changed to", onBattery)
		m.PropsMu.Lock()
		changed := m.setPropOnBattery(onBattery)
		state := m.lidSwitchState
		m.PropsMu.Unlock()

		if changed {
			if onBattery {
				playSound(soundutils.EventPowerUnplug)
				if state == lidSwitchStateClose {
					m.doLidClosedAction(m.BatteryLidClosedAction.Get())
				}
			} else {
				playSound(soundutils.EventPowerPlug)
				if state == lidSwitchStateClose {
					m.doLidClosedAction(m.LinePowerLidClosedAction.Get())
				}
			}
		}
	})

	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) handleBeforeSuspend() {
	m.setPrepareSuspend(suspendStatePrepare)
	m.setDPMSModeOff()
	logger.Debug("before sleep")
}

func (m *Manager) handleWakeup() {
	m.setPrepareSuspend(suspendStateWakeup)
	logger.Debug("wakeup")

	// Fix wayland sometimes no dpms event after wakeup
	if m.UseWayland {
		err := m.service.Conn().Object("com.deepin.daemon.KWayland",
			"/com/deepin/daemon/KWayland/Output").Call("com.deepin.daemon.KWayland.Idle.simulateUserActivity", 0).Err

		if err != nil {
			logger.Warning(err)
		}
	}

	if v := m.submodules[submodulePSP]; v != nil {
		if psp := v.(*powerSavePlan); psp != nil {
			psp.HandleIdleOff()
		}
	}

	m.setDPMSModeOn()
	err := m.display.RefreshBrightness(0)
	if err != nil {
		logger.Warning(err)
	}

	ch := make(chan *dbus.Call, 1)
	m.helper.Power.GoRefreshBatteries(0, ch)
	go func() {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			return
		}
	}()

	playSound(soundutils.EventWakeup)
}

func (m *Manager) handleRefreshMains() {
	logger.Debug("handleRefreshMains")
	power := m.helper.Power
	err := power.RefreshMains(0)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) handleBatteryDisplayUpdate() {
	logger.Debug("handleBatteryDisplayUpdate")
	power := m.helper.Power
	hasBattery, err := power.HasBattery().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	m.PropsMu.Lock()
	var warnLevelChanged bool
	var warnLevel WarnLevel

	if hasBattery {
		m.setPropBatteryIsPresent(true)

		percentage, err := power.BatteryPercentage().Get(0)
		if err != nil {
			logger.Warning(err)
		}
		m.setPropBatteryPercentage(percentage)

		timeToEmpty, err := power.BatteryTimeToEmpty().Get(0)
		if err != nil {
			logger.Warning(err)
		}

		status, err := power.BatteryStatus().Get(0)
		if err != nil {
			logger.Warning(err)
		}
		m.setPropBatteryState(status)

		warnLevel = m.getWarnLevel(percentage, timeToEmpty)
		// 当电池电量从1%->0%时，如果之前的warnLevel不为WarnLevelNone（电池的状态正常），此时不去做warnLevel改变处理
		// 防止某些机器在电量变为0时低电量屏保被取消，进入锁屏界面
		if m.warnLevelConfig.getWarnLevelConfig().UsePercentageForPolicy && percentage == 0.0 && m.WarnLevel != WarnLevelNone && m.OnBattery {
			warnLevelChanged = false
		} else {
			warnLevelChanged = m.setPropWarnLevel(warnLevel)
		}
	} else {
		warnLevel = WarnLevelNone
		warnLevelChanged = m.setPropWarnLevel(WarnLevelNone)
		delete(m.BatteryIsPresent, batteryDisplay)
		delete(m.BatteryPercentage, batteryDisplay)
		delete(m.BatteryState, batteryDisplay)

		err := m.service.EmitPropertiesChanged(m, nil, "BatteryIsPresent",
			"BatteryPercentage", "BatteryState")
		if err != nil {
			logger.Warning(err)
		}
	}

	m.PropsMu.Unlock()

	if warnLevelChanged {
		m.handleWarnLevelChanged(warnLevel)
	}
}

func (m *Manager) disableWarnLevelCountTicker() {
	if m.warnLevelCountTicker != nil {
		m.warnLevelCountTicker.Stop()
		m.warnLevelCountTicker = nil
	}
}

func (m *Manager) handleWarnLevelChanged(level WarnLevel) {
	logger.Debug("handleWarnLevelChanged")
	m.disableWarnLevelCountTicker()

	switch level {
	case WarnLevelAction:
		playSound(soundutils.EventBatteryLow)
		m.sendNotify(iconBatteryLow, "",
			Tr("Battery critically low"))

		m.warnLevelCountTicker = newCountTicker(time.Second, func(count int) {
			if count == 3 {
				// after 3 seconds, lock and then show dde low power
				if m.SleepLock.Get() {
					m.lockWaitShow(5*time.Second, false)
				}
			} else if count == 4 {
				doShowDDELowPower()
			} else if count == 5 {
				// after 5 seconds, force suspend
				m.disableWarnLevelCountTicker()
				m.doSuspend()
			}
		})

	case WarnLevelCritical:
		playSound(soundutils.EventBatteryLow)
		m.sendNotify(iconBatteryLow, "",
			Tr("Battery low, please plug in"))

	case WarnLevelDanger:
		playSound(soundutils.EventBatteryLow)
		m.sendNotify(iconBatteryLow, "",
			Tr("Battery low, please plug in"))

	case WarnLevelLow:
		playSound(soundutils.EventBatteryLow)
		m.sendNotify(iconBatteryLow, "",
			Tr("Battery low, please plug in"))

	case WarnLevelRemind:
		playSound(soundutils.EventBatteryLow)
		m.sendNotify(iconBatteryLow, "",
			Tr("Battery low, please plug in"))

	case WarnLevelNone:
		logger.Debug("Power sufficient")
		doCloseDDELowPower()
		// 由 低电量 到 电量充足，必然需要有线电源插入
	}
}
