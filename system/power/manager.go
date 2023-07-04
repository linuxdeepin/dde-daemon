// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
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
	pstatePath                        = "/sys/devices/system/cpu/intel_pstate"
	amdGPUPath                        = "/sys/class/drm/card0/device/power_dpm_force_performance_level"
	configDefaultGovFilePattern       = `config-[0-9\.]+-.*-desktop`
	configDefaultGovContentDDEPattern = `^\s*CONFIG_CPU_FREQ_DEFAULT_GOV_DDE_(\w+)`
	configDefaultGovContentPattern    = `^\s*CONFIG_CPU_FREQ_DEFAULT_GOV_(\w+)`
)

const (
	dsettingsAppID                                = "org.deepin.dde.daemon"
	dsettingsPowerName                            = "org.deepin.dde.daemon.power"
	dsettingsSpecialCpuModeJson                   = "specialCpuModeJson"
	dsettingsBalanceCpuGovernor                   = "BalanceCpuGovernor"
	dsettingsIsFirstGetCpuGovernor                = "isFirstGetCpuGovernor"
	dsettingsIdlePowersaveAspmEnabled             = "idlePowersaveAspmEnabled"
	dsettingsPowerSavingModeEnabled               = "powerSavingModeEnabled"
	dsettingsPowerSavingModeAuto                  = "powerSavingModeAuto"
	dsettingsPowerSavingModeAutoWhenBatteryLow    = "powerSavingModeAutoWhenBatteryLow"
	dsettingsPowerSavingModeBrightnessDropPercent = "powerSavingModeBrightnessDropPercent"
	dsettingsMode                                 = "mode"
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

	// 退出函数
	endSysPowerSave func()

	// powersave aspm 状态
	idlePowersaveAspmEnabled bool

	// 性能模式的表现模式，不一定和当前实际设置的一致，但和用户期望的模式一致
	fakeMode     string
	loginManager login1.Manager

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
	}
	// check pstate , if has pstate, it is intel pstate mode , then
	// we need another logic
	m.hasPstate = utils.IsFileExist(pstatePath)
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
		m.PowerSavingModeEnabled = cfg.PowerSavingModeEnabled                             // 开启和关闭节能模式
		m.PowerSavingModeAuto = cfg.PowerSavingModeAuto                                   // 自动切换节能模式，依据为是否插拔电源
		m.PowerSavingModeAutoWhenBatteryLow = cfg.PowerSavingModeAutoWhenBatteryLow       // 低电量时自动开启
		m.PowerSavingModeBrightnessDropPercent = cfg.PowerSavingModeBrightnessDropPercent // 开启节能模式时降低亮度的百分比值
		m.Mode = cfg.Mode
		m.fakeMode = cfg.Mode
	}

	// 恢复配置
	err := m.doSetMode(m.Mode, m.fakeMode)
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
	// 获取cpu Hardware
	cpuinfo, err := cpuinfo.ReadCPUInfo("/proc/cpuinfo")
	if err != nil {
		return err
	}

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

	getSpecialCpuMode := func() {
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

	getSpecialCpuMode()

	getIdlePowersaveAspmEnabled := func() {
		data, err := dsPower.Value(0, dsettingsIdlePowersaveAspmEnabled)
		if err != nil {
			logger.Warning(err)
			return
		}
		m.idlePowersaveAspmEnabled = data.Value().(bool)
		logger.Info("Set idle powersave aspm enabled", m.idlePowersaveAspmEnabled)
	}

	getIdlePowersaveAspmEnabled()

	dsPower.InitSignalExt(m.systemSigLoop, true)
	_, err = dsPower.ConnectValueChanged(func(key string) {
		if key == dsettingsIdlePowersaveAspmEnabled {
			getIdlePowersaveAspmEnabled()
		}
	})

	getPowerSavingModeAuto := func() {
		data, err := dsPower.Value(0, dsettingsPowerSavingModeAuto)
		if err != nil {
			logger.Warning(err)
			return
		}
		m.PowerSavingModeAuto = data.Value().(bool)
		logger.Info("Set power saving mode auto", m.PowerSavingModeAuto)
	}

	getPowerSavingModeEnabled := func() {
		data, err := dsPower.Value(0, dsettingsPowerSavingModeEnabled)
		if err != nil {
			logger.Warning(err)
			return
		}
		m.PowerSavingModeEnabled = data.Value().(bool)
		logger.Info("Set power saving mode enable", m.PowerSavingModeEnabled)
	}

	getPowerSavingModeAutoWhenBatteryLow := func() {
		data, err := dsPower.Value(0, dsettingsPowerSavingModeAutoWhenBatteryLow)
		if err != nil {
			logger.Warning(err)
			return
		}
		m.PowerSavingModeAutoWhenBatteryLow = data.Value().(bool)
		logger.Info("Set power saving mode auto when battery low", m.PowerSavingModeAutoWhenBatteryLow)
	}

	getPowerSavingModeBrightnessDropPercent := func() {
		data, err := dsPower.Value(0, dsettingsPowerSavingModeBrightnessDropPercent)
		if err != nil {
			logger.Warning(err)
			return
		}
		switch v := data.Value().(type) {
		case float64:
			m.PowerSavingModeBrightnessDropPercent = uint32(v)
		case int64:
			m.PowerSavingModeBrightnessDropPercent = uint32(v)
		default:
			logger.Warning("type is wrong!")
		}
		logger.Info("Set power saving mode brightness drop percent", m.PowerSavingModeBrightnessDropPercent)
	}

	getMode := func() {
		data, err := dsPower.Value(0, dsettingsMode)
		if err != nil {
			logger.Warning(err)
			return
		}
		m.Mode = data.Value().(string)
		logger.Info("Set power mode", m.Mode)
	}

	getPowerSavingModeAuto()
	getPowerSavingModeEnabled()
	getPowerSavingModeAutoWhenBatteryLow()
	getPowerSavingModeBrightnessDropPercent()
	getMode()

	return err
}

func (m *Manager) refreshSystemPowerPerformance() { // 获取系统支持的性能模式
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return
	}
	m.systemSigLoop = dbusutil.NewSignalLoop(systemBus, 10)
	err = m.initDsgConfig()
	if err != nil {
		logger.Warning(err)
	}
	m.systemSigLoop.Start()

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

func (m *Manager) saveConfig() error {
	logger.Debug("call saveConfig")

	var cfg Config
	m.PropsMu.RLock()
	cfg.PowerSavingModeAuto = m.PowerSavingModeAuto
	cfg.PowerSavingModeEnabled = m.PowerSavingModeEnabled
	cfg.PowerSavingModeAutoWhenBatteryLow = m.PowerSavingModeAutoWhenBatteryLow
	cfg.PowerSavingModeBrightnessDropPercent = m.PowerSavingModeBrightnessDropPercent
	cfg.Mode = m.Mode
	m.PropsMu.RUnlock()

	dir := filepath.Dir(configFile)
	err := os.MkdirAll(dir, 0755) // #nosec G301
	if err != nil {
		return err
	}

	content, err := json.Marshal(&cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configFile, content, 0644) // #nosec G306
}

func (m *Manager) doSetMode(mode string, fakeMode string) error {
	var err error
	switch mode {
	case "balance": // governor=performance boost=false
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

		m.setPropPowerSavingModeEnabled(false)
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
	case "powersave": // governor=powersave boost=false
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

		m.setPropPowerSavingModeEnabled(true)
		if m.hasPstate {
			err = m.doSetCpuGovernor("power")
		} else {
			err = m.doSetCpuGovernor("powersave")
		}
		if err != nil {
			logger.Warning(err)
		}

	case "performance": // governor=performance boost=true
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

		// fakeMode是用户期望的的调整过的模式，Mode是实际设置的模式，但对外表现为fakeMode
		// 例如为了防止在恢复时亮度有变化，这里设置高性能时对外表现仍然是节能模式
		if fakeMode == "powersave" {
			m.setPropPowerSavingModeEnabled(true)
			err = m.doSetCpuGovernor("performance")
			if err != nil {
				logger.Warning(err)
			}
		} else if fakeMode == "balance" {
			m.setPropPowerSavingModeEnabled(false)
			err = m.doSetCpuGovernor("performance")
			if err != nil {
				logger.Warning(err)
			}
		} else if fakeMode == "performance" {
			m.setPropPowerSavingModeEnabled(false)
			err = m.doSetCpuGovernor("performance")
			if err != nil {
				logger.Warning(err)
			}
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
		m.setPropMode(mode)
		m.fakeMode = fakeMode
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

func (m *Manager) enablePerformanceInBoot() bool {
	if m.Mode == "performance" {
		return false
	}

	err := m.doSetMode("performance", m.fakeMode)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return true
}
