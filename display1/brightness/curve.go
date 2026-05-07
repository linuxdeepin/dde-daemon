// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package brightness

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	displayBl "github.com/linuxdeepin/go-lib/backlight/display"
)

const (
	DefaultScale = 100
)

// 亮度曲线点
type BrightnessPoint struct {
	Percentage int32 `json:"percentage"` // 亮度百分比 (0-100)
	Value      int32 `json:"value"`      // 对应的亮度百分比 (0-100)，表示最大亮度的百分比
}

// AutoBrightnessCurvePoint 自动亮度曲线控制点
type AutoBrightnessCurvePoint struct {
	Lux int     `json:"lux"` // 光感值
	Br  float64 `json:"br"`  // 亮度百分比 (0-100)
}

// 自定义亮度曲线配置
type BrightnessCurveConfig struct {
	EDID        string            `json:"edid,omitempty"` // 屏幕EDID标识（可选，默认曲线不需要）
	CurvePoints []BrightnessPoint `json:"curve_points"`   // 亮度曲线点
	MaxLimit    int32             `json:"max_limit"`      // 最大亮度限制百分比 (0-100)，表示最大亮度的百分比
	MaxScale    int32             `json:"max_scale"`      // 告知外部曲线最大比例
}

// CurveManager 管理亮度曲线的配置和计算
type CurveManager struct {
	mu sync.RWMutex // 统一的锁保护所有状态

	// 亮度限制功能使用的硬件信息
	configBoardName string
	deviceBoardName string

	// 亮度限制功能开关与缩放值
	maxBrightnessUnlimited bool
	maxScale               int32

	// flm机型定制曲线
	flmCurveFunc func(percentage float64, maxBrightness int) int32

	// 默认曲线函数
	defaultCurveFunc func(percentage float64, maxBrightness int) int32

	// 受限曲线
	// 配置上可能会有多项，但只应用第一个匹配上的，因为多控制器可能导致亮度设置错乱
	limitedCurveFunc func(percentage float64) int32

	// 曲线类型
	curveType string

	// FLM曲线参数
	backLightMinValue int32
	backlightMidValue int32

	// 自动亮度曲线
	autoBrightnessCurve     []AutoBrightnessCurvePoint
	autoBrightnessCurveFunc func(lux int) float64
}

// 全局 CurveManager 实例
var _curveManager = &CurveManager{
	maxBrightnessUnlimited: true,
	maxScale:               DefaultScale,
	flmCurveFunc:           nil,
	defaultCurveFunc:       nil,
	limitedCurveFunc:       nil,
}

// InitFlmCurves 初始化FLM曲线
func InitFlmCurves(backLightMinValue int32, backlightMidValue int32) {
	_curveManager.initFlmCurves(backLightMinValue, backlightMidValue)
}

func SetCustomBrightnessCurves(jsonStr string) {
	_curveManager.setCustomBrightnessCurves(jsonStr)
}

func SetMaxBrightnessUnlimited(enabled bool) {
	_curveManager.setMaxBrightnessUnlimited(enabled)
}

func GetBacklightCurveValue(v float64, c *displayBl.Controller) (int32, bool) {
	return _curveManager.getCurveValue(v, c)
}

func SetDeviceBoardName(boardName string) {
	_curveManager.setDeviceBoardName(boardName)
}

// SetDefaultBrightnessCurve 设置默认亮度曲线（不依赖硬件信息）
func SetDefaultBrightnessCurve(jsonStr string) {
	_curveManager.setDefaultBrightnessCurve(jsonStr)
}

func IsDeviceSupported() bool {
	return _curveManager.isDeviceSupported()
}

func IsMaxLimitCurveSupported() bool {
	return _curveManager.isMaxLimitCurveSupported()
}

func GetCurrentMaxScale() int32 {
	return _curveManager.getCurrentMaxScale()
}

func SetCurveType(curveType string) {
	_curveManager.setCurveType(curveType)
}

// interpolateCurveValue 对曲线点进行线性插值计算
// percentage: 输入百分比 (0-100)
// curvePoints: 曲线点数组
// 返回插值后的曲线值（百分比形式）
func interpolateCurveValue(percentage float64, curvePoints []BrightnessPoint) float64 {
	if percentage <= 0 {
		return float64(curvePoints[0].Value)
	}

	if percentage >= 100 {
		return float64(curvePoints[len(curvePoints)-1].Value)
	}

	// 线性插值
	curveValue := float64(curvePoints[len(curvePoints)-1].Value)
	for i := 0; i < len(curvePoints)-1; i++ {
		p1, p2 := curvePoints[i], curvePoints[i+1]
		if percentage >= float64(p1.Percentage) && percentage <= float64(p2.Percentage) {
			x1, x2 := float64(p1.Percentage), float64(p2.Percentage)
			y1, y2 := float64(p1.Value), float64(p2.Value)
			if x2 == x1 {
				curveValue = y1
			} else {
				curveValue = y1 + (y2-y1)*(percentage-x1)/(x2-x1)
			}
			break
		}
	}

	return curveValue
}

func matchControllerByEDID(controller *displayBl.Controller, configEDID string) bool {
	if configEDID == "" || controller.DeviceEDID == nil {
		return false
	}

	// 厂商把产品型号作为字符串存于edid中，可以直接读到
	s := string(controller.DeviceEDID)
	// 检查配置中的EDID标识是否包含在完整的EDID中
	return strings.Contains(strings.ToLower(s), strings.ToLower(configEDID))
}

// 配置验证
func validateBrightnessCurve(curve BrightnessCurveConfig) error {
	if curve.MaxLimit < 0 {
		return fmt.Errorf("max_limit must be over 0, got: %d", curve.MaxLimit)
	}

	if len(curve.CurvePoints) == 0 {
		return fmt.Errorf("curve_points cannot be empty")
	}

	// 验证曲线点非递减
	for i := 1; i < len(curve.CurvePoints); i++ {
		if curve.CurvePoints[i].Percentage < curve.CurvePoints[i-1].Percentage {
			return fmt.Errorf("curve points must be in non-descending order")
		}
	}

	return nil
}

func (cm *CurveManager) initFlmCurves(backLightMinValue int32, backlightMidValue int32) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 保存FLM曲线参数
	cm.backLightMinValue = backLightMinValue
	cm.backlightMidValue = backlightMidValue

	// 生成FLM曲线
	cm.generateFlmCurveLocked(backLightMinValue, backlightMidValue)
}

// generateFlmCurveLocked 生成FLM曲线函数（全局唯一）
// 注意：调用此方法前必须持有写锁
func (cm *CurveManager) generateFlmCurveLocked(backLightMinValue int32, backlightMidValue int32) {
	logger.Debugf("Generating FLM curve function with minValue=%d, midValue=%d", backLightMinValue, backlightMidValue)

	const (
		x1    = 10.0
		power = 2.2
	)

	y1 := float64(backLightMinValue)
	x2 := 100.0

	// FLM曲线函数：根据百分比和最大亮度计算实际亮度值
	cm.flmCurveFunc = func(percentage float64, maxBrightness int) int32 {
		y2 := float64(maxBrightness)
		base := x2 - x1
		yAdjusted := y2 - y1
		a := yAdjusted / math.Pow(base, power)

		// 2.2次函数
		func2p2 := func(x float64) int32 {
			return int32(a*math.Pow(x-x1, power) + y1)
		}

		// 线性函数
		k := (float64(func2p2(float64(backlightMidValue))) - y1) / float64(backlightMidValue-x1)
		b := y1 - k*x1
		linearFunc := func(x float64) int32 {
			return int32(k*x + b)
		}

		// 分段计算
		if percentage <= x1 {
			return backLightMinValue
		} else if percentage > x1 && percentage <= float64(backlightMidValue) {
			return linearFunc(percentage)
		} else if percentage > float64(backlightMidValue) && percentage <= x2 {
			return func2p2(percentage)
		}
		return int32(maxBrightness)
	}

	logger.Debug("FLM curve function generated")
}

// generateDefaultCurveFuncLocked 生成默认曲线函数（全局唯一，类似FLM曲线）
// 注意：调用此方法前必须持有写锁
func (cm *CurveManager) generateDefaultCurveFuncLocked(curve BrightnessCurveConfig) {
	logger.Debugf("Generating default curve function with %d points", len(curve.CurvePoints))

	// 默认曲线函数：根据百分比和最大亮度计算实际亮度值
	cm.defaultCurveFunc = func(percentage float64, maxBrightness int) int32 {
		// 使用统一的插值函数
		curveValue := interpolateCurveValue(percentage, curve.CurvePoints)

		// 默认曲线不使用 max_limit，曲线点已经定义了完整的映射关系

		// 转换为实际亮度值
		return int32(math.Round(curveValue * float64(maxBrightness) / 100.0))
	}

	logger.Debug("Default curve function generated")
}

// setCustomBrightnessCurves 设置自定义亮度曲线配置（内部方法）
func (cm *CurveManager) setCustomBrightnessCurves(jsonStr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.limitedCurveFunc = nil

	// 解析配置
	err := cm.parseCustomBrightnessCurves(jsonStr)
	if err != nil {
		logger.Debugf("Failed to parse custom brightness curves: %v", err)
		return
	}

	logger.Info("Custom brightness curves set successfully")
}

// setMaxBrightnessUnlimited 设置最大亮度不受限制（内部方法）
func (cm *CurveManager) setMaxBrightnessUnlimited(enabled bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.maxBrightnessUnlimited = enabled
	logger.Infof("Set max brightness unlimited: %v", enabled)
}

// isMaxBrightnessUnlimited 检查是否启用最大亮度不受限制（内部方法）
func (cm *CurveManager) isMaxBrightnessUnlimited() bool {
	return cm.maxBrightnessUnlimited
}

// ParseCustomBrightnessCurves 解析自定义亮度曲线配置
func (cm *CurveManager) parseCustomBrightnessCurves(jsonStr string) error {
	var config struct {
		BoardName string                  `json:"boardName"`
		Curves    []BrightnessCurveConfig `json:"curves"`
	}

	// Remove any backslash characters from the input JSON string
	jsonStr = strings.ReplaceAll(jsonStr, "\\", "")

	logger.Debugf("Parsing custom brightness curves: %s", jsonStr)

	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return fmt.Errorf("Custom curve failed to parse JSON: %v", err)
	}

	// 存储 boardName
	cm.mu.Lock()
	cm.configBoardName = config.BoardName
	cm.mu.Unlock()

	if config.Curves == nil {
		return fmt.Errorf("Custom curve json contains nothing")
	}

	ctlrs, err := displayBl.List()
	if err != nil {
		return fmt.Errorf("Custom curve failed to list controllers: %v", err)
	}

	for _, curve := range config.Curves {
		// 验证 EDID 字段不能为空
		if curve.EDID == "" {
			logger.Debug("Skipping curve config with empty EDID")
			continue
		}

		for _, controller := range ctlrs {
			if matchControllerByEDID(controller, curve.EDID) {
				logger.Infof("Found matching controller for EDID %s: %s", curve.EDID, controller.Name)

				if err := validateBrightnessCurve(curve); err == nil {
					cm.limitedCurveFunc = cm.generateCustomCurveFunc(curve, int32(controller.MaxBrightness))
					cm.maxScale = curve.MaxScale
					return nil
				}
			}
		}
	}

	logger.Debug("Custom curve config missed all controller")
	return nil
}

func (cm *CurveManager) generateCustomCurveFunc(curve BrightnessCurveConfig, controllerMaxBrightness int32) func(float64) int32 {
	return func(percentage float64) int32 {
		// 使用统一的插值函数
		curveValue := interpolateCurveValue(percentage, curve.CurvePoints)

		// 应用max_limit限制
		// 注意：这里直接读取全局 _curveManager 的状态
		cm.mu.RLock()
		unlimited := cm.maxBrightnessUnlimited
		cm.mu.RUnlock()

		if !unlimited && curve.MaxLimit > 0 && curve.MaxLimit < 100 {
			curveValue = curveValue * float64(curve.MaxLimit) / 100.0
		}

		// 转换为实际亮度值
		return int32(math.Round(curveValue * float64(controllerMaxBrightness) / 100.0))
	}
}

// 根据当前设置亮度百分比与控制器，返回由曲线控制的实际亮度
func (cm *CurveManager) getCurveValue(v float64, c *displayBl.Controller) (int32, bool) {
	// 将v转为合适的值，注意精度，不要过早转为整数
	vv := 100 * v

	if cm.curveType == "flm" {
		if cm.flmCurveFunc == nil {
			logger.Warning("FLM curve function is not initialized")
			return int32(v * float64(c.MaxBrightness)), false
		}

		return cm.flmCurveFunc(vv, c.MaxBrightness), true
	}

	// 如果存在自定义曲线，检查适用性
	if !cm.isDeviceSupportedLocked() || cm.maxBrightnessUnlimited || cm.limitedCurveFunc == nil {
		logger.Debugf("Limited curve not working now, maxBrightnessUnlimited=%v, func exist=%v", cm.maxBrightnessUnlimited, cm.limitedCurveFunc != nil)
	} else {
		return cm.limitedCurveFunc(vv), true
	}

	if cm.defaultCurveFunc != nil {
		return cm.defaultCurveFunc(vv, c.MaxBrightness), true
	}

	// 默认无处理的值
	return int32(v * float64(c.MaxBrightness)), false
}

// setDeviceBoardName 设置设备板名（内部方法）
func (cm *CurveManager) setDeviceBoardName(boardName string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.deviceBoardName = boardName
}

// setDefaultBrightnessCurve 设置默认亮度曲线（内部方法）
func (cm *CurveManager) setDefaultBrightnessCurve(jsonStr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.defaultCurveFunc = nil

	err := cm.parseDefaultBrightnessCurve(jsonStr)
	if err != nil {
		logger.Warningf("Failed to parse default brightness curve: %v", err)
		return
	}

	logger.Info("Default brightness curve set successfully")
}

// ParseDefaultBrightnessCurve 解析默认亮度曲线配置
func (cm *CurveManager) parseDefaultBrightnessCurve(jsonStr string) error {
	// Remove any backslash characters from the input JSON string
	jsonStr = strings.ReplaceAll(jsonStr, "\\", "")

	logger.Debugf("Parsing default brightness curve: %s", jsonStr)

	var curve BrightnessCurveConfig
	if err := json.Unmarshal([]byte(jsonStr), &curve); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	// 验证曲线配置
	if err := validateBrightnessCurve(curve); err != nil {
		return fmt.Errorf("invalid curve config: %v", err)
	}

	cm.generateDefaultCurveFuncLocked(curve)

	logger.Infof("Default brightness curve parsed successfully: %d points", len(curve.CurvePoints))

	return nil
}

// isDeviceSupported 检查设备是否支持（内部方法）
func (cm *CurveManager) isDeviceSupported() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.isDeviceSupportedLocked()
}

// isDeviceSupportedLocked 检查设备是否支持（需要持有锁）
func (cm *CurveManager) isDeviceSupportedLocked() bool {
	if cm.configBoardName != "" && !strings.Contains(strings.ToLower(cm.configBoardName), strings.ToLower(cm.deviceBoardName)) {
		return false
	}

	return true
}

// isMaxLimitCurveSupported 检查是否支持最大亮度限制曲线（内部方法）
func (cm *CurveManager) isMaxLimitCurveSupported() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.isDeviceSupportedLocked() {
		return false
	}

	// 检查是否有任何控制器在自定义曲线映射中存在
	return cm.limitedCurveFunc != nil
}

// getCurrentMaxScale 获取当前最大缩放值（内部方法）
func (cm *CurveManager) getCurrentMaxScale() int32 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 如果没有自定义曲线配置，返回默认值 100（表示100%）
	if cm.limitedCurveFunc == nil || !cm.isDeviceSupportedLocked() {
		return DefaultScale
	}

	if cm.maxScale > DefaultScale {
		return cm.maxScale
	}

	// 如果没有设置 MaxScale，返回默认值 100（表示100%）
	return DefaultScale
}

// setCurveType 设置曲线类型（内部方法）
func (cm *CurveManager) setCurveType(curveType string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.curveType = curveType
	logger.Infof("Set curve type: %s", curveType)
}

// SetAutoBrightnessCurve 设置自动亮度曲线配置（JSON字符串）
func SetAutoBrightnessCurve(jsonStr string) {
	_curveManager.setAutoBrightnessCurve(jsonStr)
}

// SetAutoBrightnessCurveFromPoints 设置自动亮度曲线配置（数组）
func SetAutoBrightnessCurveFromPoints(points []AutoBrightnessCurvePoint) {
	_curveManager.setAutoBrightnessCurveFromPoints(points)
}

// GetAutoBrightnessValue 根据光照值获取目标亮度百分比 (0-1)
func GetAutoBrightnessValue(lux int) float64 {
	return _curveManager.getAutoBrightnessValue(lux)
}

// HasAutoBrightnessCurve 检查是否配置了自动亮度曲线
func HasAutoBrightnessCurve() bool {
	return _curveManager.hasAutoBrightnessCurve()
}

// setAutoBrightnessCurve 设置自动亮度曲线（内部方法，从JSON字符串解析）
func (cm *CurveManager) setAutoBrightnessCurve(jsonStr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.autoBrightnessCurveFunc = nil
	cm.autoBrightnessCurve = nil

	if jsonStr == "" {
		logger.Info("Auto brightness curve cleared")
		return
	}

	jsonStr = strings.ReplaceAll(jsonStr, "\\", "")
	logger.Debugf("Parsing auto brightness curve: %s", jsonStr)

	var points []AutoBrightnessCurvePoint
	if err := json.Unmarshal([]byte(jsonStr), &points); err != nil {
		logger.Warningf("Failed to parse auto brightness curve: %v", err)
		return
	}

	cm.setAutoBrightnessCurveFromPointsLocked(points)
}

// setAutoBrightnessCurveFromPoints 设置自动亮度曲线（内部方法，从数组）
func (cm *CurveManager) setAutoBrightnessCurveFromPoints(points []AutoBrightnessCurvePoint) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.setAutoBrightnessCurveFromPointsLocked(points)
}

// setAutoBrightnessCurveFromPointsLocked 设置自动亮度曲线（内部方法，需持有锁）
func (cm *CurveManager) setAutoBrightnessCurveFromPointsLocked(points []AutoBrightnessCurvePoint) {
	cm.autoBrightnessCurveFunc = nil
	cm.autoBrightnessCurve = nil

	if len(points) == 0 {
		logger.Info("Auto brightness curve cleared")
		return
	}

	if err := validateAutoBrightnessCurve(points); err != nil {
		logger.Warningf("Invalid auto brightness curve: %v", err)
		return
	}

	cm.autoBrightnessCurve = points
	cm.autoBrightnessCurveFunc = cm.generateAutoBrightnessCurveFunc(points)
	logger.Infof("Auto brightness curve set successfully with %d points", len(points))
}

// validateAutoBrightnessCurve 验证自动亮度曲线配置
func validateAutoBrightnessCurve(points []AutoBrightnessCurvePoint) error {
	if len(points) == 0 {
		return fmt.Errorf("curve points cannot be empty")
	}

	for i, p := range points {
		if p.Br < 0 || p.Br > 100 {
			return fmt.Errorf("brightness at index %d must be between 0 and 100, got: %f", i, p.Br)
		}
		if i > 0 && p.Lux <= points[i-1].Lux {
			return fmt.Errorf("lux values must be in ascending order at index %d", i)
		}
	}

	return nil
}

// generateAutoBrightnessCurveFunc 生成自动亮度曲线函数
func (cm *CurveManager) generateAutoBrightnessCurveFunc(points []AutoBrightnessCurvePoint) func(lux int) float64 {
	return func(lux int) float64 {
		if len(points) == 0 {
			return -1
		}

		if len(points) == 1 {
			return points[0].Br / 100.0
		}

		luxFloat := float64(lux)

		if luxFloat <= float64(points[0].Lux) {
			return points[0].Br / 100.0
		}

		if luxFloat >= float64(points[len(points)-1].Lux) {
			return points[len(points)-1].Br / 100.0
		}

		for i := 0; i < len(points)-1; i++ {
			p1, p2 := points[i], points[i+1]
			if luxFloat >= float64(p1.Lux) && luxFloat <= float64(p2.Lux) {
				x1, x2 := float64(p1.Lux), float64(p2.Lux)
				y1, y2 := p1.Br, p2.Br
				br := y1 + (y2-y1)*(luxFloat-x1)/(x2-x1)
				return br / 100.0
			}
		}

		return points[len(points)-1].Br / 100.0
	}
}

// getAutoBrightnessValue 根据光照值获取目标亮度百分比 (0-1)
func (cm *CurveManager) getAutoBrightnessValue(lux int) float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.autoBrightnessCurveFunc == nil {
		return -1
	}

	return cm.autoBrightnessCurveFunc(lux)
}

// hasAutoBrightnessCurve 检查是否配置了自动亮度曲线
func (cm *CurveManager) hasAutoBrightnessCurve() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.autoBrightnessCurveFunc != nil
}
