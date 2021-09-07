/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package power

import (
	"time"

	dbus "github.com/godbus/dbus"
	"pkg.deepin.io/dde/api/soundutils"
	. "pkg.deepin.io/lib/gettext"
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
	logger.Debug("before sleep")
}

func (m *Manager) handleWakeup() {
	m.setPrepareSuspend(suspendStateWakeup)
	logger.Debug("wakeup")

	// Fix wayland sometimes no dpms event after wakeup
	if m.UseWayland {
		err := m.helper.ScreenSaver.SimulateUserActivity(0)
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
		warnLevelChanged = m.setPropWarnLevel(warnLevel)

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
				go func() {
					if m.SleepLock.Get() {
						m.lockWaitShow(5*time.Second, false)
					}
					doShowDDELowPower()
				}()
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

	case WarnLevelNone:
		logger.Debug("Power sufficient")
		doCloseDDELowPower()
		// 由 低电量 到 电量充足，必然需要有线电源插入
	}
}
