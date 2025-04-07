// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"time"

	dbus "github.com/godbus/dbus/v5"
)

type warnLevelConfig struct {
	UsePercentageForPolicy  bool
	LowTime                 uint64
	DangerTime              uint64
	CriticalTime            uint64
	ActionTime              uint64
	LowPowerNotifyThreshold float64
	remindPercentage        float64 // 废弃
	LowPercentage           float64 // 废弃
	DangerPercentage        float64 // 废弃
	CriticalPercentage      float64 // 废弃
	ActionPercentage        float64
}

func (c *warnLevelConfig) isValid() bool {
	if c.LowTime > c.DangerTime &&
		c.DangerTime > c.CriticalTime &&
		c.CriticalTime > c.ActionTime &&
		c.remindPercentage > c.LowPercentage &&
		c.LowPercentage > c.DangerPercentage &&
		c.DangerPercentage > c.CriticalPercentage &&
		c.CriticalPercentage > c.ActionPercentage {
		return true
	}
	return false
}

type WarnLevelConfigManager struct {
	UsePercentageForPolicy bool `prop:"access:rw"`

	LowTime      int64 `prop:"access:rw"`
	DangerTime   int64 `prop:"access:rw"`
	CriticalTime int64 `prop:"access:rw"`
	ActionTime   int64 `prop:"access:rw"`

	LowPowerNotifyThreshold int64 `prop:"access:rw"`

	ActionPercentage int64 `prop:"access:rw"`

	changeTimer *time.Timer
	changeCb    func()

	powerManager *Manager
}

func NewWarnLevelConfigManager(manager *Manager) *WarnLevelConfigManager {
	m := &WarnLevelConfigManager{}
	m.powerManager = manager
	return m
}

func (m *WarnLevelConfigManager) initDsg() {
	needUpdateConfigKeys := []string{
		dsettingUsePercentageForPolicy,
		dsettingTimeToEmptyLow,
		dsettingTimeToEmptyDanger,
		dsettingTimeToEmptyCritical,
		dsettingTimeToEmptyAction,
		dsettingPercentageAction,
		dsettingLowPowerNotifyThreshold,
	}
	for _, key := range needUpdateConfigKeys {
		m.getConfig(key)
	}

	m.powerManager.dsPowerConfigManager.ConnectValueChanged(func(key string) {
		m.getConfig(key)
		m.notifyChange()
	})
}

func (m *WarnLevelConfigManager) getConfig(key string) {
	data, err := m.powerManager.dsPowerConfigManager.Value(0, key)

	if err != nil {
		logger.Warning(err)
		return
	}

	switch key {
	case dsettingUsePercentageForPolicy:
		m.UsePercentageForPolicy = data.Value().(bool)
	case dsettingTimeToEmptyLow:
		m.LowTime = data.Value().(int64)
	case dsettingTimeToEmptyDanger:
		m.DangerTime = data.Value().(int64)
	case dsettingTimeToEmptyCritical:
		m.CriticalTime = data.Value().(int64)
	case dsettingTimeToEmptyAction:
		m.ActionTime = data.Value().(int64)
	case dsettingLowPowerNotifyThreshold:
		m.LowPowerNotifyThreshold = data.Value().(int64)
	case dsettingPercentageAction:
		m.ActionPercentage = data.Value().(int64)
	}
}

func (m *WarnLevelConfigManager) getWarnLevelConfig() *warnLevelConfig {
	return &warnLevelConfig{
		UsePercentageForPolicy: m.UsePercentageForPolicy,
		LowTime:                uint64(m.LowTime),
		DangerTime:             uint64(m.DangerTime),
		CriticalTime:           uint64(m.CriticalTime),
		ActionTime:             uint64(m.ActionTime),

		LowPowerNotifyThreshold: float64(m.LowPowerNotifyThreshold),
		remindPercentage:        float64(25),
		LowPercentage:           float64(20),
		DangerPercentage:        float64(15),
		CriticalPercentage:      float64(10),
		ActionPercentage:        float64(m.ActionPercentage),
	}
}

func (m *WarnLevelConfigManager) setChangeCallback(fn func()) {
	m.changeCb = fn
}

func (m *WarnLevelConfigManager) delayCheckValid() {
	logger.Debug("delayCheckValid")
	if m.changeTimer != nil {
		m.changeTimer.Stop()
	}
	m.changeTimer = time.AfterFunc(20*time.Second, func() {
		logger.Debug("checkValid")
		wlc := m.getWarnLevelConfig()
		if !wlc.isValid() {
			logger.Info("Warn level config is invalid, reset")
			err := m.Reset()
			if err != nil {
				logger.Warning(err)
			}
		}
	})
}

func (m *WarnLevelConfigManager) notifyChange() {
	if m.changeCb != nil {
		logger.Debug("WarnLevelConfig change")
		m.changeCb()
	}
	m.delayCheckValid()
}

func (m *WarnLevelConfigManager) connectSettingsChanged() {
	m.powerManager.dsPowerConfigManager.ConnectValueChanged(func(key string) {
		switch key {
		case dsettingUsePercentageForPolicy,
			dsettingLowPowerNotifyThreshold,
			dsettingPercentageAction,
			dsettingTimeToEmptyLow,
			dsettingTimeToEmptyDanger,
			dsettingTimeToEmptyCritical,
			dsettingTimeToEmptyAction:

			logger.Debug("key changed", key)
			m.notifyChange()
		}
	})

}

func (m *WarnLevelConfigManager) Reset() *dbus.Error {
	needResetConfigKeys := []string{
		dsettingUsePercentageForPolicy,
		dsettingLowPowerNotifyThreshold,
		dsettingPercentageAction,
		dsettingTimeToEmptyLow,
		dsettingTimeToEmptyDanger,
		dsettingTimeToEmptyCritical,
		dsettingTimeToEmptyAction,
	}

	for _, key := range needResetConfigKeys {
		m.powerManager.dsPowerConfigManager.Reset(0, key)
	}

	return nil
}

func (*WarnLevelConfigManager) GetInterfaceName() string {
	return dbusInterface + ".WarnLevelConfig"
}
