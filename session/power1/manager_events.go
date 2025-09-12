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
		m.PropsMu.Unlock()

		if changed {
			if onBattery {
				playSound(soundutils.EventPowerUnplug)
			} else {
				playSound(soundutils.EventPowerPlug)
			}
		}
	})

	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) handleBeforeSuspend() {
	m.setPrepareSuspend(suspendStatePrepare)

	// 检查屏幕保护状态（删除原有的运行状态获取逻辑，仅在未捕获时补充获取锁屏需求）
	if !m.screensaverStateCaptured {
		m.screensaverLockAtAwake = m.isScreensaverLockAtAwake()
	}
	logger.Debug("before sleep, screensaver running:", m.screensaverWasRunning, "lock at awake:", m.screensaverLockAtAwake)

	m.setDDEBlackScreenActive(true)
	if m.UseWayland {
		// 如果是treeland，并且待机休眠唤醒是需要解锁，则将session 给lock
		if m.SleepLock {
			err := m.currentSession.Lock(0)
			if err != nil {
				logger.Warning("set session lock failed:", err)
			}
		}
	}
	logger.Debug("before sleep")
}

func (m *Manager) handleWakeup() {
	m.setPrepareSuspend(suspendStateWakeup)
	logger.Debug("wakeup")

	// 唤醒后还原捕获状态标志，避免影响后续逻辑
	m.screensaverStateCaptured = false

	// Fix wayland sometimes no dpms event after wakeup
	if m.UseWayland {
		err := m.service.Conn().Object("org.deepin.dde.KWayland1",
			"/org/deepin/dde/KWayland1/Output").Call("org.deepin.dde.KWayland1.Idle.simulateUserActivity", 0).Err

		if err != nil {
			logger.Warning(err)
		}
	}

	// 系统唤醒后重新处理关机定时器
	if m.ScheduledShutdownState {
		m.scheduledShutdown(Init)
	}

	m.delayInActive = true
	time.AfterFunc(time.Duration(m.delayWakeupInterval)*time.Second, func() {
		m.delayInActive = false
		playSound(soundutils.EventWakeup)

		// 如果睡眠前屏幕保护正在运行且恢复时需要密码，唤醒时必须显示锁屏界面
		if m.screensaverWasRunning && m.screensaverLockAtAwake {
			logger.Debug("wakeup: screensaver was running and lock at awake, show lock screen")
			m.doLock(true)
		}

		m.setDDEBlackScreenActive(false)
	})

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
}

func (m *Manager) handleRefreshMains() {
	logger.Debug("handleRefreshMains")
	power := m.helper.Power
	err := power.RefreshMains(0)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) handleWakeupDDELowPowerCheck() {
	power := m.helper.Power
	hasBattery, err := power.HasBattery().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	logger.Info(" PrepareForSleep(False) : wakeup, hasBattery : ", hasBattery)

	if hasBattery {
		// when wakeup update onBattery state
		onBattery, err := power.OnBattery().Get(0)
		if err != nil {
			logger.Warning(err)
			return
		}
		if onBattery != m.OnBattery {
			m.setPropOnBattery(onBattery)
		}

		// 使用电池
		if onBattery {
			percentage, err := power.BatteryPercentage().Get(0)
			if err != nil {
				logger.Warning(err)
				return
			}
			m.setPropBatteryPercentage(percentage)

			logger.Info(" PrepareForSleep(False) : wakeup,  Battery percentage : ", percentage, m.BatteryPercentage[batteryDisplay], m.warnLevelConfig.ActionPercentage)

			if percentage <= float64(m.warnLevelConfig.ActionPercentage) {
				return
			}
		}

		doCloseDDELowPower()
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
		if m.ScheduledShutdownState {
			m.scheduledShutdown(Init)
		}
		playSound(soundutils.EventBatteryLow)
		m.sendNotify(iconBatteryLow, "",
			Tr("Battery critically low"))

		m.warnLevelCountTicker = newCountTicker(time.Second, func(count int) {
			if count == 3 {
				// after 3 seconds, lock and then show dde low power
				if m.SleepLock {
					m.lockWaitShow(5*time.Second, false)
				}
			} else if count == 4 {
				doShowDDELowPower()
			} else if count == 5 {
				// after 5 seconds, force suspend
				m.disableWarnLevelCountTicker()

				if m.LowPowerAction == lowPowerActionSuspend {
					m.doSuspend()
				} else {
					m.doHibernate()
				}
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
		if m.ScheduledShutdownState {
			m.scheduledShutdown(Init)
		}
		// 由 低电量 到 电量充足，必然需要有线电源插入
	}
}
