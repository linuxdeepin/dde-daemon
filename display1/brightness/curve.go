// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
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

// BrightnessPoint 亮度曲线点
type BrightnessPoint struct {
	Percentage int32 `json:"percentage"` // 亮度百分比 (0-100)
	Value      int32 `json:"value"`      // 对应的亮度百分比 (0-100)，表示最大亮度的百分比
}

// BrightnessCurveConfig 自定义亮度曲线配置
type BrightnessCurveConfig struct {
	EDID        string            `json:"edid,omitempty"` // 屏幕EDID标识（可选，默认曲线不需要）
	CurvePoints []BrightnessPoint `json:"curve_points"`   // 亮度曲线点
	MaxLimit    int32             `json:"max_limit"`      // 最大亮度限制百分比 (0-100)，表示最大亮度的百分比；0 表示不限制
	MaxScale    int32             `json:"max_scale"`      // 告知外部曲线最大比例
}

type curveFunc func(percentage float64, maxBrightness int32) int32

// CurveManager 管理亮度曲线的配置和计算
type CurveManager struct {
	mu sync.RWMutex // 统一的锁保护所有状态

	// 亮度限制功能使用的硬件信息
	configBoardName string
	deviceBoardName string
	productName     string

	// backLight-max-brightness-choose-big 命中的机型列表
	chooseBigProductNames []string

	// 亮度限制功能开关与缩放值
	maxBrightnessUnlimited bool
	maxScale               int32

	// 默认曲线函数
	defaultCurveFunc curveFunc

	// 受限曲线
	// 配置上可能会有多项，但只应用第一个匹配上的，因为多控制器可能导致亮度设置错乱
	limitedCurveFunc curveFunc
}

// 全局 CurveManager 实例
var _curveManager = &CurveManager{
	maxBrightnessUnlimited: true,
	maxScale:               DefaultScale,
	defaultCurveFunc:       nil,
	limitedCurveFunc:       nil,
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

// SetProductName 设置设备 ProductName，用于 backLight-max-brightness-choose-big 特征判断
func SetProductName(productName string) {
	_curveManager.setProductName(productName)
}

// SetChooseBigProductNames 设置 ProductName 命中后选最大背光亮度的机型列表
func SetChooseBigProductNames(list []string) {
	_curveManager.setChooseBigProductNames(list)
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

// interpolateCurveValue 对曲线点进行线性插值计算
// percentage: 输入百分比 (0-100)
// curvePoints: 曲线点数组
// 返回插值后的曲线值（百分比形式）
func interpolateCurveValue(percentage float64, curvePoints []BrightnessPoint) float64 {
	if len(curvePoints) == 0 {
		return 0
	}

	first := curvePoints[0]
	last := curvePoints[len(curvePoints)-1]
	if percentage <= float64(first.Percentage) {
		return float64(first.Value)
	}

	if percentage >= float64(last.Percentage) {
		return float64(last.Value)
	}

	// 线性插值
	curveValue := float64(last.Value)
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
	if curve.MaxLimit < 0 || curve.MaxLimit > 100 {
		return fmt.Errorf("max_limit must be in range [0,100], got: %d", curve.MaxLimit)
	}

	if curve.MaxScale < 0 {
		return fmt.Errorf("max_scale must not be negative, got: %d", curve.MaxScale)
	}

	if len(curve.CurvePoints) == 0 {
		return fmt.Errorf("curve_points cannot be empty")
	}

	for i, point := range curve.CurvePoints {
		if point.Percentage < 0 || point.Percentage > 100 {
			return fmt.Errorf("curve point percentage must be in range [0,100], got: %d", point.Percentage)
		}
		if point.Value < 0 || point.Value > 100 {
			return fmt.Errorf("curve point value must be in range [0,100], got: %d", point.Value)
		}

		// 验证曲线点非递减
		if i > 0 && point.Percentage < curve.CurvePoints[i-1].Percentage {
			return fmt.Errorf("curve points must be in non-descending order")
		}
	}

	return nil
}

// generateDefaultCurveFuncLocked 生成默认曲线函数（全局唯一）
// 注意：调用此方法前必须持有写锁
func (cm *CurveManager) generateDefaultCurveFuncLocked(curve BrightnessCurveConfig) {
	logger.Debugf("Generating default curve function with %d points", len(curve.CurvePoints))

	// 默认曲线函数：根据百分比和最大亮度计算实际亮度值
	cm.defaultCurveFunc = func(percentage float64, maxBrightness int32) int32 {
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

// unmarshalCurveJSON 解析曲线 JSON。
// 正常配置直接按 JSON 解析；同时兼容旧配置把 JSON 作为转义字符串传入的情况，
// 不再无条件删除所有反斜杠，避免破坏合法的 JSON 转义。
func unmarshalCurveJSON(jsonStr string, v interface{}) error {
	raw := []byte(jsonStr)
	if !json.Valid(raw) {
		var unescaped string
		if err := json.Unmarshal([]byte(`"`+jsonStr+`"`), &unescaped); err != nil {
			return fmt.Errorf("invalid JSON: %v", err)
		}
		raw = []byte(unescaped)
		if !json.Valid(raw) {
			return fmt.Errorf("invalid JSON")
		}
	}

	return json.Unmarshal(raw, v)
}

// parseCustomBrightnessCurves 解析自定义亮度曲线配置
func (cm *CurveManager) parseCustomBrightnessCurves(jsonStr string) error {
	var config struct {
		BoardName string                  `json:"boardName"`
		Curves    []BrightnessCurveConfig `json:"curves"`
	}

	logger.Debugf("Parsing custom brightness curves: %s", jsonStr)

	if err := unmarshalCurveJSON(jsonStr, &config); err != nil {
		return fmt.Errorf("Custom curve failed to parse JSON: %v", err)
	}

	// 存储 boardName
	cm.configBoardName = config.BoardName

	if config.Curves == nil {
		return fmt.Errorf("Custom curve json contains nothing")
	}

	controllers, err := displayBl.List()
	if err != nil {
		return fmt.Errorf("Custom curve failed to list controllers: %v", err)
	}

	for _, curve := range config.Curves {
		// 验证 EDID 字段不能为空
		if curve.EDID == "" {
			logger.Debug("Skipping curve config with empty EDID")
			continue
		}

		if err := validateBrightnessCurve(curve); err != nil {
			logger.Warningf("Curve validation failed for EDID %s: %v", curve.EDID, err)
			continue
		}

		for _, controller := range controllers {
			if matchControllerByEDID(controller, curve.EDID) {
				logger.Infof("Found matching controller for EDID %s: %s", curve.EDID, controller.Name)
				cm.limitedCurveFunc = cm.generateCustomCurveFunc(curve)
				cm.maxScale = curve.MaxScale
				return nil
			}
		}
	}

	logger.Debug("Custom curve config missed all controller")
	return nil
}

func (cm *CurveManager) generateCustomCurveFunc(curve BrightnessCurveConfig) curveFunc {
	return func(percentage float64, controllerMaxBrightness int32) int32 {
		// 使用统一的插值函数
		curveValue := interpolateCurveValue(percentage, curve.CurvePoints)

		// 应用max_limit限制
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

	cm.mu.RLock()
	supported := cm.isDeviceSupportedLocked()
	unlimited := cm.maxBrightnessUnlimited
	limitedFunc := cm.limitedCurveFunc
	defaultFunc := cm.defaultCurveFunc
	cm.mu.RUnlock()

	// 如果存在自定义曲线，检查适用性
	if !supported || unlimited || limitedFunc == nil {
		logger.Debugf("Limited curve not working now, maxBrightnessUnlimited=%v, func exist=%v", unlimited, limitedFunc != nil)
	} else {
		// 使用实际写入的控制器最大亮度进行换算，避免 EDID 匹配到的控制器
		// 与实际选中的背光控制器不一致时按错误的最大值缩放。
		return limitedFunc(vv, int32(c.MaxBrightness)), true
	}

	if defaultFunc != nil {
		return defaultFunc(vv, int32(c.MaxBrightness)), true
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

func (cm *CurveManager) setProductName(productName string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.productName = productName
}

func (cm *CurveManager) setChooseBigProductNames(list []string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.chooseBigProductNames = append([]string(nil), list...)
}

func (cm *CurveManager) isChooseBigProduct() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, name := range cm.chooseBigProductNames {
		if name == cm.productName {
			return true
		}
	}
	return false
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

// parseDefaultBrightnessCurve 解析默认亮度曲线配置
func (cm *CurveManager) parseDefaultBrightnessCurve(jsonStr string) error {
	logger.Debugf("Parsing default brightness curve: %s", jsonStr)

	var curve BrightnessCurveConfig
	if err := unmarshalCurveJSON(jsonStr, &curve); err != nil {
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
	if cm.configBoardName == "" {
		return true
	}

	if cm.deviceBoardName == "" {
		return false
	}

	if !strings.Contains(strings.ToLower(cm.configBoardName), strings.ToLower(cm.deviceBoardName)) {
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
