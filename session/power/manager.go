// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-daemon/common/dsync"
	"github.com/linuxdeepin/dde-daemon/session/common"
	display "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.display"
	calendar "github.com/linuxdeepin/go-dbus-factory/com.deepin.dataserver.Calendar"
	shutdownfront "github.com/linuxdeepin/go-dbus-factory/com.deepin.dde.shutdownfront"
	sessionmanager "github.com/linuxdeepin/go-dbus-factory/com.deepin.sessionmanager"
	systemPower "github.com/linuxdeepin/go-dbus-factory/com.deepin.system.power"
	wm "github.com/linuxdeepin/go-dbus-factory/com.deepin.wm"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	DisplayManager "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.DisplayManager"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	notifications "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.notifications"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/gsprop"
	"github.com/linuxdeepin/go-lib/gettext"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

//go:generate dbusutil-gen -type Manager manager.go
//go:generate dbusutil-gen em -type Manager,WarnLevelConfigManager

const (
	DSettingsAppID        = "org.deepin.startdde"
	DSettingsDisplayName  = "org.deepin.Display"
	DSettingsAutoChangeWm = "auto-change-deepin-wm"

	// 定时关机
	dsettingScheduledShutdownState = "scheduledShutdownState"
	dsettingShutdownTime           = "shutdownTime"
	dsettingShutdownRepetition     = "shutdownRepetition"
	dsettingCustomShutdownWeekDays = "customShutdownWeekDays"
	dsettingShutdownCountdown      = "shutdownCountdown"
)

const (
	Once int = iota
	Everyday
	Workdays
	Custom
)

type FestivalRootConfig struct {
	Description  string        `json:"description"`
	Id           string        `json:"id"`
	List         []HolidayDate `json:"list"`
	Month        byte          `json:"month"`
	NewDaemoname string        `json:"name"`
	Rest         string        `json:"rest"`
}

type HolidayDate struct {
	Date   string `json:"date"`
	Status byte   `json:"status"`
}

type Manager struct {
	service              *dbusutil.Service
	sessionSigLoop       *dbusutil.SignalLoop
	systemSigLoop        *dbusutil.SignalLoop
	syncConfig           *dsync.Config
	helper               *Helper
	settings             *gio.Settings
	warnLevelCountTicker *countTicker
	warnLevelConfig      *WarnLevelConfigManager
	submodules           map[string]submodule
	inhibitor            *sleepInhibitor
	inhibitFd            dbus.UnixFD
	systemPower          systemPower.Power
	display              display.Display
	calendar             calendar.HuangLi
	objLogin             login1.Manager
	ddeShutdown          shutdownfront.ShutdownFront
	displayManager       DisplayManager.DisplayManager
	lightSensorEnabled   bool

	sessionManager     sessionmanager.SessionManager
	currentSessionPath dbus.ObjectPath
	currentSession     login1.Session

	// 定时关机
	ScheduledShutdownState bool   `prop:"access:rw"`
	ShutdownTime           string `prop:"access:rw"`
	ShutdownRepetition     int    `prop:"access:rw"`
	// dbusutil-gen: equal=byteSliceEqual
	CustomShutdownWeekDays []byte `prop:"access:rw"`
	shutdownTimer          *time.Timer
	shutdownCountdown      int
	notifyId               uint32
	notifyIdMu             sync.Mutex
	notify                 notifications.Notifications
	countdowning           bool

	PropsMu sync.RWMutex
	// 是否有盖子，一般笔记本电脑才有
	LidIsPresent bool
	// 是否使用电池, 接通电源时为 false, 使用电池时为 true
	OnBattery bool
	// 是否使用Wayland
	UseWayland bool
	// 警告级别
	WarnLevel WarnLevel

	// 是否有环境光传感器
	HasAmbientLightSensor bool

	// dbusutil-gen: ignore-below
	// 电池是否可用，是否存在
	BatteryIsPresent map[string]bool
	// 电池电量百分比
	BatteryPercentage map[string]float64
	// 电池状态
	BatteryState map[string]uint32

	// 接通电源时，不做任何操作，到显示屏保的时间
	LinePowerScreensaverDelay gsprop.Int `prop:"access:rw"`
	// 接通电源时，不做任何操作，到关闭屏幕的时间
	LinePowerScreenBlackDelay gsprop.Int `prop:"access:rw"`
	// 接通电源时，不做任何操作，到睡眠的时间
	LinePowerSleepDelay gsprop.Int `prop:"access:rw"`

	// 使用电池时，不做任何操作，到显示屏保的时间
	BatteryScreensaverDelay gsprop.Int `prop:"access:rw"`
	// 使用电池时，不做任何操作，到关闭屏幕的时间
	BatteryScreenBlackDelay gsprop.Int `prop:"access:rw"`
	// 使用电池时，不做任何操作，到睡眠的时间
	BatterySleepDelay gsprop.Int `prop:"access:rw"`

	// 关闭屏幕前是否锁定
	ScreenBlackLock gsprop.Bool `prop:"access:rw"`
	// 睡眠前是否锁定
	SleepLock gsprop.Bool `prop:"access:rw"`

	// 废弃
	LidClosedSleep gsprop.Bool `prop:"access:rw"`

	// 接通电源时，笔记本电脑盖上盖子 待机（默认选择）、睡眠、关闭显示器、无任何操作
	LinePowerLidClosedAction gsprop.Enum `prop:"access:rw"`

	// 接通电源时，按下电源按钮 关机（默认选择）、待机、睡眠、关闭显示器、无任何操作
	LinePowerPressPowerBtnAction gsprop.Enum `prop:"access:rw"` // keybinding中监听power按键事件,获取gsettings的值

	// 使用电池时，笔记本电脑盖上盖子 待机（默认选择）、睡眠、关闭显示器、无任何操作
	BatteryLidClosedAction gsprop.Enum `prop:"access:rw"`

	// 使用电池时，按下电源按钮 关机（默认选择）、待机、睡眠、关闭显示器、无任何操作
	BatteryPressPowerBtnAction gsprop.Enum `prop:"access:rw"` // keybinding中监听power按键事件,获取gsettings的值

	// 接通电源时，不做任何操作，到自动锁屏的时间
	LinePowerLockDelay gsprop.Int `prop:"access:rw"`
	// 使用电池时，不做任何操作，到自动锁屏的时间
	BatteryLockDelay gsprop.Int `prop:"access:rw"`

	// 打开电量通知
	LowPowerNotifyEnable gsprop.Bool `prop:"access:rw"` // 开启后默认当电池仅剩余达到电量水平低时（默认15%）发出系统通知“电池电量低，请连接电源”；
	// 当电池仅剩余为设置低电量时（默认5%），发出系统通知“电池电量耗尽”，进入待机模式；

	// 电池低电量通知百分比
	LowPowerNotifyThreshold gsprop.Int `prop:"access:rw"` // 设置电量低提醒的阈值，可设置范围10%-25%，默认为20%

	// 自动待机电量百分比
	LowPowerAutoSleepThreshold gsprop.Int `prop:"access:rw"` // 设置电池电量进入待机模式（s3）的阈值，可设置范围为1%-9%，默认为5%（范围待确定）

	savingModeBrightnessDropPercent gsprop.Int // 用来接收和保存来自system power中降低的屏幕亮度值

	AmbientLightAdjustBrightness gsprop.Bool `prop:"access:rw"`

	ambientLightClaimed bool
	lightLevelUnit      string
	lidSwitchState      uint
	sessionActive       bool
	sessionActiveTime   time.Time

	// if prepare suspend, ignore idle off
	prepareSuspend       int
	prepareSuspendLocker sync.Mutex

	// 是否支持高性能模式
	IsHighPerformanceSupported bool
	gsHighPerformanceEnabled   bool

	// 是否支持节能模式
	isPowerSaveSupported bool
	kwinHanleIdleOffCh   chan bool

	dsDisplayConfigManager configManager.Manager
	dsPowerConfigManager   configManager.Manager
	wmDBus                 wm.Wm

	delayInActive       bool
	delayWakeupInterval uint32
}

var _manager *Manager

func newManager(service *dbusutil.Service) (*Manager, error) {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	m := new(Manager)
	m.service = service
	sessionBus := service.Conn()
	m.sessionSigLoop = dbusutil.NewSignalLoop(sessionBus, 10)
	m.systemSigLoop = dbusutil.NewSignalLoop(systemBus, 10)
	m.objLogin = login1.NewManager(systemBus)
	m.ddeShutdown = shutdownfront.NewShutdownFront(sessionBus)
	m.displayManager = DisplayManager.NewDisplayManager(systemBus)
	m.inhibitFd = -1
	m.prepareSuspend = suspendStateUnknown

	m.syncConfig = dsync.NewConfig("power", &syncConfig{m: m}, m.sessionSigLoop, dbusPath, logger)

	helper, err := newHelper(systemBus, sessionBus)
	if err != nil {
		return nil, err
	}
	m.helper = helper

	m.sessionManager = sessionmanager.NewSessionManager(sessionBus)
	m.currentSessionPath, err = m.sessionManager.CurrentSessionPath().Get(0)
	if err != nil || m.currentSessionPath == "" {
		logger.Warning("get sessionManager CurrentSessionPath failed:", err)
		return nil, err
	}
	m.currentSession, err = login1.NewSession(systemBus, m.currentSessionPath)
	if err != nil || m.currentSession == nil {
		logger.Error("Failed to connect self session:", err)
		return nil, err
	}

	m.settings = gio.NewSettings(gsSchemaPower)
	m.warnLevelConfig = NewWarnLevelConfigManager(m.settings)

	m.LinePowerScreensaverDelay.Bind(m.settings, settingKeyLinePowerScreensaverDelay)
	m.LinePowerScreenBlackDelay.Bind(m.settings, settingKeyLinePowerScreenBlackDelay)
	m.LinePowerSleepDelay.Bind(m.settings, settingKeyLinePowerSleepDelay)
	m.LinePowerLockDelay.Bind(m.settings, settingKeyLinePowerLockDelay)
	m.BatteryScreensaverDelay.Bind(m.settings, settingKeyBatteryScreensaverDelay)
	m.BatteryScreenBlackDelay.Bind(m.settings, settingKeyBatteryScreenBlackDelay)
	m.BatterySleepDelay.Bind(m.settings, settingKeyBatterySleepDelay)
	m.BatteryLockDelay.Bind(m.settings, settingKeyBatteryLockDelay)
	m.ScreenBlackLock.Bind(m.settings, settingKeyScreenBlackLock)
	m.SleepLock.Bind(m.settings, settingKeySleepLock)

	m.LinePowerLidClosedAction.Bind(m.settings, settingKeyLinePowerLidClosedAction)
	m.LinePowerPressPowerBtnAction.Bind(m.settings, settingKeyLinePowerPressPowerBtnAction)
	m.BatteryLidClosedAction.Bind(m.settings, settingKeyBatteryLidClosedAction)
	m.BatteryPressPowerBtnAction.Bind(m.settings, settingKeyBatteryPressPowerBtnAction)
	m.LowPowerNotifyEnable.Bind(m.settings, settingKeyLowPowerNotifyEnable)
	m.LowPowerNotifyThreshold.Bind(m.settings, settingKeyLowPowerNotifyThreshold)
	m.LowPowerAutoSleepThreshold.Bind(m.settings, settingKeyLowPowerAutoSleepThreshold)
	m.savingModeBrightnessDropPercent.Bind(m.settings, settingKeyBrightnessDropPercent)
	m.initGSettingsConnectChanged()
	m.AmbientLightAdjustBrightness.Bind(m.settings,
		settingKeyAmbientLightAdjuestBrightness)
	m.lightSensorEnabled = m.settings.GetBoolean(settingLightSensorEnabled)
	m.gsHighPerformanceEnabled = m.settings.GetBoolean(settingKeyHighPerformanceEnabled)

	power := m.helper.Power
	err = common.ActivateSysDaemonService(power.ServiceName_())
	if err != nil {
		logger.Warning(err)
	}

	m.LidIsPresent, err = power.HasLidSwitch().Get(0)
	if err != nil {
		logger.Warning(err)
	}

	m.OnBattery, err = power.OnBattery().Get(0)
	if err != nil {
		logger.Warning(err)
	}

	logger.Info("LidIsPresent", m.LidIsPresent)

	if m.lightSensorEnabled {
		m.HasAmbientLightSensor, _ = helper.SensorProxy.HasAmbientLight().Get(0)
		logger.Debug("HasAmbientLightSensor:", m.HasAmbientLightSensor)
		if m.HasAmbientLightSensor {
			m.lightLevelUnit, _ = helper.SensorProxy.LightLevelUnit().Get(0)
		}
	}

	m.sessionActive, _ = helper.SessionWatcher.IsActive().Get(0)

	// init battery display
	m.BatteryIsPresent = make(map[string]bool)
	m.BatteryPercentage = make(map[string]float64)
	m.BatteryState = make(map[string]uint32)

	// 初始化电源模式
	m.systemPower = systemPower.NewPower(systemBus)
	isHighPerformanceSupported, err := m.systemPower.IsHighPerformanceSupported().Get(0)
	if err != nil {
		logger.Warning("Get systemPower.IsHighPerformanceSupported err :", err)
	}
	m.setPropIsHighPerformanceSupported(isHighPerformanceSupported && m.settings.GetBoolean(settingKeyHighPerformanceEnabled))
	m.isPowerSaveSupported, err = m.systemPower.IsPowerSaveSupported().Get(0)
	if err != nil {
		logger.Warning("Get systemPower.IsPowerSaveSupported err :", err)
	}

	// 绑定com.deepin.daemon.Display的DBus
	m.display = display.NewDisplay(sessionBus)
	m.wmDBus = wm.NewWm(sessionBus)

	m.calendar = calendar.NewHuangLi(sessionBus)
	m.notify = notifications.NewNotifications(sessionBus)
	m.notify.InitSignalExt(m.sessionSigLoop, true)

	return m, nil
}

func (m *Manager) init() {
	m.claimOrReleaseAmbientLight()
	m.sessionSigLoop.Start()
	m.systemSigLoop.Start()

	if len(os.Getenv("WAYLAND_DISPLAY")) != 0 {
		m.UseWayland = true
	} else {
		m.UseWayland = false
	}

	logger.Info("init Getenv(WAYLAND_DISPLAY) UseWayland : ", m.UseWayland)
	m.helper.initSignalExt(m.systemSigLoop, m.sessionSigLoop)

	// init sleep inhibitor
	m.inhibitor = newSleepInhibitor(m.helper.LoginManager, m.helper.Daemon)
	m.inhibitor.OnBeforeSuspend = m.handleBeforeSuspend
	m.inhibitor.OnWakeup = m.handleWakeup
	err := m.inhibitor.block()
	if err != nil {
		logger.Warning(err)
	}

	m.handleBatteryDisplayUpdate()
	power := m.helper.Power
	_, err = power.ConnectBatteryDisplayUpdate(func(timestamp int64) {
		logger.Debug("BatteryDisplayUpdate", timestamp)
		m.handleBatteryDisplayUpdate()
	})
	if err != nil {
		logger.Warning(err)
	}

	if m.lightSensorEnabled {
		err = m.helper.SensorProxy.LightLevel().ConnectChanged(func(hasValue bool, value float64) {
			if !hasValue {
				return
			}
			m.handleLightLevelChanged(value)
		})
		if err != nil {
			logger.Warning(err)
		}
	}

	_, err = m.helper.SysDBusDaemon.ConnectNameOwnerChanged(
		func(name string, oldOwner string, newOwner string) {
			if m.lightSensorEnabled {
				serviceName := m.helper.SensorProxy.ServiceName_()
				if name == serviceName && newOwner != "" {
					logger.Debug("sensorProxy restarted")
					hasSensor, _ := m.helper.SensorProxy.HasAmbientLight().Get(0)
					var lightLevelUnit string
					if hasSensor {
						lightLevelUnit, _ = m.helper.SensorProxy.LightLevelUnit().Get(0)
					}

					m.PropsMu.Lock()
					m.setPropHasAmbientLightSensor(hasSensor)
					m.ambientLightClaimed = false
					m.lightLevelUnit = lightLevelUnit
					m.PropsMu.Unlock()

					m.claimOrReleaseAmbientLight()
				}
			}
			if name == m.helper.LoginManager.ServiceName_() && oldOwner != "" && newOwner == "" {
				if m.prepareSuspend == suspendStatePrepare {
					logger.Info("auto handleWakeup if systemd-logind coredump")
					m.handleWakeup()
				}
			}
		})
	if err != nil {
		logger.Warning(err)
	}

	err = m.helper.SessionWatcher.IsActive().ConnectChanged(func(hasValue bool, value bool) {
		if !hasValue {
			return
		}

		m.PropsMu.Lock()
		m.sessionActive = value
		m.sessionActiveTime = time.Now()
		m.PropsMu.Unlock()

		logger.Debug("session active changed to:", value)
		m.claimOrReleaseAmbientLight()
		// 用户不活跃停止关机倒计时
		if !m.sessionActive && m.countdowning {
			m.scheduledShutdownSwitch(false, true)
			m.notify.CloseNotification(0, m.notifyId)
		} else if m.sessionActive && m.ScheduledShutdownState {
			if m.countdowning && m.ShutdownRepetition == Once && !m.checkShutdownTrigger() {
				m.actionShutdown(true)
			} else {
				m.scheduledShutdownSwitch(true, false)
			}
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	m.warnLevelConfig.setChangeCallback(m.handleBatteryDisplayUpdate)

	m.initOnBatteryChangedHandler()
	m.initSubmodules()
	m.startSubmodules()
	m.inhibitLogind()
	m.initDsg()

	so := m.service.GetServerObject(m)
	if so != nil {
		err = so.SetWriteCallback(m, "ScheduledShutdownState", func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				logger.Warning("Type is not bool")
			} else {
				logger.Info("ScheduledShutdownState change to", value)
			}
			m.setPropScheduledShutdownState(value)
			err = m.savePowerDsgConfig(dsettingScheduledShutdownState)
			return dbusutil.ToError(err)
		})
		err = so.SetWriteCallback(m, "ShutdownTime", func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(string)
			if !ok {
				logger.Warning("Type is not string")
			} else {
				logger.Info("ShutdownTime change to", value)
			}
			m.setPropShutdownTime(value)
			err = m.savePowerDsgConfig(dsettingShutdownTime)
			return dbusutil.ToError(err)
		})
		err = so.SetWriteCallback(m, "ShutdownRepetition", func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int32)
			if !ok {
				logger.Warning("Type is not int")
			} else {
				logger.Info("ShutdownRepetition change to", value)
			}
			m.setPropShutdownRepetition(int(value))
			err = m.savePowerDsgConfig(dsettingShutdownRepetition)
			return dbusutil.ToError(err)
		})
		err = so.SetWriteCallback(m, "CustomShutdownWeekDays", func(write *dbusutil.PropertyWrite) *dbus.Error {
			days := []byte{}
			for _, v := range write.Value.([]uint8) {
				days = append(days, byte(v))
			}
			m.setPropCustomShutdownWeekDays(days)
			err = m.savePowerDsgConfig(dsettingCustomShutdownWeekDays)
			return dbusutil.ToError(err)
		})
	}

	if m.UseWayland {
		m.kwinHanleIdleOffCh = make(chan bool, 10)
		go m.listenEventToHandleIdleOff()

		go func() {
			for ch := range m.kwinHanleIdleOffCh {
				if ch {
					logger.Info("kwin handle idle off")

					if v := m.submodules[submodulePSP]; v != nil {
						if psp := v.(*powerSavePlan); psp != nil {
							psp.HandleIdleOff()
						}
					}
				}
			}
		}()
	}
}

func (m *Manager) destroy() {
	m.destroySubmodules()
	m.releaseAmbientLight()
	m.permitLogind()

	if m.helper != nil {
		m.helper.Destroy()
		m.helper = nil
	}

	if m.inhibitor != nil {
		err := m.inhibitor.unblock()
		if err != nil {
			logger.Warning(err)
		}
		m.inhibitor = nil
	}

	m.systemSigLoop.Stop()
	m.sessionSigLoop.Stop()
	m.syncConfig.Destroy()
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) Reset() *dbus.Error {
	logger.Debug("Reset settings")

	var settingKeys = []string{
		settingKeyLinePowerScreenBlackDelay,
		settingKeyLinePowerSleepDelay,
		settingKeyLinePowerLockDelay,
		settingKeyLinePowerLidClosedAction,
		settingKeyLinePowerPressPowerBtnAction,

		settingKeyBatteryScreenBlackDelay,
		settingKeyBatterySleepDelay,
		settingKeyBatteryLockDelay,
		settingKeyBatteryLidClosedAction,
		settingKeyBatteryPressPowerBtnAction,

		settingKeyScreenBlackLock,
		settingKeySleepLock,
		settingKeyPowerButtonPressedExec,

		settingKeyLowPowerNotifyEnable,
		settingKeyLowPowerNotifyThreshold,
		settingKeyLowPowerAutoSleepThreshold,
		settingKeyBrightnessDropPercent,
	}
	for _, key := range settingKeys {
		logger.Debug("reset setting", key)
		m.settings.Reset(key)
	}
	return nil
}

func (m *Manager) initDsg() {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return
	}

	if !dutils.IsFileExist("/usr/share/dsg/configs/org.deepin.startdde/org.deepin.Display.json") {
		logger.Warning(" [initDsg] dconfig file not exist : /usr/share/dsg/configs/org.deepin.startdde/org.deepin.Display.json.")
		return
	}
	dsg := configManager.NewConfigManager(systemBus)

	// display
	displayConfigManagerPath, err := dsg.AcquireManager(0, DSettingsAppID, DSettingsDisplayName, "")
	if err != nil {
		logger.Warning(err)
		return
	}

	m.dsDisplayConfigManager, err = configManager.NewManager(systemBus, displayConfigManagerPath)
	if err != nil || displayConfigManagerPath == "" {
		logger.Warning(err)
	}

	// power
	powerConfigManagerPath, err := dsg.AcquireManager(0, dsettingsAppID, dsettingsPowerName, "")
	if err != nil {
		logger.Warning(err)
		return
	}
	m.dsPowerConfigManager, err = configManager.NewManager(systemBus, powerConfigManagerPath)
	if err != nil {
		logger.Warning(err)
		return
	}
	getDsPowerConfig := func(key string, init bool) {
		data, err := m.dsPowerConfigManager.Value(0, key)
		if err != nil {
			logger.Warning(err)
			return
		}
		switch key {
		case dsettingCustomShutdownWeekDays:
			res := []byte{}
			for _, v := range data.Value().([]dbus.Variant) {
				res = append(res, byte(v.Value().(float64)))
			}
			if init {
				m.CustomShutdownWeekDays = res
				return
			}
			if m.setPropCustomShutdownWeekDays(res) {
				logger.Info("Set CustomShutdownWeekDays property", m.CustomShutdownWeekDays)
			}
		case dsettingShutdownCountdown:
			m.shutdownCountdown = int(data.Value().(float64))
		case dsettingShutdownRepetition:
			if init {
				m.ShutdownRepetition = int(data.Value().(float64))
				return
			}
			if m.setPropShutdownRepetition(int(data.Value().(float64))) {
				logger.Info("Set ShutdownRepetition property", m.ShutdownRepetition)
			}
		case dsettingShutdownTime:
			if init {
				m.ShutdownTime = data.Value().(string)
				return
			}
			if m.setPropShutdownTime(data.Value().(string)) {
				logger.Info("Set ShutdownTime property", m.ShutdownTime)
			}
		case dsettingScheduledShutdownState:
			if init {
				m.ScheduledShutdownState = data.Value().(bool)
			} else {
				if m.setPropScheduledShutdownState(data.Value().(bool)) {
					logger.Info("Set ScheduledShutdownState property", m.ScheduledShutdownState)
				}
			}
		}
		m.scheduledShutdownSwitch(false, false)
		m.scheduledShutdownSwitch(m.ScheduledShutdownState, false)
	}

	getDsPowerConfig(dsettingCustomShutdownWeekDays, true)
	getDsPowerConfig(dsettingShutdownCountdown, true)
	getDsPowerConfig(dsettingShutdownRepetition, true)
	getDsPowerConfig(dsettingShutdownTime, true)
	getDsPowerConfig(dsettingScheduledShutdownState, true)
	m.dsPowerConfigManager.InitSignalExt(m.systemSigLoop, true)
	m.dsPowerConfigManager.ConnectValueChanged(func(key string) {
		logger.Info("DSG org.deepin.dde.daemon.power valueChanged, key : ", key)
		getDsPowerConfig(key, false)
	})
}

func (m *Manager) savePowerDsgConfig(key string) (err error) {
	var value interface{}
	switch key {
	case dsettingCustomShutdownWeekDays:
		var tmp []dbus.Variant
		for _, v := range m.CustomShutdownWeekDays {
			tmp = append(tmp, dbus.MakeVariant(float64(v)))
		}
		value = tmp
	case dsettingShutdownCountdown:
		value = m.shutdownCountdown
	case dsettingShutdownRepetition:
		value = m.ShutdownRepetition
	case dsettingShutdownTime:
		value = m.ShutdownTime
	case dsettingScheduledShutdownState:
		value = m.ScheduledShutdownState
	}
	err = m.setDsgData(key, value, m.dsPowerConfigManager)
	if err != nil {
		logger.Warning(err)
		return err
	}
	return nil
}

func (m *Manager) setDsgData(key string, value interface{}, dsg configManager.Manager) error {
	if dsg == nil {
		return errors.New("setDsgData dsg is nil")
	}
	err := dsg.SetValue(0, key, dbus.MakeVariant(value))
	if err != nil {
		logger.Warningf("setDsgData key : %s. err : %v", key, err)
		return err
	}
	logger.Infof("setDsgData key : %s , value : %v", key, value)
	return nil
}

func (m *Manager) scheduledShutdownSwitch(state bool, countdownContinue bool) {
	// 属性修改后需要重新检测
	m.countdowning = countdownContinue
	if !state {
		if m.shutdownTimer != nil {
			m.shutdownTimer.Stop()
			m.shutdownTimer = nil
		}
		return
	}
	if m.shutdownTimer == nil {
		m.shutdownTimer = time.NewTimer(1 * time.Second)
	} else {
		m.shutdownTimer.Reset(1 * time.Second)
	}
	go func() {
		count := m.shutdownCountdown
		playSound := true
		logger.Warning("shutdownCountdown:", m.shutdownCountdown)
		for {
			select {
			case <-m.shutdownTimer.C:
				if m.countdowning {
					if count == 0 {
						logger.Warning("shut down now!")
						m.actionShutdown(false)
						// 执行关机
						m.doAutoShutdown()
						return
					}
					m.shutdownCountdownNotify(count, playSound)
					if playSound {
						playSound = false
					}
					count--
				} else {
					m.countdowning = m.checkShutdownTrigger()
				}
			}
			if m.shutdownTimer == nil {
				m.shutdownTimer = time.NewTimer(1 * time.Second)
			} else {
				m.shutdownTimer.Reset(1 * time.Second)
			}
		}
	}()
}

func (m *Manager) actionShutdown(isCancle bool) {
	// 如果中途取消，需要根据重复选项定时重新检查
	m.scheduledShutdownSwitch(false, false)
	m.notify.CloseNotification(0, m.notifyId)
	if m.ShutdownRepetition == Once {
		m.setPropScheduledShutdownState(false)
		m.savePowerDsgConfig(dsettingScheduledShutdownState)
	}
	if isCancle {
		m.shutdownTimer = time.AfterFunc(time.Duration(m.shutdownCountdown)*time.Second, func() {
			m.shutdownTimer.Stop()
			m.shutdownTimer = nil
			m.scheduledShutdownSwitch(true, false)
		})
	}
}

func (m *Manager) shutdownCountdownNotify(count int, playSound bool) {
	body := fmt.Sprintf(gettext.Tr("The system will shut down automatically after %d s"), count)
	title := gettext.Tr("Scheduled Shutdown")
	actions := []string{"Cancle", gettext.Tr("Cancle"), "Shutdown", gettext.Tr("Shut down")}
	hints := map[string]dbus.Variant{"x-deepin-PlaySound": dbus.MakeVariant(playSound),
		"x-deepin-ShowInNotifyCenter": dbus.MakeVariant(false),
		"x-deepin-ClickToDisappear":   dbus.MakeVariant(false),
		"x-deepin-DisappearAfterLock": dbus.MakeVariant(false)}
	m.notifyIdMu.Lock()
	nid := m.notifyId
	m.notifyIdMu.Unlock()
	nid, err := m.notify.Notify(0, "dde-control-center", nid, "preferences-system", title, body, actions, hints, -1)
	if err != nil {
		logger.Warningf("failed to send notify: %s", err)
	}
	m.notifyIdMu.Lock()
	m.notifyId = nid
	m.notifyIdMu.Unlock()
	_, err = m.notify.ConnectActionInvoked(func(id uint32, actionKey string) {
		if id != m.notifyId {
			return
		}
		if actionKey == "Cancle" {
			m.actionShutdown(true)
		}
		if actionKey == "Shutdown" {
			m.actionShutdown(false)
			// 执行关机
			m.doAutoShutdown()
		}
	})
}

func (m *Manager) isWorkday(date time.Time) (res bool) {
	year, month, _ := date.Date()
	val, err := m.calendar.GetFestivalMonth(0, uint32(year), uint32(month))
	if err == nil {
		var rootCfg []FestivalRootConfig
		err = json.Unmarshal([]byte(val), &rootCfg)
		if err != nil {
			logger.Warning(err)
			return false
		}
		// 如果list不包含，则按正常周末处理，如果包含，则判断status状态
		if len(rootCfg) <= 0 {
			return date.Weekday() != time.Sunday && date.Weekday() != time.Saturday
		}
		for _, holidayInfo := range rootCfg[0].List {
			holiday, err := time.Parse("2006-1-2", holidayInfo.Date)
			if err != nil {
				logger.Warning(err)
				return false
			}
			if holiday.Equal(date) {
				return holidayInfo.Status == 2
			}
		}
		return date.Weekday() != time.Sunday && date.Weekday() != time.Saturday
	} else {
		logger.Warning(err)
	}
	return
}

func (m *Manager) checkShutdownTrigger() bool {
	// 获取当前日期
	currentTime := time.Now()

	triggerFlag := false

	switch m.ShutdownRepetition {
	case Once, Everyday:
		triggerFlag = true
	case Workdays:
		triggerFlag = m.isWorkday(currentTime)
	case Custom:
		for _, v := range m.CustomShutdownWeekDays {
			if byte(currentTime.Weekday()) == v {
				triggerFlag = true
				logger.Debug("Today is included in custom shutdown weekdays")
				break
			}
		}
	}

	if !triggerFlag {
		return false
	}
	// 构建目标时间
	targetTimeLayout := "15:04"
	targetTime, err := time.Parse(targetTimeLayout, m.ShutdownTime)
	if err != nil {
		logger.Warning("Failed to get start time:", err)
		return false
	}
	targetTime = time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), targetTime.Hour(), targetTime.Minute(), 0, 0, currentTime.Location())
	// 计算时间间隔
	currentToStart := targetTime.Sub(currentTime)
	logger.Warning("time to shut down:", int(currentToStart.Seconds()))
	// 默认提前60s发通知
	if int(currentToStart.Seconds()) >= 0 && int(currentToStart.Seconds()) <= m.shutdownCountdown {
		return true
	}
	return false
}

func (m *Manager) getAutoChangeDeepinWm() bool {
	if m.dsDisplayConfigManager == nil {
		logger.Warning("getAutoChangeDeepinWm, dsgConfig org.deepin.startdde auto-change-deepin-wm not exist")
		return false
	}
	v, err := m.dsDisplayConfigManager.Value(0, DSettingsAutoChangeWm)
	if err != nil {
		logger.Warning(err)
		return false
	}
	dsAutoChangeDeepinWm := v.Value().(bool)
	logger.Info("Auto Change Deepin Wm:", dsAutoChangeDeepinWm)

	return dsAutoChangeDeepinWm
}

func (m *Manager) setAutoChangeDeepinWm(value bool) error {
	if m.dsDisplayConfigManager == nil {
		return errors.New("setAutoChangeDeepinWm, dsgConfig org.deepin.startdde auto-change-deepin-wm not exist")
	}
	err := m.dsDisplayConfigManager.SetValue(0, DSettingsAutoChangeWm, dbus.MakeVariant(value))
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) inhibitLogind() {
	inhibit := func() {
		fd, err := m.helper.LoginManager.Inhibit(0,
			"handle-power-key:handle-lid-switch:handle-suspend-key", dbusServiceName,
			"handling key press and lid switch close", "block")
		logger.Debug("inhibitLogind fd:", fd)
		if err != nil {
			logger.Warning(err)
			return
		}
		m.inhibitFd = fd
	}
	inhibit()
	// handle login1 restart
	dbusObj := ofdbus.NewDBus(m.systemSigLoop.Conn())
	sysLoop := dbusutil.NewSignalLoop(m.systemSigLoop.Conn(), 10)
	sysLoop.Start()
	dbusObj.InitSignalExt(sysLoop, true)
	_, _ = dbusObj.ConnectNameOwnerChanged(func(name string, oldOwner string, newOwner string) {
		if name == "org.freedesktop.login1" && newOwner != "" && oldOwner == "" {
			if m.inhibitFd != -1 { // 如果之前存在inhibit时，login1重启需要重新inhibit
				err := syscall.Close(int(m.inhibitFd))
				m.inhibitFd = -1
				if err != nil {
					logger.Warning("failed to close fd:", err)
					return
				}
				inhibit()
			}
		}
	})
	// end handle login1 restart
}

func (m *Manager) permitLogind() {
	if m.inhibitFd != -1 {
		err := syscall.Close(int(m.inhibitFd))
		if err != nil {
			logger.Warning("failed to close inhibitFd:", err)
		}
		m.inhibitFd = -1
	}
}

func (m *Manager) SetPrepareSuspend(suspendState int) *dbus.Error {
	m.setPrepareSuspend(suspendState)
	return nil
}

func (m *Manager) isSessionActive() bool {
	active, err := m.currentSession.Active().Get(dbus.FlagNoAutoStart)
	if err != nil {
		logger.Error("failed to get session active status:", err)
		return false
	}
	return active
}

// wayland下在收到键盘或者鼠标事件后，需要进行系统空闲处理（主要是唤醒屏幕）
func (m *Manager) listenEventToHandleIdleOff() error {
	// + 监控按键事件
	err := m.systemSigLoop.Conn().Object("com.deepin.daemon.Gesture",
		"/com/deepin/daemon/Gesture").AddMatchSignal("com.deepin.daemon.Gesture", "KeyboardEvent").Err
	if err != nil {
		logger.Warning(err)
		return err
	}

	m.systemSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "com.deepin.daemon.Gesture.KeyboardEvent",
	}, func(sig *dbus.Signal) {
		if len(sig.Body) > 1 {
			value := sig.Body[1].(uint32)
			if m.getDPMSMode() != dpmsStateOn && value == 1 {
				logger.Debug("receive keyboard event to handle idle off")
				m.kwinHanleIdleOffCh <- true
			}
		}
	})

	// + 监控鼠标移动事件
	err = m.sessionSigLoop.Conn().Object("com.deepin.daemon.KWayland",
		"/com/deepin/daemon/KWayland/Output").AddMatchSignal("com.deepin.daemon.KWayland.Output", "CursorMove").Err
	if err != nil {
		logger.Warning(err)
		return err
	}
	m.sessionSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "com.deepin.daemon.KWayland.Output.CursorMove",
	}, func(sig *dbus.Signal) {
		if len(sig.Body) > 1 {
			if m.getDPMSMode() != dpmsStateOn {
				logger.Debug("receive cursor move event to handle idle off")
				m.kwinHanleIdleOffCh <- true
			}
		}
	})

	// + 监控鼠标按下事件
	err = m.sessionSigLoop.Conn().Object("com.deepin.daemon.KWayland",
		"/com/deepin/daemon/KWayland/Output").AddMatchSignal("com.deepin.daemon.KWayland.Output", "ButtonPress").Err
	if err != nil {
		logger.Warning(err)
		return err
	}
	m.sessionSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "com.deepin.daemon.KWayland.Output.ButtonPress",
	}, func(sig *dbus.Signal) {
		if len(sig.Body) > 1 {
			if m.getDPMSMode() != dpmsStateOn {
				logger.Debug("acquire button press to handle idle off")
				m.kwinHanleIdleOffCh <- true
			}
		}
	})

	return nil
}
