// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

// nolint
const (
	// cmd
	cmdDDELowPower = "/usr/lib/deepin-daemon/dde-lowpower"

	batteryDisplay = "Display"
)

// dconfig keys
const (
	dsettingLinePowerScreensaverDelay            = "linePowerScreensaverDelay"
	dsettingBatteryScreensaverDelay              = "batteryScreensaverDelay"
	dsettingPowerSavingModeBrightnessDropPercent = "powerSavingModeBrightnessDropPercent"
	dsettingBatteryScreenBlackDelay              = "batteryScreenBlackDelay"
	dsettingBatterySleepDelay                    = "batterySleepDelay"
	dsettingLinePowerScreenBlackDelay            = "linePowerScreenBlackDelay"
	dsettingLinePowerSleepDelay                  = "linePowerSleepDelay"
	dsettingLinePowerLockDelay                   = "linePowerLockDelay"
	dsettingBatteryLockDelay                     = "batteryLockDelay"
	dsettingAdjustBrightnessEnabled              = "adjustBrightnessEnabled"
	dsettingAmbientLightAdjustBrightness         = "ambientLightAdjustBrightness"
	dsettingScreenBlackLock                      = "screenBlackLock"
	dsettingSleepLock                            = "sleepLock"
	dsettingLidClosedSleep                       = "lidClosedSleep"
	dsettingBatteryLidClosedSleep                = "batteryLidClosedSleep"
	dsettingPowerButtonPressedExec               = "powerButtonPressedExec"
	dsettingFullscreenWorkaroundAppList          = "fullscreenWorkaroundAppList"
	dsettingUsePercentageForPolicy               = "usePercentageForPolicy"
	dsettingPowerModuleInitialized               = "powerModuleInitialized"
	dsettingLowPowerNotifyThreshold              = "lowPowerNotifyThreshold"
	dsettingLowPowerAction                       = "lowPowerAction"
	dsettingPercentageAction                     = "percentageAction"
	dsettingTimeToEmptyLow                       = "timeToEmptyLow"
	dsettingTimeToEmptyDanger                    = "timeToEmptyDanger"
	dsettingTimeToEmptyCritical                  = "timeToEmptyCritical"
	dsettingTimeToEmptyAction                    = "timeToEmptyAction"
	dsettingLinePowerLidClosedAction             = "linePowerLidClosedAction"
	dsettingLinePowerPressPowerButton            = "linePowerPressPowerButton"
	dsettingBatteryLidClosedAction               = "batteryLidClosedAction"
	dsettingBatteryPressPowerButton              = "batteryPressPowerButton"
	dsettingLowPowerNotifyEnable                 = "lowPowerNotifyEnable"
	dsettingSaveBrightnessWhilePsm               = "saveBrightnessWhilePsm"
	dsettingLightSensorEnabled                   = "lightSensorEnabled"
	dsettingLowPowerPercentInUpdatingNotify      = "lowPowerPercentInUpdatingNotify"
	dsettingHighPerformanceEnabled               = "highPerformanceEnabled"
	dsettingsPowerSavingModeEnabled              = "powerSavingModeEnabled"
	dsettingScheduledShutdownState               = "scheduledShutdownState"
	dsettingShutdownTime                         = "shutdownTime"
	dsettingShutdownRepetition                   = "shutdownRepetition"
	dsettingCustomShutdownWeekDays               = "customShutdownWeekDays"
	dsettingShutdownCountdown                    = "shutdownCountdown"
	dsettingNextShutdownTime                     = "nextShutdownTime"
)

const (
	DSettingsAppID        = "org.deepin.dde.daemon"
	DSettingsDisplayName  = "org.deepin.Display"
	DSettingsAutoChangeWm = "auto-change-deepin-wm"
)

const (
	powerActionShutdown int32 = iota
	powerActionSuspend
	powerActionHibernate
	powerActionTurnOffScreen
	powerActionShowShutdownInterface
	powerActionDoNothing
)

const (
	lowPowerActionSuspend int32 = iota
	lowPowerActionHibernate
)
