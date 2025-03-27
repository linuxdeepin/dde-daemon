// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"bufio"
	"bytes"
	"errors"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"

	"github.com/godbus/dbus/v5"
	geoclue2 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.geoclue2"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	// ColorTemperatureModeNone 不调整色温
	ColorTemperatureModeNone int32 = iota
	// ColorTemperatureModeAuto 自动调整色温
	ColorTemperatureModeAuto
	// ColorTemperatureModeManual 手动调整色温
	ColorTemperatureModeManual
	ColorTemperatureModeCustom
)

const (
	timeZoneFile = "/usr/share/zoneinfo/zone1970.tab"

	defaultAutoColorTemperatureConf = "6500:3500"
	defaultTemperature              = 6500
)

func isValidColorTempMode(mode int32) bool {
	return mode >= ColorTemperatureModeNone && mode <= ColorTemperatureModeCustom
}

// dbus 上导出的方法
func (m *Manager) setColorTempMode(mode int32) error {
	if !isValidColorTempMode(mode) {
		return errors.New("mode out of range, not 0 or 1 or 3")
	}

	if mode != ColorTemperatureModeNone {
		m.setCustomColorTempModeOn(mode)
	}
	m.setPropColorTemperatureMode(mode)
	m.setPropColorTemperatureEnabled(mode != 0)
	m.setColorTempModeReal(mode)
	m.saveColorTempModeInCfg(mode)
	return nil
}

func (m *Manager) setColorTempModeReal(mode int32) {
	if _greeterMode {
		return
	}
	switch mode {
	case ColorTemperatureModeAuto: // 自动模式调节色温 启动服务
		m.redshiftRunner.start()
		m.stopCustomColorTempMode()
	case ColorTemperatureModeManual, ColorTemperatureModeNone:
		// manual 手动调节色温
		// none 恢复正常色温
		m.redshiftRunner.stop()
		m.stopCustomColorTempMode()
	case ColorTemperatureModeCustom:
		m.redshiftRunner.stop()
		m.listenCustomColorTempTime()
	}
	// 对于自动模式，也要先把色温设置为正常。
	m.setColorTempOneShot()
}

const (
	redshiftStateRunning = iota + 1
	redshiftStateStopped
)

type zoneInfo struct {
	country   string
	latitude  float64
	longitude float64
	distance  float64
}

type redshiftRunner struct {
	mu                 sync.Mutex
	state              int
	timer              *time.Timer
	process            *os.Process
	value              int
	cb                 func(value int)
	sysService         *dbusutil.Service
	geoAgentRegistered bool

	zoneInfoMap map[string]*zoneInfo
}

func convertPos(pos string, digits int32) float64 {
	if len(pos) < 4 || digits > 9 {
		return 0.0
	}

	integer := pos[:digits+1]
	fraction := pos[digits+1:]
	t1, _ := strconv.ParseFloat(integer, 64)
	t2, _ := strconv.ParseFloat(fraction, 64)
	if t1 > 0.0 {
		return t1 + t2/math.Pow(10.0, float64(len(fraction)))
	} else {
		return t1 - t2/math.Pow(10.0, float64(len(fraction)))
	}
}

func newRedshiftRunner() *redshiftRunner {
	sysService, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Warning("new sys service failed:", err)
	}
	zoneInfoMap := make(map[string]*zoneInfo)
	contents, err := os.ReadFile(timeZoneFile)
	if err != nil {
		logger.Warning("Red timezone file failed:", err)
	}
	lines := bytes.Split(contents, []byte{'\n'})
	for _, line := range lines {
		if !bytes.HasPrefix(line, []byte{'#'}) {
			parts := bytes.Split(line, []byte{'\t'})
			if len(parts) >= 3 {
				coordinates := string(parts[1])
				index := strings.Index(coordinates[3:], "+")
				if index == -1 {
					index = strings.Index(coordinates[3:], "-")
				}
				if index > -1 {
					latitude := convertPos(coordinates[:index+3], 2)
					longitude := convertPos(coordinates[index+3:], 3)
					zone_info := &zoneInfo{
						country:   string(parts[0]),
						latitude:  latitude,
						longitude: longitude,
					}
					zoneInfoMap[string(parts[2])] = zone_info
				}
			}
		}
	}
	return &redshiftRunner{
		sysService:  sysService,
		zoneInfoMap: zoneInfoMap,
	}
}

func (r *redshiftRunner) start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	logger.Debugf("redshiftRunner.start")

	if r.state == redshiftStateRunning {
		return
	}
	r.state = redshiftStateRunning
	var colorConf string
	val, err := getGlobalDconfValue(DSettingsAppID, DSettingsDisplayName, "", DSettingsKeyAutoColorTemperature)
	if err != nil {
		colorConf = defaultAutoColorTemperatureConf
		logger.Warning(err)
	} else {
		colorConf = val.(string)
	}
	latitude := r.zoneInfoMap[_timeZone].latitude
	longitude := r.zoneInfoMap[_timeZone].longitude
	geographicalPosition := strconv.FormatFloat(latitude, 'f', -1, 64) + ":" + strconv.FormatFloat(longitude, 'f', -1, 64)
	logger.Info("Get geographicalPosition:", geographicalPosition)
	cmd := exec.Command("redshift", "-m", "dummy", "-t", colorConf, "-r")
	if geographicalPosition != "" {
		cmd.Args = append(cmd.Args, "-l", geographicalPosition)
	}
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Warning("get stdout pipe failed:", err)
		r.state = redshiftStateStopped
		return
	}
	err = cmd.Start()
	if err != nil {
		logger.Warning("start redshift failed:", err)
		r.state = redshiftStateStopped
		return
	}

	r.process = cmd.Process
	r.value = 0

	go func() {
		logger.Debugf("redshift is running, pid: %v", r.process.Pid)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Bytes()
			logger.Debugf("line: %s", line)
			temp, ok := getTemperatureWithLine(line)
			if ok {
				logger.Debug("temp:", temp)
				r.updateValue(temp)
			}
		}
		err := scanner.Err()
		if err != nil {
			logger.Warning("scanner err:", err)
		}

		err = cmd.Wait()
		if err != nil {
			logger.Debugf("redshift cmd wait err: %v, errBuf: %s", err, errBuf.Bytes())
		}
		logger.Debug("redshift stopped")

		r.mu.Lock()
		r.state = redshiftStateStopped
		r.process = nil
		r.mu.Unlock()
	}()

	return
}

func (m *Manager) listenTimezone() {
	jobMatchRule := dbusutil.NewMatchRuleBuilder().ExtPropertiesChanged(
		"/org/freedesktop/timedate1", "org.freedesktop.timedate1").Build()
	err := jobMatchRule.AddTo(m.sysBus)
	if err != nil {
		logger.Warning(err)
		return
	}
	sigChan := make(chan *dbus.Signal, 10)
	m.sysBus.Signal(sigChan)

	defer func() {
		m.sysBus.RemoveSignal(sigChan)
		err := jobMatchRule.RemoveFrom(m.sysBus)
		if err != nil {
			logger.Warning(err)
		}
	}()

	for sig := range sigChan {
		if sig.Path == "/org/freedesktop/timedate1" &&
			sig.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" {
			if len(sig.Body) != 3 {
				logger.Warning(err)
				return
			}

			props, ok := sig.Body[1].(map[string]dbus.Variant)
			if !ok {
				logger.Warning(err)
				return
			}
			v, ok := props["Timezone"]
			if ok {
				timezone, _ := v.Value().(string)
				logger.Info("Timezone change to", timezone)
				_timeZone = timezone
				if m.redshiftRunner.state == redshiftStateRunning {
					logger.Info("Redshift is Running")
					m.redshiftRunner.stop()
					time.AfterFunc(50*time.Millisecond, func() {
						m.redshiftRunner.start()
					})
				}
			}
		}
	}
}

func (r *redshiftRunner) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state == redshiftStateStopped {
		return
	}

	logger.Debug("redshiftRunner.stop")
	if r.process != nil {
		err := r.process.Signal(os.Interrupt)
		if err != nil {
			logger.Warning("send signal interrupt to redshift process failed:", err)
		}
	}
	r.value = 0
}

func (r *redshiftRunner) updateValue(value int) {
	r.mu.Lock()
	if r.value == value {
		// no change
		r.mu.Unlock()
		return
	}
	r.value = value
	r.mu.Unlock()

	if r.cb != nil {
		r.cb(value)
	}
}

func (r *redshiftRunner) getValue() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.value
}

func getTemperatureWithLine(line []byte) (int, bool) {
	const prefix = "Temperature: "
	if bytes.HasPrefix(line, []byte(prefix)) {
		numStr := string(line[len(prefix):])
		num, err := strconv.Atoi(numStr)
		return num, err == nil
	}
	return 0, false
}

// dbus 上导出的方法
func (m *Manager) setColorTempValue(value int32) error {
	if m.ColorTemperatureMode != ColorTemperatureModeManual {
		return errors.New("current not manual mode, can not adjust color temperature by manual")
	}
	if !isValidColorTempValue(value) {
		return errors.New("value out of range")
	}
	m.PropsMu.Lock()
	m.setPropColorTemperatureManual(value)
	m.PropsMu.Unlock()
	m.setColorTempOneShot()
	m.saveColorTempValueInCfg(value)
	return nil
}

func isValidColorTempValue(value int32) bool {
	return value >= 1000 && value <= 25000
}

// saveColorTempValueInCfg 保存手动色温值到用户配置
func (m *Manager) saveColorTempValueInCfg(value int32) {
	m.modifySuitableUserMonitorModeConfig(func(cfg *UserMonitorModeConfig) {
		cfg.ColorTemperatureManual = value
	})
	err := m.saveUserConfig()
	if err != nil {
		logger.Warning(err)
	}
}

// saveColorTempModeInCfg 保存色温模式到用户配置
func (m *Manager) saveColorTempModeInCfg(mode int32) {
	m.modifySuitableUserMonitorModeConfig(func(cfg *UserMonitorModeConfig) {
		cfg.ColorTemperatureMode = mode
		if cfg.ColorTemperatureMode != ColorTemperatureModeNone {
			cfg.ColorTemperatureModeOn = cfg.ColorTemperatureMode
		}
	})
	err := m.saveUserConfig()
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) getColorTemperatureValue() int {
	m.PropsMu.RLock()
	mode := m.ColorTemperatureMode
	manual := m.ColorTemperatureManual
	m.PropsMu.RUnlock()

	switch mode {
	case ColorTemperatureModeNone:
		return defaultTemperatureManual
	case ColorTemperatureModeManual:
		return int(manual)
	case ColorTemperatureModeAuto:
		return m.redshiftRunner.getValue()
	case ColorTemperatureModeCustom:
		value := defaultTemperature
		if m.customColorTempFlag {
			value = int(manual)
		}
		return value
	}

	return defaultTemperatureManual
}

// applyColorTempConfig 应用色温设置
func (m *Manager) applyColorTempConfig(displayMode byte) {
	cfg := m.getSuitableUserMonitorModeConfig(displayMode)
	if cfg == nil {
		cfg = getDefaultUserMonitorModeConfig()
	}
	m.setPropColorTemperatureMode(cfg.ColorTemperatureMode)
	m.setPropColorTemperatureEnabled(cfg.ColorTemperatureMode != 0)
	m.setPropColorTemperatureManual(cfg.ColorTemperatureManual)
	m.setColorTempModeReal(m.ColorTemperatureMode)
}

func controlRedshift(action string) {
	// #nosec G204
	_, err := exec.Command("systemctl", "--user", action, "redshift.service").Output()
	if err != nil {
		logger.Warning("failed to ", action, " redshift.service:", err)
	} else {
		logger.Info("success to ", action, " redshift.service")
	}
}

var _setColorTempMu sync.Mutex

func (m *Manager) setColorTempOneShot() {
	_setColorTempMu.Lock()
	defer _setColorTempMu.Unlock()

	monitors := m.getConnectedMonitors()
	for _, monitor := range monitors {
		monitor.PropsMu.RLock()
		br := monitor.Brightness
		name := monitor.Name
		monitor.PropsMu.RUnlock()

		err := m.setBrightness(name, br)
		if err != nil {
			logger.Warning(err)
		}
	}
}

const (
	dbusIfcGeoClueAgent  = "org.freedesktop.GeoClue2.Agent"
	dbusPathGeoClueAgent = "/org/freedesktop/GeoClue2/Agent"
)

type geoClueAgent struct {
	MaxAccuracyLevel uint32
}

const (
	AccuracyLevelNone         = 0
	AccuracyLevelCountry      = 1
	AccuracyLevelCity         = 4
	AccuracyLevelNeighborhood = 5
	AccuracyLevelStreet       = 6
	AccuracyLevelExact        = 8
)

func (a *geoClueAgent) GetInterfaceName() string {
	return dbusIfcGeoClueAgent
}

func (a *geoClueAgent) AuthorizeApp(desktopId string, reqAccuracyLevel uint32) (authorized bool, allowedAccuracyLevel uint32, busErr *dbus.Error) {
	// 目前发现这个方法不会被调用。
	logger.Debugf("AuthorizeApp desktopId: %q, reqAccuracyLevel: %v", desktopId, reqAccuracyLevel)
	return true, reqAccuracyLevel, nil
}

func (a *geoClueAgent) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name:    "AuthorizeApp",
			Fn:      a.AuthorizeApp,
			InArgs:  []string{"desktopId", "reqAccuracyLevel"},
			OutArgs: []string{"authorized", "allowedAccuracyLevel"},
		},
	}
}

func (r *redshiftRunner) registerGeoClueAgent() error {
	if r.geoAgentRegistered {
		return nil
	}
	if r.sysService == nil {
		return errors.New("sys service is nil")
	}

	sysBus := r.sysService.Conn()
	agent := &geoClueAgent{
		MaxAccuracyLevel: AccuracyLevelStreet,
	}
	err := r.sysService.Export(dbusPathGeoClueAgent, agent)
	if err != nil {
		return err
	}

	geoManager := geoclue2.NewManager(sysBus)
	err = geoManager.AddAgent(0, "geoclue-demo-agent")
	if err != nil {
		return err
	}

	r.geoAgentRegistered = true
	return nil
}

func setGlobalDconfValue(appID string, name string, subPath string, key string, value dbus.Variant) error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return err
	}
	ds := configManager.NewConfigManager(sysBus)
	managerPath, err := ds.AcquireManager(0, appID, name, subPath)
	if err != nil {
		logger.Warning(err)
		return err
	}

	dsManager, err := configManager.NewManager(sysBus, managerPath)
	if err != nil {
		logger.Warning(err)
		return err
	}

	err = dsManager.SetValue(0, key, value)
	if err != nil {
		logger.Warning(err)
		return err
	}
	return nil
}

func (m *Manager) setCustomColorTempTimePeriod(timePeriod string) error {
	pattern := `^(?:[01]\d|2[0-3]):[0-5]\d-(?:[01]\d|2[0-3]):[0-5]\d$`
	re := regexp.MustCompile(pattern)
	var err error
	if re.MatchString(timePeriod) {
		err = setGlobalDconfValue(DSettingsAppID, DSettingsDisplayName, "", DSettingsKeyCustomModeTime, dbus.MakeVariant(timePeriod))
		if err == nil {
			m.setPropCustomColorTempTimePeriod(timePeriod)
		}
	} else {
		err = errors.New("The timeperiod parameter is invalid")
	}
	return err
}

func (m *Manager) setCustomColorTempModeOn(modeOn int32) error {
	var err error
	err = setGlobalDconfValue(DSettingsAppID, DSettingsDisplayName, "", DSettingKeyColorTemperatureModeOn, dbus.MakeVariant(modeOn))
	if err != nil {
		logger.Warning("try set modeOn failed")
	}

	return err
}

func (m *Manager) listenCustomColorTempTime() {
	if m.customColorTempTimer == nil {
		m.customColorTempTimer = time.NewTimer(5 * time.Second)
	} else {
		m.customColorTempTimer.Reset(5 * time.Second)
	}
	go func() {
		for {
			select {
			case <-m.customColorTempTimer.C:
				if m.ColorTemperatureMode == ColorTemperatureModeCustom {
					m.customColorTempFlag = m.checkCustomModeTime()
					m.setColorTempOneShot()
				}
			}
			if m.customColorTempTimer == nil {
				m.customColorTempTimer = time.NewTimer(5 * time.Second)
			} else {
				m.customColorTempTimer.Reset(5 * time.Second)
			}
		}
	}()
}

func (m *Manager) checkCustomModeTime() bool {
	parts := strings.Split(m.CustomColorTempTimePeriod, "-")
	if len(parts) == 2 {
		// 获取当前日期
		timeLocation, err := time.LoadLocation(_timeZone)
		if err != nil {
			logger.Warning(err)
			return false
		}
		currentTime := time.Now().In(timeLocation)
		year, month, day := currentTime.Date()

		// 构建目标时间
		targetTimeLayout := "15:04"
		targetTimeStart, err := time.Parse(targetTimeLayout, parts[0])
		if err != nil {
			logger.Warning("Failed to get start time:", err)
			return false
		}
		targetTimeEnd, err := time.Parse(targetTimeLayout, parts[1])
		if err != nil {
			logger.Warning("Failed to get start time:", err)
			return false
		}
		// 需要考虑凌晨，因为不是同一天
		targetTimeStart = time.Date(year, month, day, targetTimeStart.Hour(), targetTimeStart.Minute(), 0, 0, currentTime.Location())
		targetTimeEnd = time.Date(year, month, day, targetTimeEnd.Hour(), targetTimeEnd.Minute(), 0, 0, currentTime.Location())
		// 计算时间间隔
		currentToStart := targetTimeStart.Sub(currentTime)
		currentToEnd := targetTimeEnd.Sub(currentTime)
		startToEnd := targetTimeEnd.Sub(targetTimeStart)
		// 如果设定的时间段小于0，表示结束时间为第二天
		if startToEnd < 0 {
			return (currentToStart < 0 && currentToEnd < 0) || (currentToStart > 0 && currentToEnd > 0)
		} else {
			return (currentToStart < 0) && (currentToEnd > 0)
		}
	}
	return false
}

func (m *Manager) stopCustomColorTempMode() {
	if m.customColorTempTimer != nil {
		m.customColorTempTimer.Stop()
		m.customColorTempTimer = nil
	}
}

func getGlobalDconfValue(appID string, name string, subPath string, key string) (interface{}, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	ds := configManager.NewConfigManager(sysBus)
	managerPath, err := ds.AcquireManager(0, appID, name, subPath)
	if err != nil {
		logger.Warning(err)
		return nil, err
	}

	dsManager, err := configManager.NewManager(sysBus, managerPath)
	if err != nil {
		logger.Warning(err)
		return nil, err
	}

	val, err := dsManager.Value(0, key)
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	return val.Value(), nil
}
