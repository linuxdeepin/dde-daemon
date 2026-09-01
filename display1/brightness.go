// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"

	"github.com/linuxdeepin/dde-daemon/display1/brightness"
	"github.com/linuxdeepin/dde-daemon/display1/utils"
)

type InvalidOutputNameError struct {
	Name string
}

// initBacklightCurve 初始化背光曲线相关特征配置
// 与 startdde 保持相同的特征判定逻辑：
//  1. IsM900Config：M900 机型特征（从 systeminfo dsettings 读取）
//  2. backLight-max-brightness-choose-big：ProductName 命中列表
//  3. BoardName 匹配：custom-brightness-curves 配置 boardName 与 DMI 板名匹配
//  4. backlight-curve-type：曲线类型（flm/default）
func (m *Manager) initBacklightCurve() {
	brightness.SetProductName(getDmiProductName())
	brightness.SetDeviceBoardName(getDmiBoardName())

	m.getBackLightMaxBrightnessChooseBigConfig()
	m.getM900Config()
	m.getBacklightCurveType()
	m.getBacklightMinValue()
	m.getBacklightMidValue()
	if m.backlightCurveType == "flm" {
		brightness.InitFlmCurves(m.backlightMinValue, m.backlightMidValue)
	}
	m.getDsgBrightnessPercentage()
	m.getCustomBrightnessCurves()
	m.getDefaultBrightnessCurve()
	m.getMaxBrightnessUnlimited()

	brightness.SetHasBuiltinMonitor(m.hasBuiltinMonitor)
	m.refreshMaxBacklightBrightness()
}

// getDsgBrightnessPercentage 读取实际亮度百分比（取值范围 [50,100]）
func (m *Manager) getDsgBrightnessPercentage() {
	v, err := m.displayConfigMgr.Value(0, DSettingsKeyBrightnessPercentage)
	if err != nil {
		logger.Warning(err)
		m.dsgBrightnessPercentage = 100
		return
	}
	switch vType := v.Value().(type) {
	case float64:
		m.dsgBrightnessPercentage = int32(vType)
	case int64:
		m.dsgBrightnessPercentage = int32(vType)
	default:
		logger.Warning("type is wrong!")
		m.dsgBrightnessPercentage = 100
	}

	// limit min/max value
	if m.dsgBrightnessPercentage < 50 {
		m.dsgBrightnessPercentage = 50
	} else if m.dsgBrightnessPercentage > 100 {
		m.dsgBrightnessPercentage = 100
	}
	logger.Info("Brightness percentage value:", m.dsgBrightnessPercentage)
}

// refreshMaxBacklightBrightness 重新计算并刷新 MaxBacklightBrightness 属性
func (m *Manager) refreshMaxBacklightBrightness() {
	m.setPropMaxBacklightBrightness(uint32(brightness.GetMaxBacklightBrightness()))
}

// getM900Config 读取 systeminfo dsettings 的 IsM900Config，设置 M900 特征
func (m *Manager) getM900Config() {
	sysBus := m.sysBus
	if sysBus == nil {
		var err error
		sysBus, err = dbus.SystemBus()
		if err != nil {
			logger.Warning(err)
			return
		}
	}
	ds := configManager.NewConfigManager(sysBus)
	managerPath, err := ds.AcquireManager(0, "org.deepin.dde.daemon", "org.deepin.dde.daemon.systeminfo", "")
	if err != nil {
		logger.Warning(err)
		return
	}
	mgr, err := configManager.NewManager(sysBus, managerPath)
	if err != nil {
		logger.Warning(err)
		return
	}
	v, err := mgr.Value(0, "IsM900Config")
	if err != nil {
		logger.Warning(err)
		return
	}
	isM900, ok := v.Value().(bool)
	if !ok {
		logger.Warning("IsM900Config is not bool type")
		return
	}
	logger.Info("CpuHardware is M900:", isM900)
	brightness.SetIsM900Config(isM900)
}

// getBackLightMaxBrightnessChooseBigConfig 读取 ProductName 命中后选最大亮度的机型列表
func (m *Manager) getBackLightMaxBrightnessChooseBigConfig() {
	v, err := m.displayConfigMgr.Value(0, DSettingsKeyBackLightMaxBrightnessChooseBigConfig)
	if err != nil {
		logger.Warning(err)
		return
	}
	itemList := v.Value().([]dbus.Variant)
	var list []string
	for _, i := range itemList {
		list = append(list, i.Value().(string))
	}
	m.chooseBigProductNames = list
	brightness.SetChooseBigProductNames(list)
	logger.Info("Backlight choose big product names:", list)
}

// getBacklightCurveType 读取曲线类型（default/flm）
func (m *Manager) getBacklightCurveType() {
	v, err := m.displayConfigMgr.Value(0, DSettingsKeyBacklightCurveType)
	if err != nil {
		logger.Warning(err)
		m.backlightCurveType = "default"
		brightness.SetCurveType("default")
		return
	}
	m.backlightCurveType = v.Value().(string)
	logger.Info("Backlight Curve Type:", m.backlightCurveType)
	brightness.SetCurveType(m.backlightCurveType)
}

// getBacklightMinValue 读取 FLM 曲线起始点
func (m *Manager) getBacklightMinValue() {
	v, err := m.displayConfigMgr.Value(0, DSettingsKeyBacklightMinValue)
	if err != nil {
		logger.Warning(err)
		m.backlightMinValue = 4
		return
	}
	switch vType := v.Value().(type) {
	case float64:
		m.backlightMinValue = int32(vType)
	case int64:
		m.backlightMinValue = int32(vType)
	default:
		logger.Warning("type is wrong!")
		m.backlightMinValue = 4
	}
	logger.Info("Backlight min value:", m.backlightMinValue)
}

// getBacklightMidValue 读取 FLM 曲线中间点
func (m *Manager) getBacklightMidValue() {
	v, err := m.displayConfigMgr.Value(0, DSettingsKeyBacklightMidValue)
	if err != nil {
		logger.Warning(err)
		m.backlightMidValue = 50
		return
	}
	switch vType := v.Value().(type) {
	case float64:
		m.backlightMidValue = int32(vType)
	case int64:
		m.backlightMidValue = int32(vType)
	default:
		logger.Warning("type is wrong!")
		m.backlightMidValue = 50
	}
	logger.Info("Backlight mid value:", m.backlightMidValue)
}

// getCustomBrightnessCurves 读取自定义亮度曲线配置
func (m *Manager) getCustomBrightnessCurves() {
	v, err := m.displayConfigMgr.Value(0, DSettingsKeyCustomBrightnessCurves)
	if err != nil {
		logger.Warning(err)
		return
	}
	jsonStr, ok := v.Value().(string)
	if !ok {
		logger.Warning("Custom brightness curves configuration is not a string")
		return
	}
	brightness.SetCustomBrightnessCurves(jsonStr)
	m.setPropCurveMaxScale(brightness.GetCurrentMaxScale())
}

// getDefaultBrightnessCurve 读取默认亮度曲线配置
func (m *Manager) getDefaultBrightnessCurve() {
	v, err := m.displayConfigMgr.Value(0, DSettingsKeyDefaultBrightnessCurve)
	if err != nil {
		logger.Warning(err)
		return
	}
	jsonStr, ok := v.Value().(string)
	if !ok {
		logger.Warning("Default brightness curve configuration is not a string")
		return
	}
	brightness.SetDefaultBrightnessCurve(jsonStr)
}

// getMaxBrightnessUnlimited 读取最大亮度不受限开关，并在 BoardName 匹配时启用
func (m *Manager) getMaxBrightnessUnlimited() {
	v, err := m.displayConfigMgr.Value(0, DSettingsKeyMaxBrightnessUnlimited)
	if err != nil {
		logger.Warning(err)
		return
	}
	enabled, ok := v.Value().(bool)
	if !ok {
		logger.Warning("max-brightness-unlimited is not bool type")
		return
	}

	brightness.SetDeviceBoardName(getDmiBoardName())
	boardSupported := brightness.IsDeviceSupported()
	if !boardSupported {
		logger.Warningf("Current board %s not match config", getDmiBoardName())
		return
	}

	maxScale := brightness.GetCurrentMaxScale()
	if maxScale <= 100 {
		logger.Warningf("Curve scale %d too low", maxScale)
		return
	}

	logger.Info("Max brightness unlimited:", enabled)

	// 同步 DBus 属性与受限亮度（setMaxBrightnessUnlimited 内部会触发 resetLimitedBrightness）
	m.setMaxBrightnessUnlimited(enabled)
}

// resetLimitedBrightness 根据缩放值变化调整亮度属性值
func (m *Manager) resetLimitedBrightness() {
	builtinMonitor := m.getBuiltinMonitor()
	if builtinMonitor == nil {
		return
	}
	currentBr := builtinMonitor.Brightness
	var newBr float64
	maxScale := brightness.GetCurrentMaxScale()
	if maxScale <= 100 {
		return
	}
	if m.MaxBrightnessUnlimited {
		newBr = currentBr * 100.0 / float64(maxScale)
	} else {
		newBr = currentBr * float64(maxScale) / 100.0
	}
	logger.Debugf("Updating brightness property for scale change: %f -> %f", currentBr, newBr)

	if newBr > 1.0 {
		newBr = 1.0
		m.SetBrightness(builtinMonitor.Name, newBr)
	} else {
		builtinMonitor.setPropBrightnessWithLock(newBr)
		m.syncPropBrightness()
	}
}

// setMaxBrightnessUnlimited 设置最大亮度不受限功能（DBus 属性写回调）
func (m *Manager) setMaxBrightnessUnlimited(enabled bool) error {
	logger.Infof("SetMaxBrightnessUnlimited called with: %v", enabled)

	brightness.SetDeviceBoardName(getDmiBoardName())
	boardSupported := brightness.IsDeviceSupported()
	if !boardSupported {
		logger.Warningf("Current board %s not match config, cannot enable max brightness limit", getDmiBoardName())
		return errors.New("board name mismatch: current board not supported")
	}

	maxScale := brightness.GetCurrentMaxScale()
	if maxScale <= 100 {
		logger.Warningf("Curve max scale too small: %d", maxScale)
		return errors.New("Curve config: max scale too low")
	}

	brightness.SetMaxBrightnessUnlimited(enabled)

	if m.MaxBrightnessUnlimited != enabled {
		m.MaxBrightnessUnlimited = enabled
		m.emitPropChangedMaxBrightnessUnlimited(enabled)
	}

	m.resetLimitedBrightness()

	return nil
}

// emitPropChangedMaxBrightnessUnlimited 发送 MaxBrightnessUnlimited 属性变化信号
func (m *Manager) emitPropChangedMaxBrightnessUnlimited(value bool) error {
	return m.service.EmitPropertyChanged(m, "MaxBrightnessUnlimited", value)
}

// setPropCurveMaxScale 设置 CurveMaxScale 属性
func (m *Manager) setPropCurveMaxScale(value int32) {
	if m.CurveMaxScale != value {
		m.CurveMaxScale = value
		m.emitPropChangedCurveMaxScale(value)
	}
}

func (m *Manager) emitPropChangedCurveMaxScale(value int32) error {
	return m.service.EmitPropertyChanged(m, "CurveMaxScale", value)
}

func (err InvalidOutputNameError) Error() string {
	return fmt.Sprintf("invalid output name %q", err.Name)
}

func (m *Manager) saveBrightnessInCfg(valueMap map[string]float64) error {
	if len(valueMap) == 0 {
		return nil
	}
	changed := false
	m.modifySuitableSysMonitorConfigs(func(configs SysMonitorConfigs) SysMonitorConfigs {
		for _, config := range configs {
			v, ok := valueMap[config.Name]
			if ok {
				config.Brightness = v
			} else {
				// 存在当从wayland切换到x11后，在wayland中设置过显示配置，此时配置文件中Name与切换到x11之后中的Name不匹配
				// 因此当失败时，在通过uuid查找一次，把Name改写，亮度不变
				monitors := m.getConnectedMonitors()
				for name := range valueMap {
					monitor := monitors.GetByName(name)
					if monitor == nil {
						logger.Warning("call GetByName failed: ", name)
						continue
					}

					if config.UUID == monitor.uuid {
						other := monitors.GetByUuidAndName(config.UUID, config.Name)
						if other != nil && other != monitor {
							// 存在其他的名字和UUID都对应配置的显示器，不要改该配置
							continue
						}
						config.Name = name
						config.Brightness = v
					}
				}
			}
			changed = true
		}
		return configs
	})

	if !changed {
		return nil
	}

	err := m.saveSysConfig("brightness changed")
	return err
}

func (m *Manager) changeBrightness(raised bool) error {
	var step = 0.05
	if m.MaxBacklightBrightness < 100 && m.MaxBacklightBrightness != 0 {
		step = 1 / float64(m.MaxBacklightBrightness)
	}
	if !raised {
		step = -step
	}

	monitors := m.getConnectedMonitors()

	successMap := make(map[string]float64)
	for _, monitor := range monitors {
		// 如果此显示器不支持亮度调节，则退出
		if ok, err := m.CanSetBrightness(monitor.Name); !ok {
			logger.Warning("call CanSetBrightness failed: ", err)
			continue
		}

		v, ok := m.Brightness[monitor.Name]
		if !ok {
			v = 1.0
		}

		var br float64
		br = v + step
		if br > 1.0 {
			br = 1.0
		}
		if br < 0.0 {
			br = 0.0
		}
		logger.Debug("[changeBrightness] will set to:", monitor.Name, br)
		err := m.setBrightnessAndSync(monitor.Name, br)
		if err != nil {
			logger.Warning(err)
			continue
		}
		successMap[monitor.Name] = br
	}
	err := m.saveBrightnessInCfg(successMap)
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

func (m *Manager) initBrightness() {
	m.Brightness = make(map[string]float64)
	monitors := m.getConnectedMonitors()
	monitorsId := monitors.getMonitorsId()
	configs := m.getSuitableSysMonitorConfigs(m.DisplayMode, monitorsId, monitors)
	for _, config := range configs {
		if config.Enabled {
			m.Brightness[config.Name] = scaleBrightness(config.Brightness, m.getBrightnessScale())
		}
	}
}

func (m *Manager) getSetterConfig() int {
	// NOTE: 特殊处理龙芯笔记本亮度设置问题
	blDir := "/sys/class/backlight/loongson"
	_, err := os.Stat(blDir)
	if err == nil {
		_, err := os.Stat(filepath.Join(blDir, "device/edid"))
		if err != nil {
			return brightness.SetterBacklight
		}
	}

	v, err := m.displayConfigMgr.Value(0, DSettingsKeyBrightnessSetter)
	if err != nil {
		logger.Warning(err)
		return brightness.SetterAuto
	}

	return int(v.Value().(int64))
}

// see also: gnome-desktop/libgnome-desktop/gnome-rr.c
//
//	'_gnome_rr_output_name_is_builtin_display'
func (m *Manager) isBuiltinMonitor(name string) bool {
	name = strings.ToLower(name)
	switch {
	case strings.HasPrefix(name, "vga"):
		return false
	case strings.HasPrefix(name, "hdmi"):
		return false

	case strings.HasPrefix(name, "dvi"):
		return true
	case strings.HasPrefix(name, "lvds"):
		// Most drivers use an "LVDS" prefix
		return true
	case strings.HasPrefix(name, "lcd"):
		// fglrx uses "LCD" in some versions
		return true
	case strings.HasPrefix(name, "edp"):
		// eDP is for internal built-in panel connections
		return true
	case strings.HasPrefix(name, "dsi"):
		return true
	case name == "default":
		return true
	}
	return false
}

func (m *Manager) setMonitorBrightness(monitor *Monitor, brightnessValue float64) error {
	// 根据dsg调整实际亮度百分比
	brightnessValue = math.Round(brightnessValue*float64(m.dsgBrightnessPercentage)) / 100.0
	logger.Debug("setMonitorBrightness reality value:", brightnessValue)

	setter := m.createBrightnessSetter(monitor)
	if setter == nil {
		return fmt.Errorf("failed to create brightness setter for monitor %s", monitor.Name)
	}
	return setter(brightnessValue)
}

func (m *Manager) createBrightnessSetter(monitor *Monitor) func(float64) error {
	isBuiltin := m.isBuiltinMonitor(monitor.Name)
	edid := utils.EncodeEdidBase64(monitor.edid)
	_uuid := monitor.uuid
	if _useWayland {
		_uuid = monitor.uuidV0
	}

	// 获取当前色温值，用于 gamma 设置路径
	temperature := m.getColorTemperatureValue()

	setter := m.getSetterConfig()

	var setterFunc func(float64) error

	switch setter {
	case brightness.SetterBacklight:
		setterFunc = func(brightnessValue float64) error {
			return brightness.SetBacklight(brightnessValue, edid)
		}
	case brightness.SetterAuto:
		if isBuiltin && brightness.SupportBacklight() {
			setterFunc = func(brightnessValue float64) error {
				return brightness.SetBacklight(brightnessValue, edid)
			}
		} else {
			setterFunc = func(brightnessValue float64) error {
				return brightness.SetOutputGama(brightnessValue, temperature, monitor.ID, m.xConn, _uuid)
			}
		}
	case brightness.SetterDDCCI:
		if isBuiltin {
			setterFunc = func(brightnessValue float64) error {
				return brightness.SetBacklight(brightnessValue, edid)
			}
		} else {
			setterFunc = func(brightnessValue float64) error {
				return brightness.SetDDCCIBrightness(brightnessValue, edid)
			}
		}
	case brightness.SetterGamma:
	case brightness.SetterDRM:
	default: // BrightnessSetterGamma
		setterFunc = func(brightnessValue float64) error {
			return brightness.SetOutputGama(brightnessValue, temperature, monitor.ID, m.xConn, _uuid)
		}
	}

	return setterFunc
}

// setColorTemperature 设置色温（通过 gamma）
func (m *Manager) setColorTemperature(monitor *Monitor, brightnessVal float64) error {
	temperature := m.getColorTemperatureValue()
	logger.Debug("setColorTemperature", monitor.Name, temperature)

	isBuiltin := m.isBuiltinMonitor(monitor.Name)
	_uuid := monitor.uuid
	if _useWayland {
		_uuid = monitor.uuidV0
	}

	// 内建显示器使用背光时，色温通过 gamma 设置（亮度为1）
	if isBuiltin && brightness.SupportBacklight() {
		brightnessVal = 1
	}

	return brightness.SetOutputGama(brightnessVal, temperature, monitor.ID, m.xConn, _uuid)
}

func (m *Manager) setBrightness(name string, value float64) error {
	m.brightnessWriteMu.Lock()
	defer m.brightnessWriteMu.Unlock()
	logger.Debug("Starting brightness setting", name, value)
	monitors := m.getConnectedMonitors()
	monitor := monitors.GetByName(name)
	if monitor == nil {
		logger.Debug("Monitor not found:", name)
		return InvalidOutputNameError{Name: name}
	}

	monitor.PropsMu.RLock()
	enabled := monitor.Enabled
	monitor.PropsMu.RUnlock()

	value = math.Round(value*1000) / 1000 // 通过该方法，用来对亮度值(亮度值范围为0-1)四舍五入保留小数点后三位有效数字
	if enabled {
		// 保持最小亮度，不能全黑
		if value <= 0.1 {
			value = 0.1
		} else if value > 1 {
			value = 1
		}

		err := m.setMonitorBrightness(monitor, value)
		if err != nil {
			logger.Warningf("failed to set brightness for %s: %v", name, err)
			return err
		}
	}

	monitor.setPropBrightnessWithLock(value)

	logger.Debug("end set brightness", name, value)

	return nil
}

func (m *Manager) setBrightnessAndSync(name string, value float64) error {
	err := m.setBrightness(name, value)
	if err == nil {
		m.syncPropBrightness()
	}
	return err
}

// getDefaultMonitorBrightness 获取默认显示器亮度（带 fallback 逻辑）
func (m *Manager) getDefaultMonitorBrightness(name string) float64 {
	if v, ok := m.Brightness[name]; ok {
		return v
	}
	if v, ok := m.Brightness["default"]; ok {
		return v
	}
	return 1
}

func (m *Manager) shouldUseDDCCIBrightness(name string) bool {
	return m.getSetterConfig() == brightness.SetterDDCCI && !m.isBuiltinMonitor(name)
}
