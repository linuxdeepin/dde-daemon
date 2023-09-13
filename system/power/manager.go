// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-api/powersupply"
	"github.com/linuxdeepin/dde-api/powersupply/battery"
	"github.com/linuxdeepin/dde-daemon/common/cpuinfo"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	gudev "github.com/linuxdeepin/go-gir/gudev-1.0"
	"github.com/linuxdeepin/go-lib/arch"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
	"github.com/linuxdeepin/go-lib/utils"
)

var noUEvent bool

const (
	configManagerId                   = "org.desktopspec.ConfigManager"
	_configHwSystem                   = "/usr/share/uos-hw-config"
	intelPstatePath                   = "/sys/devices/system/cpu/intel_pstate"
	amdPstatePath                     = "/sys/devices/system/cpu/amd_pstate"
	amdGPUPath                        = "/sys/class/drm/card0/device/power_dpm_force_performance_level"
	configDefaultGovFilePattern       = `config-[0-9\.]+-.*-desktop`
	configDefaultGovContentDDEPattern = `^\s*CONFIG_CPU_FREQ_DEFAULT_GOV_DDE_(\w+)`
	configDefaultGovContentPattern    = `^\s*CONFIG_CPU_FREQ_DEFAULT_GOV_(\w+)`
)

const (
	dsettingsAppID                                = "org.deepin.dde.daemon"
	dsettingsPowerName                            = "org.deepin.dde.daemon.power"
	dsettingsTlpName                              = "org.deepin.dde.daemon.power.tlp"
	dsettingsBluetoothAdapterName                 = "org.deepin.dde.daemon.power.bluetoothAdapter"
	dsettingsSpecialCpuModeJson                   = "specialCpuModeJson"
	dsettingsBalanceCpuGovernor                   = "BalanceCpuGovernor"
	dsettingsIsFirstGetCpuGovernor                = "isFirstGetCpuGovernor"
	dsettingsIdlePowersaveAspmEnabled             = "idlePowersaveAspmEnabled"
	dsettingsPowerSavingModeEnabled               = "powerSavingModeEnabled"
	dsettingsPowerSavingModeAuto                  = "powerSavingModeAuto"
	dsettingsPowerSavingModeAutoWhenBatteryLow    = "powerSavingModeAutoWhenBatteryLow"
	dsettingsPowerSavingModeBrightnessDropPercent = "powerSavingModeBrightnessDropPercent"
	dsettingsMode                                 = "mode"
	dsettingsSupportCpuGovernors                  = "supportCpuGovernors"
)

type supportMode struct {
	Balance    bool `json:"balance"`
	Performace bool `json:"performace"`
	PowerSave  bool `json:"powersave"`
}

func init() {
	if arch.Get() == arch.Sunway {
		noUEvent = true
	}
}

//go:generate dbusutil-gen -type Manager,Battery -import github.com/linuxdeepin/dde-api/powersupply/battery manager.go battery.go
//go:generate dbusutil-gen em -type Manager,Battery

// https://www.kernel.org/doc/Documentation/power/power_supply_class.txt
type Manager struct {
	service       *dbusutil.Service
	systemSigLoop *dbusutil.SignalLoop
	batteries     map[string]*Battery
	batteriesMu   sync.Mutex
	ac            *AC
	gudevClient   *gudev.Client
	dsgPower      ConfigManager.Manager

	// 电池是否电量低
	batteryLow bool
	// 初始化是否完成
	initDone bool

	// CPU操作接口
	cpus *CpuHandlers
	// INFO: pstate中，可选项是performance balance_performance balance_power power
	hasPstate bool
	hasAmddpm bool

	PropsMu      sync.RWMutex
	OnBattery    bool
	HasLidSwitch bool
	// battery display properties:
	HasBattery         bool
	BatteryPercentage  float64
	BatteryStatus      battery.Status
	BatteryTimeToEmpty uint64
	BatteryTimeToFull  uint64
	// 电池容量
	BatteryCapacity float64

	// 开启和关闭节能模式
	PowerSavingModeEnabled bool `prop:"access:rw"`

	// 自动切换节能模式，依据为是否插拔电源
	PowerSavingModeAuto bool `prop:"access:rw"`

	// 低电量时自动开启
	PowerSavingModeAutoWhenBatteryLow bool `prop:"access:rw"`

	// 开启节能模式时降低亮度的百分比值
	PowerSavingModeBrightnessDropPercent uint32 `prop:"access:rw"`

	// 开启节能模式时保存的数据
	PowerSavingModeBrightnessData string `prop:"access:rw"`

	// CPU频率调节模式，支持powersave和performance
	CpuGovernor string

	// CPU频率增强是否开启
	CpuBoost bool

	// 是否支持Boost
	IsHighPerformanceSupported bool

	// 是否支持平衡模式
	IsBalanceSupported bool

	// 是否支持节能模式
	IsPowerSaveSupported bool

	// 性能模式-平衡模式dsg
	balanceScalingGovernor string
	configManagerPath      dbus.ObjectPath

	// 当前模式
	Mode string

	//是否在启动阶段，启动阶段不允许调节亮度; 若在启动阶段切换模式后(切节能模式降低亮度)，可以调节亮度.
	IsInBootTime bool

	// 上次非低电量时的模式
	lastMode string

	// 退出函数
	endSysPowerSave func()

	// powersave aspm 状态
	idlePowersaveAspmEnabled bool
	loginManager             login1.Manager

	// 蓝牙适配器状态
	bluetoothAdapterEnabledCmd string
	bluetoothAdapterScanCmd string

	// Special Cpu suppoert mode
	specialCpuMode *supportMode

	// nolint
	signals *struct {
		BatteryDisplayUpdate struct {
			timestamp int64
		}

		BatteryAdded struct {
			path dbus.ObjectPath
		}

		BatteryRemoved struct {
			path dbus.ObjectPath
		}

		LidClosed struct{}
		LidOpened struct{}
	}
}

func newManager(service *dbusutil.Service) (*Manager, error) {
	m := &Manager{
		service:           service,
		BatteryPercentage: 100,
		cpus:              NewCpuHandlers(),
		lastMode:          "balance",
	}
	// check pstate , if has pstate, it is intel pstate mode , then
	// we need another logic
	m.hasPstate = utils.IsFileExist(intelPstatePath) || utils.IsFileExist(amdPstatePath)
	// check if amd is used
	m.hasAmddpm = utils.IsFileExist(amdGPUPath)

	m.refreshSystemPowerPerformance()
	if m.hasPstate {
		m.balanceScalingGovernor = "balance_performance"
	}
	err := m.init()
	if err != nil {
		m.destroy()
		return nil, err
	}

	return m, nil
}

type AC struct {
	gudevClient *gudev.Client
	sysfsPath   string
}

func newAC(manager *Manager, device *gudev.Device) *AC {
	sysfsPath := device.GetSysfsPath()
	return &AC{
		gudevClient: manager.gudevClient,
		sysfsPath:   sysfsPath,
	}
}

func (ac *AC) newDevice() *gudev.Device {
	return ac.gudevClient.QueryBySysfsPath(ac.sysfsPath)
}

func (m *Manager) refreshAC(ac *gudev.Device) { // 拔插电源时候触发
	online := ac.GetPropertyAsBoolean("POWER_SUPPLY_ONLINE")
	logger.Debug("ac online:", online)
	onBattery := !online

	m.PropsMu.Lock()
	m.setPropOnBattery(onBattery)
	m.PropsMu.Unlock()
	// 根据OnBattery的状态,修改节能模式
	m.updatePowerSavingMode()
}

func (m *Manager) initAC(devices []*gudev.Device) {
	var ac *gudev.Device
	for _, dev := range devices {
		if powersupply.IsMains(dev) {
			ac = dev
			break
		}
	}
	if ac != nil {
		m.refreshAC(ac)
		m.ac = newAC(m, ac)

		if noUEvent {
			go func() {
				c := time.Tick(2 * time.Second)
				for range c {
					err := m.RefreshMains()
					if err != nil {
						logger.Warning(err)
					}
				}
			}()
		}
	}
}

func (m *Manager) init() error {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	m.systemSigLoop = dbusutil.NewSignalLoop(systemBus, 10)
	err = m.initDsgConfig()
	if err != nil {
		logger.Warning(err)
	}
	err = m.initTlpDsgConfig()
	if err != nil {
		logger.Warning(err)
	}
	err = m.initBluetoothAdapterDsgConfig()
	if err != nil {
		logger.Warning(err)
	}
	m.systemSigLoop.Start()

	subsystems := []string{"power_supply", "input"}
	m.gudevClient = gudev.NewClient(subsystems)
	if m.gudevClient == nil {
		return errors.New("gudevClient is nil")
	}

	m.initLidSwitch()
	devices := powersupply.GetDevices(m.gudevClient)

	cfg := loadConfigSafe()
	if cfg != nil {
		// 将config.json中的配置完成初始化
		m.setPowerSavingModeEnabled(cfg.PowerSavingModeEnabled)                           // 开启和关闭节能模式
		m.PowerSavingModeAuto = cfg.PowerSavingModeAuto                                   // 自动切换节能模式，依据为是否插拔电源
		m.PowerSavingModeAutoWhenBatteryLow = cfg.PowerSavingModeAutoWhenBatteryLow       // 低电量时自动开启
		m.PowerSavingModeBrightnessDropPercent = cfg.PowerSavingModeBrightnessDropPercent // 开启节能模式时降低亮度的百分比值
		m.Mode = cfg.Mode
		migrateErr := m.migrateFromCurrentConfigsToDsg(cfg)
		if migrateErr != nil {
			logger.Error("migrateFromCurrentConfigsToDsg failed, err:", migrateErr)
		}
	}

	logger.Info(" ## init(second) m.Mode : ", m.Mode)
	// 恢复配置
	err = m.doSetMode(m.Mode)
	if err != nil {
		logger.Warning(err)
	} else {
		logger.Debugf("init mode %s", m.Mode)
	}

	m.initAC(devices)
	m.initBatteries(devices)
	for _, dev := range devices {
		dev.Unref()
	}

	m.gudevClient.Connect("uevent", m.handleUEvent)
	m.initDone = true
	// init LMT config
	m.updatePowerSavingMode()

	m.CpuBoost, err = m.cpus.GetBoostEnabled()
	if err != nil {
		logger.Warning(err)
	}

	m.CpuGovernor, err = m.cpus.GetGovernor()
	if err != nil {
		logger.Warning(err)
	}
	m.loginManager = login1.NewManager(m.service.Conn())
	m.loginManager.InitSignalExt(m.systemSigLoop, true)
	return nil
}

func (m *Manager) initDsgConfig() error {
	logger.Info("initDsgConfig.")
	// dsg 配置
	ds := ConfigManager.NewConfigManager(m.systemSigLoop.Conn())
	dsPowerPath, err := ds.AcquireManager(0, dsettingsAppID, dsettingsPowerName, "")
	if err != nil {
		return err
	}
	dsPower, err := ConfigManager.NewManager(m.systemSigLoop.Conn(), dsPowerPath)
	if err != nil {
		return err
	}
	m.dsgPower = dsPower

	getSpecialCpuMode := func() {
		// 获取cpu Hardware
		cpuinfo, err := cpuinfo.ReadCPUInfo("/proc/cpuinfo")
		if err != nil {
			logger.Warning("ReadCPUInfo /proc/cpuinfo err : ", err)
			return
		}

		data, err := dsPower.Value(0, dsettingsSpecialCpuModeJson)
		if err != nil {
			logger.Warning(err)
			return
		}

		str := data.Value().(string)
		modeMap := make(map[string]*supportMode)
		if err := json.Unmarshal([]byte(str), &modeMap); err != nil {
			logger.Warning(err)
		}
		for k, v := range modeMap {
			if strings.HasPrefix(cpuinfo.Hardware, k) {
				logger.Info("use special cpu mode", v)
				m.specialCpuMode = v
				return
			}
		}
	}

	getIdlePowersaveAspmEnabled := func() {
		data, err := dsPower.Value(0, dsettingsIdlePowersaveAspmEnabled)
		if err != nil {
			logger.Warning(err)
			return
		}

		m.idlePowersaveAspmEnabled = data.Value().(bool)
		logger.Info("Set idle powersave aspm enabled", m.idlePowersaveAspmEnabled)
	}

	getPowerSavingModeAuto := func(init bool) {
		data, err := dsPower.Value(0, dsettingsPowerSavingModeAuto)
		if err != nil {
			logger.Warning(err)
			return
		}

		if init {
			m.PowerSavingModeAuto = data.Value().(bool)
			return
		}

		if m.setPropPowerSavingModeAuto(data.Value().(bool)) {
			logger.Info("Set power saving mode auto", m.PowerSavingModeAuto)
			m.updatePowerSavingMode()
		}
	}

	getPowerSavingModeEnabled := func(init bool) {
		data, err := dsPower.Value(0, dsettingsPowerSavingModeEnabled)
		if err != nil {
			logger.Warning(err)
			return
		}

		if init {
			m.setPowerSavingModeEnabled(data.Value().(bool))
			return
		}

		if m.setPowerSavingModeEnabled(data.Value().(bool)) {
			logger.Info("Set power saving mode enable", m.PowerSavingModeEnabled)
		}
	}

	getPowerSavingModeAutoWhenBatteryLow := func(init bool) {
		data, err := dsPower.Value(0, dsettingsPowerSavingModeAutoWhenBatteryLow)
		if err != nil {
			logger.Warning(err)
			return
		}

		if init {
			m.setPowerSavingModeEnabled(data.Value().(bool))
			return
		}

		if m.setPropPowerSavingModeAutoWhenBatteryLow(data.Value().(bool)) {
			logger.Info("Set power saving mode auto when battery low", m.PowerSavingModeAutoWhenBatteryLow)
			m.updatePowerSavingMode()
		}
	}

	getPowerSavingModeBrightnessDropPercent := func(init bool) {
		data, err := dsPower.Value(0, dsettingsPowerSavingModeBrightnessDropPercent)
		if err != nil {
			logger.Warning(err)
			return
		}

		if init {
			switch vv := data.Value().(type) {
			case float64:
				m.PowerSavingModeBrightnessDropPercent = uint32(vv)
			case int64:
				m.PowerSavingModeBrightnessDropPercent = uint32(vv)
			default:
				logger.Warning("type is wrong! type : ", vv)
			}

			return
		}

		ret := false
		switch vv := data.Value().(type) {
		case float64:
			ret = m.setPropPowerSavingModeBrightnessDropPercent(uint32(vv))
		case int64:
			ret = m.setPropPowerSavingModeBrightnessDropPercent(uint32(vv))
		default:
			logger.Warning("type is wrong! type : ", vv)
		}
		if ret {
			logger.Info("Set power saving mode brightness drop percent", m.PowerSavingModeBrightnessDropPercent)
		}
	}

	getMode := func(init bool) {
		ret, err := dsPower.Value(0, dsettingsMode)
		if err != nil {
			logger.Warning(err)
			return
		}

		value := ret.Value().(string)

		if init {
			m.Mode = value
			return
		}

		//dsg更新配置后，校验mode有效性
		if value == "balance" || value == "powersave" || value == "performance" {
			logger.Info(" ---- update DsgConfig mode : ", value)
			err = m.doSetMode(value)
			if err != nil {
				logger.Warning(err)
				return
			}
		} else {
			logger.Warningf("invalid dsg of mode : %s. Please use balance/powersave/performance", value)
			expectValue := "balance"
			//非法制需要强制设置成有效值
			if m.Mode == "balance" || m.Mode == "powersave" || m.Mode == "performance" {
				expectValue = m.Mode
			} else {
				// dsg获取非法数据，设置成平衡模式
				expectValue = "balance"
			}
			err = m.doSetMode(expectValue)
			if err != nil {
				logger.Warning(err)
				return
			}
			m.setDsgData(dsettingsMode, expectValue, dsPower)
		}
	}

	getSpecialCpuMode()
	getIdlePowersaveAspmEnabled()
	getPowerSavingModeAuto(true)
	getPowerSavingModeEnabled(true)
	getPowerSavingModeAutoWhenBatteryLow(true)
	getPowerSavingModeBrightnessDropPercent(true)
	getMode(true)

	dsPower.InitSignalExt(m.systemSigLoop, true)
	dsPower.ConnectValueChanged(func(key string) {
		logger.Info("DSG org.deepin.dde.daemon.power valueChanged, key : ", key)
		switch key {
		case dsettingsSupportCpuGovernors:
			data, err := dsPower.Value(0, dsettingsSupportCpuGovernors)
			if err != nil {
				logger.Warning(err)
				return
			}
			// 当supportCpuGovernors被人为改动后，就和availableArrGovernors比较，如果不同则直接设置成availableArrGovernors数据
			cpuGovernors := interfaceToArrayString(data.Value().(string))
			availableArrGovernors := getLocalAvailableGovernors()
			if len(cpuGovernors) != len(availableArrGovernors) {
				m.setDsgData(dsettingsSupportCpuGovernors, availableArrGovernors, dsPower)
				logger.Info("Modification availableGovernors not allowed.")
			} else {
				for _, v := range cpuGovernors {
					if !strv.Strv(availableArrGovernors).Contains(v.(string)) {
						m.setDsgData(dsettingsSupportCpuGovernors, availableArrGovernors, dsPower)
						logger.Info("Modification availableGovernors not allowed.")
						return
					}
				}
			}
		case dsettingsPowerSavingModeAuto:
			getPowerSavingModeAuto(false)
		case dsettingsPowerSavingModeEnabled:
			getPowerSavingModeEnabled(false)
		case dsettingsPowerSavingModeAutoWhenBatteryLow:
			getPowerSavingModeAutoWhenBatteryLow(false)
		case dsettingsPowerSavingModeBrightnessDropPercent:
			getPowerSavingModeBrightnessDropPercent(false)
		case dsettingsIdlePowersaveAspmEnabled:
			getIdlePowersaveAspmEnabled()
		case dsettingsMode:
			getMode(false)

		default:
			logger.Debug("Not process. valueChanged, key : ", key)
		}
	})

	return err
}

func (m *Manager) refreshSystemPowerPerformance() { // 获取系统支持的性能模式
	path := m.cpus.getCpuGovernorPath(m.hasPstate)

	if path == "" {
		m.IsHighPerformanceSupported = false
		m.IsBalanceSupported = false
		m.IsPowerSaveSupported = false
		return
	}

	re := regexp.MustCompile(configDefaultGovFilePattern)
	if result, err := utils.FindFileInDirRegexp("/boot/", re); err != nil {
		logger.Warning(err)
	} else if len(result) > 0 {
		re = regexp.MustCompile(configDefaultGovContentDDEPattern)
		_, matches, err := utils.FindContentInFileRegexp(result[0], re)
		if err != nil {
			logger.Warning(err)
			re = regexp.MustCompile(configDefaultGovContentPattern)
			_, matches, err = utils.FindContentInFileRegexp(result[0], re)
			if err != nil {
				logger.Warning(err)
			}
		}
		if len(matches) > 1 {
			m.balanceScalingGovernor = strings.ToLower(matches[1])
		}
	}

	if m.balanceScalingGovernor == "" {
		logger.Warning("can't find balance scaling governor use ondemand")
		m.balanceScalingGovernor = "ondemand"
	}

	logger.Info(" ## refreshSystemPowerPerformance balance ScalingGovernor : ", m.balanceScalingGovernor)

	if m.specialCpuMode != nil {
		m.IsBalanceSupported = m.specialCpuMode.Balance
		m.IsHighPerformanceSupported = m.specialCpuMode.Performace
		m.IsPowerSaveSupported = m.specialCpuMode.PowerSave
	} else {
		logger.Info("check available governor")
		m.IsBalanceSupported = true
		ret := m.cpus.tryWriteGovernor(strv.Strv{"performance", "powersave"}, m.hasPstate)
		if ret.Contains("performance") {
			m.IsHighPerformanceSupported = true
		}
		if ret.Contains("powersave") {
			m.IsPowerSaveSupported = true
		}
		if ret.Contains("power") && m.hasPstate {
			m.IsPowerSaveSupported = true
		}
	}

	logger.Info(" init end. m.balanceScalingGovernor : ", m.balanceScalingGovernor)
}

func (m *Manager) handleUEvent(client *gudev.Client, action string, device *gudev.Device) {
	logger.Debug("on uevent action:", action)
	defer device.Unref()

	switch action {
	case "change":
		if powersupply.IsMains(device) {
			if m.ac == nil {
				m.ac = newAC(m, device)
			} else if m.ac.sysfsPath != device.GetSysfsPath() {
				logger.Warning("found another AC", device.GetSysfsPath())
				return
			}

			// now m.ac != nil, and sysfsPath equal
			m.refreshAC(device)
			time.AfterFunc(1*time.Second, m.refreshBatteries)
			time.AfterFunc(3*time.Second, m.refreshBatteries)
			// 电源状态变更时，需要一段时间才能稳定，因此在1分钟内，每隔5秒刷新一次，保证数据及时更新
			for i := 1; i <= 12; i++ {
				time.AfterFunc(time.Duration(i*5)*time.Second, m.refreshBatteries)
			}
		} else if powersupply.IsSystemBattery(device) {
			m.addAndExportBattery(device)
		}
	case "add":
		if powersupply.IsSystemBattery(device) {
			m.addAndExportBattery(device)
		}
		// ignore add mains

	case "remove":
		if powersupply.IsSystemBattery(device) {
			m.removeBattery(device)
		}
	}

}

func (m *Manager) initBatteries(devices []*gudev.Device) {
	m.batteries = make(map[string]*Battery)
	for _, dev := range devices {
		m.addBattery(dev)
	}
	logger.Debugf("initBatteries done %#v", m.batteries)
}

func (m *Manager) addAndExportBattery(dev *gudev.Device) {
	bat, added := m.addBattery(dev)
	if added {
		err := m.service.Export(bat.getObjPath(), bat)
		if err == nil {
			m.emitBatteryAdded(bat)
		} else {
			logger.Warning("failed to export battery:", err)
		}
	}
}

func (m *Manager) addBattery(dev *gudev.Device) (*Battery, bool) {
	logger.Debug("addBattery dev:", dev)
	if !powersupply.IsSystemBattery(dev) {
		return nil, false
	}

	sysfsPath := dev.GetSysfsPath()
	logger.Debug(sysfsPath)

	m.batteriesMu.Lock()
	bat, ok := m.batteries[sysfsPath]
	m.batteriesMu.Unlock()
	if ok {
		logger.Debugf("add battery failed , sysfsPath exists %q", sysfsPath)
		bat.Refresh()
		return bat, false
	}

	bat = newBattery(m, dev)
	if bat == nil {
		logger.Debugf("add batteries failed, sysfsPath %q, new battery failed", sysfsPath)
		return nil, false
	}

	m.batteriesMu.Lock()
	m.batteries[sysfsPath] = bat
	m.refreshBatteryDisplay()
	m.batteriesMu.Unlock()
	bat.setRefreshDoneCallback(m.refreshBatteryDisplay)
	return bat, true
}

// removeBattery remove the battery from Manager.batteries, and stop export it.
func (m *Manager) removeBattery(dev *gudev.Device) {
	sysfsPath := dev.GetSysfsPath()

	m.batteriesMu.Lock()
	bat, ok := m.batteries[sysfsPath]
	m.batteriesMu.Unlock()

	if ok {
		logger.Info("removeBattery", sysfsPath)
		m.batteriesMu.Lock()
		delete(m.batteries, sysfsPath)
		m.refreshBatteryDisplay()
		m.batteriesMu.Unlock()

		err := m.service.StopExport(bat)
		if err != nil {
			logger.Warning(err)
		}
		m.emitBatteryRemoved(bat)

		bat.destroy()
	} else {
		logger.Warning("removeBattery failed: invalid sysfsPath ", sysfsPath)
	}
}

func (m *Manager) emitBatteryAdded(bat *Battery) {
	err := m.service.Emit(m, "BatteryAdded", bat.getObjPath())
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) emitBatteryRemoved(bat *Battery) {
	err := m.service.Emit(m, "BatteryRemoved", bat.getObjPath())
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) destroy() {
	logger.Debug("destroy")
	m.batteriesMu.Lock()
	for _, bat := range m.batteries {
		bat.destroy()
	}
	m.batteries = nil
	m.batteriesMu.Unlock()

	if m.gudevClient != nil {
		m.gudevClient.Unref()
		m.gudevClient = nil
	}
	m.systemSigLoop.Stop()
}

const configFile = "/var/lib/dde-daemon/power/config.json"

type Config struct {
	PowerSavingModeEnabled               bool
	PowerSavingModeAuto                  bool
	PowerSavingModeAutoWhenBatteryLow    bool
	PowerSavingModeBrightnessDropPercent uint32
	Mode                                 string
}

func loadConfig() (*Config, error) {
	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(content, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func loadConfigSafe() *Config {
	cfg, err := loadConfig()
	if err != nil {
		// ignore not exist error
		if !os.IsNotExist(err) {
			logger.Warning(err)
		}
		return nil
	}
	// 新增字段后第一次启动时,缺少两个新增字段的json,导致亮度下降百分比字段默认为0,导致与默认值不符,需要处理
	// 低电量自动待机字段的默认值为false,不会导致错误影响
	// 正常情况下该字段范围为10-40,只有在该情况下会出现0的可能
	if cfg.PowerSavingModeBrightnessDropPercent == 0 {
		cfg.PowerSavingModeBrightnessDropPercent = 20
	}

	if cfg.Mode == "" {
		cfg.Mode = "balance"
	}
	return cfg
}

func (m *Manager) migrateFromCurrentConfigsToDsg(cfg *Config) error {
	err := m.saveDsgConfig("")
	if err != nil {
		logger.Warning("saveDsgConfig failed", err)
		return err
	}

	// 迁移完成后，删除本地配置文件
	err = os.Remove(configFile)
	if err != nil {
		logger.Warning("delete local configs file failed", err)
		return err
	}

	return nil
}

func (m *Manager) saveDsgConfig(value string) (err error) {
	switch value {
	case "PowerSavingModeBrightnessDropPercent":
		err = m.setDsgData(dsettingsPowerSavingModeBrightnessDropPercent, int64(m.PowerSavingModeBrightnessDropPercent), m.dsgPower)
		if err != nil {
			return err
		}
	case "PowerSavingModeAutoWhenBatteryLow":
		err = m.setDsgData(dsettingsPowerSavingModeAutoWhenBatteryLow, m.PowerSavingModeAutoWhenBatteryLow, m.dsgPower)
		if err != nil {
			return err
		}
	case "PowerSavingModeEnabled":
		err = m.setDsgData(dsettingsPowerSavingModeEnabled, m.PowerSavingModeEnabled, m.dsgPower)
		if err != nil {
			return err
		}
	case "PowerSavingModeAuto":
		err = m.setDsgData(dsettingsPowerSavingModeAuto, m.PowerSavingModeAuto, m.dsgPower)
		if err != nil {
			return err
		}
	case "":
		err = m.setDsgData(dsettingsPowerSavingModeBrightnessDropPercent, int64(m.PowerSavingModeBrightnessDropPercent), m.dsgPower)
		if err != nil {
			return err
		}
		err = m.setDsgData(dsettingsPowerSavingModeAutoWhenBatteryLow, m.PowerSavingModeAutoWhenBatteryLow, m.dsgPower)
		if err != nil {
			return err
		}
		err = m.setDsgData(dsettingsPowerSavingModeEnabled, m.PowerSavingModeEnabled, m.dsgPower)
		if err != nil {
			return err
		}
		err = m.setDsgData(dsettingsPowerSavingModeAuto, m.PowerSavingModeAuto, m.dsgPower)
		if err != nil {
			return err
		}
	}

	return m.setDsgData(dsettingsMode, m.Mode, m.dsgPower)
}

func (m *Manager) doSetMode(mode string) error {
	logger.Info(" doSetMode, mode : ", mode)
	var err error
	switch mode {
	case "balance": // governor=performance
		if m.hasAmddpm {
			err := ioutil.WriteFile(amdGPUPath, []byte("auto"), 0644)
			if err != nil {
				logger.Warning(err)
			}
		}
		if !m.IsBalanceSupported {
			err = dbusutil.MakeErrorf(m, "PowerMode", "%q mode is not supported", mode)
			break
		}

		m.setPowerSavingModeEnabled(false)
		err = m.doSetCpuGovernor(m.balanceScalingGovernor)
		if err != nil {
			logger.Warning(err)
			mode := "powersave"
			if m.hasPstate {
				mode = "power"
			}
			logger.Infof("set %s mode failed, try to set %s mode", m.balanceScalingGovernor, mode)
			err = m.doSetCpuGovernor(mode)
			if err != nil {
				logger.Warning(err)
			}
		}
	case "powersave": // governor=powersave
		if m.hasAmddpm {
			err := ioutil.WriteFile(amdGPUPath, []byte("low"), 0644)
			if err != nil {
				logger.Warning(err)
			}
		}
		if !m.IsPowerSaveSupported {
			err = dbusutil.MakeErrorf(m, "PowerMode", "%q mode is not supported", mode)
			break
		}

		m.setPowerSavingModeEnabled(true)
		if m.hasPstate {
			err = m.doSetCpuGovernor("power")
		} else {
			err = m.doSetCpuGovernor("powersave")
		}
		if err != nil {
			logger.Warning(err)
		}

	case "performance": // governor=performance
		if m.hasAmddpm {
			err := ioutil.WriteFile(amdGPUPath, []byte("high"), 0644)
			if err != nil {
				logger.Warning(err)
			}
		}
		if !m.IsHighPerformanceSupported {
			err = dbusutil.MakeErrorf(m, "PowerMode", "%q mode is not supported", mode)
			break
		}

		m.setPowerSavingModeEnabled(false)
		err = m.doSetCpuGovernor("performance")
		if err != nil {
			logger.Warning(err)
		}

	default:
		err = dbusutil.MakeErrorf(m, "PowerMode", "%q mode is not supported", mode)
	}
	if mode == "powersave" {
		m.startSysPowersave()
	} else if m.endSysPowerSave != nil {
		m.endSysPowerSave()
	}

	if err == nil {
		if m.setPropMode(mode) {
			logger.Info("Set power mode", m.Mode)
			m.IsInBootTime = false
			m.setDsgData(dsettingsMode, mode, m.dsgPower)
		} else {
			logger.Warningf("Set power mode failed. mode : %s, current mode : %s", mode, m.Mode)
		}
		if m.lastMode != mode && mode != "powersave" {
			m.lastMode = mode
		}
	}

	return err
}

func (m *Manager) doSetCpuBoost(enabled bool) error {
	if m.specialCpuMode != nil {
		logger.Info("=== special cpu mode is on ===")
		return nil
	}

	err := m.cpus.SetBoostEnabled(enabled)
	if err == nil {
		m.setPropCpuBoost(enabled)
	}
	return err
}

func (m *Manager) doSetCpuGovernor(governor string) error {
	if m.specialCpuMode != nil {
		logger.Info("=== special cpu mode is on ===")
		return nil
	}
	err := m.cpus.SetGovernor(governor, m.hasPstate)

	if err == nil {
		m.setPropCpuGovernor(governor)
	}
	return err
}

func (m *Manager) startSysPowersave() {
	// echo powersave > /sys/module/pcie_aspm/parameters/policy
	// echo low > /sys/class/drm/card0/device/power_dpm_force_performance_level
	policy := ""
	level := getPowerDpmForcePerformanceLevel()

	if m.idlePowersaveAspmEnabled {
		policy = getPowerPolicy()
	}

	logger.Info("start system power save [policy: powersave, level: low]")
	var err error
	if policy != "" {
		err = setPowerPolicy("powersave")
		if err != nil {
			logger.Warning(err)
		}
	}
	err = setPowerDpmForcePerformanceLevel("low")
	if err != nil {
		logger.Warning(err)
	}

	m.endSysPowerSave = func() {
		logger.Infof("end system power save [policy: %s, level: %s]", policy, level)
		if policy != "" {
			err = setPowerPolicy(policy)
			if err != nil {
				logger.Warning(err)
			}
		}
		err = setPowerDpmForcePerformanceLevel(level)
		if err != nil {
			logger.Warning(err)
		}
	}
}

//需求: 为了提高启动速度，登录前将性能模式设置为performance
//① 为了减小耦合性，仅写文件(doSetCpuGovernor)，不修改后端相关属性
func (m *Manager) enablePerformanceInBoot() bool {
	m.IsInBootTime = true
	if m.Mode == "performance" {
		return false
	}

	logger.Info("enablePerformanceInBoot performance")
	err := m.doSetCpuGovernor("performance")
	if err != nil {
		logger.Warning(err)
		return false
	}
	return true
}

func (m *Manager) setPowerSavingModeEnabled(enable bool) (changed bool) {
	changed = m.setPropPowerSavingModeEnabled(enable)
	err := m.writePowerSavingModeEnabledCbImpl(enable)
	if err != nil {
		logger.Warning(err)
	}
	err = m.saveDsgConfig("PowerSavingModeEnabled")
	if err != nil {
		logger.Warning(err)
	}
	return
}
