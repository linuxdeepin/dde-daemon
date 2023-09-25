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
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/powersupply"
	"github.com/linuxdeepin/dde-api/powersupply/battery"
	gudev "github.com/linuxdeepin/go-gir/gudev-1.0"
	"github.com/linuxdeepin/go-lib/arch"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

var noUEvent bool

const (
	dsettingsAppID                = "org.deepin.dde.daemon"
	dsettingsTlpName              = "org.deepin.dde.daemon.power.tlp"
	dsettingsBluetoothAdapterName = "org.deepin.dde.daemon.power.bluetoothAdapter"
)

const (
	configManagerId = "org.desktopspec.ConfigManager"
	_configHwSystem = "/usr/share/uos-hw-config"
	intelPstatePath = "/sys/devices/system/cpu/intel_pstate"
	amdPstatePath   = "/sys/devices/system/cpu/amd_pstate"
	amdGPUPath      = "/sys/class/drm/card0/device/power_dpm_force_performance_level"
)

const (
	pstateConfPath  = "/sys/devices/system/cpu/cpufreq/policy0/energy_performance_preference"
	scalingConfPath = "/sys/devices/system/cpu/cpufreq/policy0/scaling_governor"
)

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

	// 开启自动节能模式电量的百分比值
	PowerSavingModeAutoBatteryPercent uint32 `prop:"access:rw"`

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

	// 是否支持平衡性能模式
	IsBalancePerformanceSupported bool

	// 性能模式-平衡模式dsg
	balanceScalingGovernor string
	configManagerPath      dbus.ObjectPath

	// 当前模式
	Mode string

	// 蓝牙适配器状态
	bluetoothAdapterEnabledCmd string
	bluetoothAdapterScanCmd    string

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
	m.hasPstate = cpuHasPstate() // check if amd is used
	m.hasAmddpm = dutils.IsFileExist(amdGPUPath)

	m.refreshSystemPowerPerformance()

	err := m.init()
	if err != nil {
		m.destroy()
		return nil, err
	}

	go m.initFsWatcher()

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

type ModeData struct {
	mode     string
	governor string
}

func (m *Manager) getModeFromFile() (ModeData, error) {
	fileName := ""
	if m.hasPstate {
		fileName = pstateConfPath
	} else {
		fileName = scalingConfPath
	}
	data, err := os.ReadFile(fileName)
	if err != nil {
		return ModeData{"", ""}, err
	}
	status := strings.TrimSpace(string(data))
	if status == m.balanceScalingGovernor {
		return ModeData{"balance", status}, nil
	}
	if status == "performance" || status == "high" {
		return ModeData{"performance", status}, nil
	}

	if status == "powersave" || status == "power" {
		return ModeData{"powersave", status}, nil
	}
	logger.Info("Unknown status:", status)

	return ModeData{"", ""}, nil
}

func (m *Manager) initFsWatcher() {
	watcher, err := fsnotify.NewWatcher()
	defer watcher.Close()

	if err != nil {
		logger.Warning("power fswatcher error:", err)
		return
	}

	e := watcher.Add("/sys/devices/system/cpu/cpufreq/policy0")
	if e != nil {
		logger.Warning("power fswatcher error:", e)
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok || event.Op&fsnotify.Write != fsnotify.Write {
				continue
			}
			if (m.hasPstate && strings.HasSuffix(event.Name, "energy_performance_preference")) || strings.HasSuffix(event.Name, "scaling_governor") {
				modeAndgovData, err := m.getModeFromFile()
				if err == nil && modeAndgovData.mode != "" {
					if modeAndgovData.mode != m.Mode {
						err = m.doSetMode(m.Mode)
						if err != nil {
							logger.Warning("power set mode error:", err)
						}
						continue
					}
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Warning("power fswatcher error:", err)
		}
	}
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
	// 将config.json中的配置完成初始化
	m.PowerSavingModeEnabled = cfg.PowerSavingModeEnabled                       // 开启和关闭节能模式
	m.PowerSavingModeAuto = cfg.PowerSavingModeAuto                             // 自动切换节能模式，依据为是否插拔电源
	m.PowerSavingModeAutoWhenBatteryLow = cfg.PowerSavingModeAutoWhenBatteryLow // 低电量时自动开启

	m.PowerSavingModeBrightnessDropPercent = cfg.PowerSavingModeBrightnessDropPercent // 开启节能模式时降低亮度的百分比值
	m.PowerSavingModeAutoBatteryPercent = cfg.PowerSavingModeAutoBatteryPercent       // 开启在低电量自动节能模式时候的百分比

	m.Mode = cfg.Mode

	// 恢复配置
	err := m.doSetMode(m.Mode)
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

	return nil
}

func (m *Manager) refreshSystemPowerPerformance() { // 获取系统支持的性能模式
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return
	}
	m.systemSigLoop = dbusutil.NewSignalLoop(systemBus, 10)
	m.initDsgConfig(systemBus)
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

	path := m.cpus.getCpuGovernorPath(m.hasPstate)
	if "" == path {
		m.IsHighPerformanceSupported = false
		m.IsBalanceSupported = false
		m.IsPowerSaveSupported = false
		m.IsBalancePerformanceSupported = false
		return
	}

	dsgCpuGovernor := interfaceToString(m.getDsgData("BalanceCpuGovernor"))
	availableArrGovernors := m.cpus.getAvailableArrGovernors(m.hasPstate)

	setUseNormalBalance(useNormalBalance())
	//  如果当前是 平衡模式, 且不走正常平衡模式，且CpuGovernor不是performance, 则需要将m.CpuGovernor设置为performance
	if m.Mode == "balance" && !getUseNormalBalance() && m.CpuGovernor != "performance" {
		err := m.doSetCpuGovernor("performance")
		if err != nil {
			logger.Warning(err)
		}
	}

	// 重启会进去，手动修改了dconfig的值才会进入else
	if interfaceToBool(m.getDsgData("isFirstGetCpuGovernor")) {
		if len(*m.cpus) <= 0 {
			return
		}

		// 全部模式都支持的时候,不需要进行写入文件验证
		// TODO: 如果有pstate跳过检查
		if len(availableArrGovernors) != 6 && !m.hasPstate {
			availableArrGovernors = m.cpus.tryWriteGovernor(availableArrGovernors, m.hasPstate)
		}
		logger.Info(" First. available cpuGovernors : ", availableArrGovernors)
		m.setDsgData("supportCpuGovernors", setSupportGovernors(availableArrGovernors))
		setLocalAvailableGovernors(availableArrGovernors)

		err, targetGovernor := trySetBalanceCpuGovernor(dsgCpuGovernor)
		if err != nil {
			logger.Warning(err)
		} else {
			if dsgCpuGovernor != targetGovernor && targetGovernor != "" {
				m.setDsgData("BalanceCpuGovernor", targetGovernor)
				logger.Info(" DConfig BalanceCpuGovernor not support. Set Governor : ", targetGovernor)
			}
		}
	} else {
		supportCpuGovernors := m.getDsgData("supportCpuGovernors")
		cpuGovernors := interfaceToArrayString(supportCpuGovernors)
		logger.Info(" dsg translate to Arr cpuGovernors : ", cpuGovernors)

		supportGovernors := make([]string, len(cpuGovernors))
		for i, v := range cpuGovernors {
			supportGovernors[i] = v.(string)
		}
		setSupportGovernors(supportGovernors)
		setLocalAvailableGovernors(availableArrGovernors)
	}

	m.IsBalanceSupported = getIsBalanceSupported(m.hasPstate)
	m.IsHighPerformanceSupported = getIsHighPerformanceSupported(m.hasPstate)
	m.IsPowerSaveSupported = getIsPowerSaveSupported(m.hasPstate)
	m.IsBalancePerformanceSupported = m.hasPstate

	if m.hasPstate {
		// INFO: balance_performance or balance_power?
		m.balanceScalingGovernor = "balance_power"
	} else if strv.Strv(getSupportGovernors()).Contains(dsgCpuGovernor) {
		m.balanceScalingGovernor = dsgCpuGovernor
	} else {
		_, m.balanceScalingGovernor = trySetBalanceCpuGovernor(dsgCpuGovernor)
	}

	if !m.IsBalanceSupported {
		logger.Info(" init end. getSupportGovernors ： ", getSupportGovernors(), " , m.balanceScalingGovernor : ", m.balanceScalingGovernor)
		return
	}

	// 当支持平衡模式时, 错误将dsg配置设置成""或不支持的值, 则使用默认的平衡模式
	if !m.hasPstate && (m.balanceScalingGovernor == "" || isSystemSupportMode(interfaceToString(m.getDsgData("BalanceCpuGovernor")))) {
		availableArrGovsLen := len(availableArrGovernors)
		if availableArrGovsLen <= 0 {
			m.IsBalanceSupported = false
			return
		} else {
			for i, v := range availableArrGovernors {
				if strv.Strv(getScalingBalanceAvailableGovernors()).Contains(v) && !isSystemSupportMode(m.balanceScalingGovernor) {
					m.balanceScalingGovernor = v
					m.setDsgData("BalanceCpuGovernor", m.balanceScalingGovernor)
					break
				}

				if i == availableArrGovsLen {
					m.IsBalanceSupported = false
					break
				}
			}

		}
	}

	logger.Info(" init end. getSupportGovernors ： ", getSupportGovernors(), " , m.balanceScalingGovernor : ", m.balanceScalingGovernor)
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
	PowerSavingModeAutoBatteryPercent    uint32
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
		return &Config{
			// default config
			PowerSavingModeAuto:                  true,
			PowerSavingModeEnabled:               false,
			PowerSavingModeAutoWhenBatteryLow:    false,
			PowerSavingModeBrightnessDropPercent: 20,
			PowerSavingModeAutoBatteryPercent:    20,
			Mode:                                 "balance",
		}
	}
	// 新增字段后第一次启动时,缺少两个新增字段的json,导致亮度下降百分比字段默认为0,导致与默认值不符,需要处理
	// 低电量自动待机字段的默认值为false,不会导致错误影响
	// 正常情况下该字段范围为10-40,只有在该情况下会出现0的可能
	if cfg.PowerSavingModeBrightnessDropPercent == 0 {
		cfg.PowerSavingModeBrightnessDropPercent = 20
	}

	// when PowerSavingModeAutoBatteryPercent is lower than 10, it is not available, so change set it to 20 as default
	if cfg.PowerSavingModeAutoBatteryPercent < 10 {
		cfg.PowerSavingModeAutoBatteryPercent = 20
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
	cfg.PowerSavingModeAutoBatteryPercent = m.PowerSavingModeAutoBatteryPercent
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

func (m *Manager) doSetMode(mode string) error {
	var err error

	previousMode := m.Mode
	m.setPropMode(mode)

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

		m.PropsMu.Lock()
		m.setPowerSavingModeEnabled(false)
		m.PropsMu.Unlock()

		balanceScalingGovernor := m.balanceScalingGovernor
		// if do not have pstate, go to the logic below
		if !m.hasPstate {
			err, targetGovernor := trySetBalanceCpuGovernor(balanceScalingGovernor)
			if err != nil {
				logger.Warning(err)
				return err
			}
			dsgCpuGovernor := interfaceToString(m.getDsgData("BalanceCpuGovernor"))
			logger.Infof(" [doSetMode] dsgCpuGovernor : %s, targetGovernor : %s, balanceScalingGovernor : %s ", dsgCpuGovernor, targetGovernor, balanceScalingGovernor)
			if balanceScalingGovernor != targetGovernor {
				balanceScalingGovernor = targetGovernor
				logger.Info("[doSetMode] BalanceCpuGovernor not support. Set available governor : ", targetGovernor)
			}

			if balanceScalingGovernor != dsgCpuGovernor {
				m.setDsgData("BalanceCpuGovernor", balanceScalingGovernor)
			}

			logger.Info("[doSetMode] Set Governor, balanceScalingGovernor : ", balanceScalingGovernor)
		}
		err = m.doSetCpuGovernor(balanceScalingGovernor)
		if err != nil {
			logger.Warning(err)
		}
		// TODO remove it later
		if m.cpus.IsBoostFileExist() {
			err = m.doSetCpuBoost(true)
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
		m.PropsMu.Lock()
		m.setPowerSavingModeEnabled(true)
		m.PropsMu.Unlock()
		if m.hasPstate {
			err = m.doSetCpuGovernor("power")
		} else {
			err = m.doSetCpuGovernor("powersave")
		}
		if err != nil {
			logger.Warning(err)
		}
		// TODO remove it later
		if m.cpus.IsBoostFileExist() {
			err = m.doSetCpuBoost(true)
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
		m.PropsMu.Lock()
		m.setPowerSavingModeEnabled(false)
		m.PropsMu.Unlock()
		err = m.doSetCpuGovernor("performance")
		if err != nil {
			logger.Warning(err)
		}

		if m.cpus.IsBoostFileExist() {
			err = m.doSetCpuBoost(true)
		}
	case "balance_performance":
		if !m.IsBalancePerformanceSupported {
			err = dbusutil.MakeErrorf(m, "PowerMode", "%q mode is not supported", mode)
			break
		}
		m.setPropPowerSavingModeEnabled(false)
		err = m.doSetCpuGovernor("balance_performance")
		if err != nil {
			logger.Warning(err)
		}

		if m.cpus.IsBoostFileExist() {
			err = m.doSetCpuBoost(true)
		}
	default:
		err = dbusutil.MakeErrorf(m, "PowerMode", "%q mode is not supported", mode)
	}

	//set CpuGovernor file failed, restore previous mode
	if err != nil {
		m.setPropMode(previousMode)
	}

	return err
}

func (m *Manager) doSetCpuBoost(enabled bool) error {
	err := m.cpus.SetBoostEnabled(enabled)
	if err == nil {
		m.setPropCpuBoost(enabled)
	}
	return err
}

func (m *Manager) doSetCpuGovernor(governor string) error {
	// 节能模式，高性能模式不受影响
	if governor != "powersave" && governor != "performance" && !getUseNormalBalance() {
		governor = "performance"
		logger.Infof("[doSetCpuGovernor] change governor : %s to performance.", governor)
	}

	err := m.cpus.SetGovernor(governor, m.hasPstate)
	if err == nil {
		m.setPropCpuGovernor(governor)
	}
	return err
}

func (m *Manager) setPowerSavingModeEnabled(enable bool) (changed bool) {
	changed = m.setPropPowerSavingModeEnabled(enable)
	if !changed {
		return
	}
	err := m.writePowerSavingModeEnabledCbImpl(enable)
	if err != nil {
		logger.Warning(err)
	}
	return
}
