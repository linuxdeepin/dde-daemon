/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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

type WarnLevel uint32

const (
	WarnLevelNone WarnLevel = iota
	WarnLevelLow
	WarnLevelDanger
	WarnLevelCritical
	WarnLevelAction
)

func (lv WarnLevel) String() string {
	switch lv {
	case WarnLevelNone:
		return "None"
	case WarnLevelLow:
		return "Low"
	case WarnLevelDanger:
		return "Danger"
	case WarnLevelCritical:
		return "Critical"
	case WarnLevelAction:
		return "Action"
	default:
		return "Unknown"
	}
}

func getWarnLevel(config *warnLevelConfig, onBattery bool,
	percentage float64, timeToEmpty uint64) WarnLevel { // 低电量的处理

	if !onBattery {
		return WarnLevelNone
	}

	usePercentageForPolicy := config.UsePercentageForPolicy
	logger.Debugf("_getWarnLevel onBattery %v, percentage %v, timeToEmpty %v, usePercentage %v",
		onBattery, percentage, timeToEmpty, usePercentageForPolicy)
	if usePercentageForPolicy {
		if percentage == 0.0 {
			return WarnLevelNone
		}

		// 当电池电量到达低电量阈值且达到系统固定低电量提醒值时，才去弹低电量提醒通知
		if percentage <= config.LowPowerNotifyThreshold {
			if percentage <= config.ActionPercentage {
				return WarnLevelAction
			}

			if percentage <= config.CriticalPercentage {
				return WarnLevelCritical
			}

			if percentage <= config.DangerPercentage {
				return WarnLevelDanger
			}

			if percentage <= config.LowPercentage && config.LowPercentage <= config.LowPowerNotifyThreshold {
				return WarnLevelLow
			}

			return WarnLevelNone
		}

		return WarnLevelNone
	} else {
		if timeToEmpty > config.LowTime || timeToEmpty == 0 {
			return WarnLevelNone
		}
		if timeToEmpty > config.DangerTime {
			return WarnLevelLow
		}
		if timeToEmpty > config.CriticalTime {
			return WarnLevelDanger
		}
		if timeToEmpty > config.ActionTime {
			return WarnLevelCritical
		}
		return WarnLevelAction
	}
}

func (m *Manager) getWarnLevel(percentage float64, timeToEmpty uint64) WarnLevel {
	return getWarnLevel(m.warnLevelConfig.getWarnLevelConfig(), m.OnBattery, percentage, timeToEmpty)
}
