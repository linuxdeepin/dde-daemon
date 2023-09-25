// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"errors"
	"github.com/godbus/dbus/v5"
	configmanager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (m *Manager) initTlpDsgConfig() error {
	if !isTlpBinOk() {
		return errors.New("tlp is not installed")
	}
	logger.Info("initTlpDsgConfig.")
	// dsg 配置
	var keyMap = map[string]string{
		// 音频
		"soundPowerSaveOnAc":       "SOUND_POWER_SAVE_ON_AC",      // 0/1
		"soundPowerSaveOnBat":      "SOUND_POWER_SAVE_ON_BAT",     // 0/1
		"soundPowerSaveController": "SOUND_POWER_SAVE_CONTROLLER", // Y/N
		// 电池
		"startChargeThreshBatPrimary":   "START_CHARGE_THRESH_BAT0",  // values in %
		"stopChargeThreshBatPrimary":    "STOP_CHARGE_THRESH_BAT0",   // values in %
		"startChargeThreshBatSecondary": "START_CHARGE_THRESH_BAT1",  // values in %
		"stopChargeThreshBatSecondary":  "STOP_CHARGE_THRESH_BAT1",   // values in %
		"restoreThresholdsOnBat":        "RESTORE_THRESHOLDS_ON_BAT", // values in %
		// 驱动器槽
		"bayPoweroffOnAc":  "BAY_POWEROFF_ON_AC",  // 0/1
		"bayPoweroffOnBat": "BAY_POWEROFF_ON_BAT", // 0/1
		"bayDevice":        "BAY_DEVICE",          // Default when unconfigured: sr0
		// 磁盘
		"maxLostWorkSecsOnAc":  "MAX_LOST_WORK_SECS_ON_AC",  // Timeout (in seconds)
		"maxLostWorkSecsOnBat": "MAX_LOST_WORK_SECS_ON_BAT", // Timeout (in seconds)
		// 内核
		"nmiWatchdog": "NMI_WATCHDOG", // 0/1
		// 网络
		"wifiPwrOnAc":  "WIFI_PWR_ON_AC",  // of/on
		"wifiPwrOnBat": "WIFI_PWR_ON_BAT", // of/on
		"wolDisable":   "WOL_DISABLE",     // Y/N
		// 无线设备
		"restoreDeviceStateOnStartup":    "RESTORE_DEVICE_STATE_ON_STARTUP",      // 0/1
		"devicesToDisableOnStartup":      "DEVICES_TO_DISABLE_ON_STARTUP",        // bluetooth/wifi/wwan (Multiple devices are separated with blanks)
		"devicesToEableOnStartup":        "DEVICES_TO_ENABLE_ON_STARTUP",         // as above
		"devicesToDisableOnShutdown":     "DEVICES_TO_DISABLE_ON_SHUTDOWN",       // as above
		"devicesToEableOnShutdown":       "DEVICES_TO_ENABLE_ON_SHUTDOWN",        // as above
		"devicesToEableOnAc":             "DEVICES_TO_ENABLE_ON_AC",              // as above
		"devicesToDisableOnBat":          "DEVICES_TO_DISABLE_ON_BAT",            // as above
		"devicesToDisableOnBatNotInUse":  "DEVICES_TO_DISABLE_ON_BAT_NOT_IN_USE", // as above
		"devicesToDisableOnLanConnect":   "DEVICES_TO_DISABLE_ON_LAN_CONNECT",    // as above
		"devicesToDisableOnWifiConnect":  "DEVICES_TO_DISABLE_ON_WIFI_CONNECT",   // as above
		"devicesToDisableOnWwanConnect":  "DEVICES_TO_DISABLE_ON_WWAN_CONNECT",   // as above
		"devicesToEableOnLanDisconnect":  "DEVICES_TO_ENABLE_ON_LAN_DISCONNECT",  // as above
		"devicesToEableOnWifiDisconnect": "DEVICES_TO_ENABLE_ON_WIFI_DISCONNECT", // as above
		"devicesToEableOnWwanDisconnect": "DEVICES_TO_ENABLE_ON_WWAN_DISCONNECT", // as above
		"devicesToEableOnDock":           "DEVICES_TO_ENABLE_ON_DOCK",            // as above
		"devicesToDisableOnDock":         "DEVICES_TO_DISABLE_ON_DOCK",           // as above
		"devicesToEableOnUndock":         "DEVICES_TO_ENABLE_ON_UNDOCK",          // as above
		"devicesToDisableOnUndock":       "DEVICES_TO_DISABLE_ON_UNDOCK",         // as above
		// 运行时电源管理
		"runtimePmOnAc":  "RUNTIME_PM_ON_AC",  // on/auto
		"runtimePmOnBat": "RUNTIME_PM_ON_BAT", // on/auto
		"pcieAspmOnAc":   "PCIE_ASPM_ON_AC",   // default/performance/powersave/powersupersave
		"pcieAspmOnBat":  "PCIE_ASPM_ON_BAT",  // default/performance/powersave/powersupersave
		// USB
		"usbAutosuspend":                  "USB_AUTOSUSPEND",                     // 0/1
		"usbAutosuspendDisableOnShutdown": "USB_AUTOSUSPEND_DISABLE_ON_SHUTDOWN", // 0/1
		// 硬盘
		"diskApmLevelOnAc":  "DISK_APM_LEVEL_ON_AC",  // default:"254 254"
		"diskApmLevelOnBat": "DISK_APM_LEVEL_ON_BAT", // default:"128 128"
		"sataLinkPwrOnAc":   "SATA_LINKPWR_ON_AC",    // default:"med_power_with_dipm max_performance"
		"sataLinkPwrOnBat":  "SATA_LINKPWR_ON_BAT",   // default:"med_power_with_dipm min_power"
	}

	ds := configmanager.NewConfigManager(m.systemSigLoop.Conn())
	dsTlpPath, err := ds.AcquireManager(0, dsettingsAppID, dsettingsTlpName, "")
	if err != nil {
		return err
	}
	dsTlp, err := configmanager.NewManager(m.systemSigLoop.Conn(), dsTlpPath)
	if err != nil {
		return err
	}

	getTlpChangedVal := func(key string) (val string) {
		data, err := dsTlp.Value(0, key)
		if err != nil {
			logger.Warning(err)
			return
		}

		return data.Value().(string)
	}

	dsTlp.InitSignalExt(m.systemSigLoop, true)
	dsTlp.ConnectValueChanged(func(key string) {
		logger.Info("DSG org.deepin.dde.daemon.tlp valueChanged, key : ", key)
		setTlpConfig(keyMap[key], getTlpChangedVal(key))
	})

	return err
}

func (m *Manager) initBluetoothAdapterDsgConfig() error {
	logger.Info("initBluetoothAdapterDsgConfig.")
	ds := configmanager.NewConfigManager(m.systemSigLoop.Conn())
	dsBluetoothAdapterPath, err := ds.AcquireManager(0, dsettingsAppID, dsettingsBluetoothAdapterName, "")
	if err != nil {
		return err
	}
	dsBluetoothAdapter, err := configmanager.NewManager(m.systemSigLoop.Conn(), dsBluetoothAdapterPath)
	if err != nil {
		return err
	}

	getBluetoothAdapterChangedVal := func(key string) (val string) {
		data, err := dsBluetoothAdapter.Value(0, key)
		if err != nil {
			logger.Warning(err)
			return
		}

		return data.Value().(string)
	}

	dsBluetoothAdapter.InitSignalExt(m.systemSigLoop, true)
	dsBluetoothAdapter.ConnectValueChanged(func(key string) {
		logger.Info("DSG org.deepin.dde.daemon.bluetoothAdapter valueChanged, key : ", key)
		go setHciconfig(getBluetoothAdapterChangedVal(key))
		m.bluetoothAdapterEnabledCmd = getBluetoothAdapterChangedVal("enableDevices")
		m.bluetoothAdapterScanCmd = getBluetoothAdapterChangedVal("scanDevices")
	})

	m.bluetoothAdapterEnabledCmd = getBluetoothAdapterChangedVal("enableDevices")
	m.bluetoothAdapterScanCmd = getBluetoothAdapterChangedVal("scanDevices")

	return err
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
	m.setPowerSavingModeEnabled(enable)
	m.PropsMu.Unlock()
}

func (m *Manager) writePowerSavingModeEnabledCb(write *dbusutil.PropertyWrite) *dbus.Error {
	logger.Info("set tlp enabled", write.Value)
	return dbusutil.ToError(m.writePowerSavingModeEnabledCbImpl(write.Value.(bool)))
}

func (m *Manager) writePowerSavingModeEnabledCbImpl(enabled bool) error {
	var err error
	var tlpCfgChanged bool
	bluetoothAdapterEnabledCmd := "up"
	bluetoothAdapterScanCmd := "piscan"

	m.setPropPowerSavingModeEnabled(enabled)

	if enabled {
		tlpCfgChanged, err = setTlpConfigMode(tlpConfigEnabled)
		bluetoothAdapterEnabledCmd = m.bluetoothAdapterEnabledCmd
		bluetoothAdapterScanCmd = m.bluetoothAdapterScanCmd
	} else {
		tlpCfgChanged, err = setTlpConfigMode(tlpConfigDisabled)
	}

	go func() {
		setHciconfig(bluetoothAdapterEnabledCmd)
		setHciconfig(bluetoothAdapterScanCmd)
	}()

	if err != nil {
		logger.Warning("failed to set tlp config:", err)
	}

	if tlpCfgChanged {
		err := reloadTlpService()
		if err != nil {
			logger.Warning(err)
		}
	}

	return nil
}
