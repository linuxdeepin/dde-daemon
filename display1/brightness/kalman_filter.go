// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package brightness

import (
	"math"
)

// 卡尔曼滤波器默认参数
const (
	DefaultProcessNoiseQ     = 0.5  // 过程噪声协方差（系统模型的不确定性）
	DefaultMeasurementNoiseR = 0.02 // 测量噪声协方差（传感器噪声）
	DefaultWindowSize        = 5    // 自适应滤波器窗口大小
	DefaultVarianceScale     = 0.1  // 方差缩放系数（用于自适应调整R）
)

// KalmanFilter1D 一维卡尔曼滤波器
// 用于传感器数据的平滑和噪声抑制
type KalmanFilter1D struct {
	// 系统状态
	xEst float64 // 估计值
	PEst float64 // 估计协方差

	// 系统模型参数
	F float64 // 状态转移矩阵（通常为1，假设状态不变）
	H float64 // 观测矩阵（通常为1）
	Q float64 // 过程噪声协方差
	R float64 // 测量噪声协方差

	// 初始化标志
	initialized bool
}

// NewKalmanFilter1D 创建一维卡尔曼滤波器
// q: 过程噪声协方差（系统模型的不确定性）
// r: 测量噪声协方差（传感器噪声）
// initialValue: 初始估计值
func NewKalmanFilter1D(q, r, initialValue float64) *KalmanFilter1D {
	return &KalmanFilter1D{
		F:           1.0, // 假设状态不变
		H:           1.0, // 直接观测
		Q:           q,
		R:           r,
		xEst:        initialValue,
		PEst:        1.0,
		initialized: false,
	}
}

// Update 更新滤波器
// measurement: 测量值
// 返回: 滤波后的估计值
func (kf *KalmanFilter1D) Update(measurement float64) float64 {
	if !kf.initialized {
		kf.xEst = measurement
		kf.PEst = 1.0
		kf.initialized = true
		logger.Debugf("[AutoBrightness::KalmanFilter] Initialized with measurement: %d", int(measurement))
		return kf.xEst
	}

	// 记录输入值
	logger.Debugf("[AutoBrightness::KalmanFilter] Input measurement: %d, previous estimate: %d", int(measurement), int(kf.xEst))

	// 预测步骤
	xPred := kf.F * kf.xEst           // 状态预测
	pPred := kf.F*kf.PEst*kf.F + kf.Q // 协方差预测

	// 更新步骤
	y := measurement - kf.H*xPred // 测量残差
	s := kf.H*pPred*kf.H + kf.R   // 残差协方差
	k := pPred * kf.H / s         // 卡尔曼增益

	// 状态更新
	oldEstimate := kf.xEst
	kf.xEst = xPred + k*y
	kf.PEst = (1.0 - k*kf.H) * pPred

	// 记录输出值
	logger.Debugf("[AutoBrightness::KalmanFilter] Output estimate: %d (change: %.2f, gain: %.4f)",
		int(kf.xEst), kf.xEst-oldEstimate, k)

	return kf.xEst
}

// Reset 重置滤波器
func (kf *KalmanFilter1D) Reset() {
	kf.initialized = false
	kf.PEst = 1.0
}

// GetEstimate 获取当前估计值
func (kf *KalmanFilter1D) GetEstimate() float64 {
	return kf.xEst
}

// GetCovariance 获取当前协方差
func (kf *KalmanFilter1D) GetCovariance() float64 {
	return kf.PEst
}

// SetProcessNoise 设置过程噪声协方差
func (kf *KalmanFilter1D) SetProcessNoise(q float64) {
	kf.Q = q
}

// SetMeasurementNoise 设置测量噪声协方差
func (kf *KalmanFilter1D) SetMeasurementNoise(r float64) {
	kf.R = r
}

// AdaptiveKalmanFilter 自适应卡尔曼滤波器
// 根据测量值的方差自动调整噪声参数
type AdaptiveKalmanFilter struct {
	*KalmanFilter1D
	window              []float64 // 测量值窗口
	windowSize          int       // 窗口大小
	measurementVariance float64   // 测量方差
}

// NewAdaptiveKalmanFilter 创建自适应卡尔曼滤波器
// q: 过程噪声协方差
// r: 测量噪声协方差
// window: 滑动窗口大小（用于计算方差）
func NewAdaptiveKalmanFilter(q, r float64, window int) *AdaptiveKalmanFilter {
	return &AdaptiveKalmanFilter{
		KalmanFilter1D: NewKalmanFilter1D(q, r, 0.0),
		window:         make([]float64, 0, window),
		windowSize:     window,
	}
}

// NewDefaultAdaptiveKalmanFilter 使用默认参数创建自适应卡尔曼滤波器
func NewDefaultAdaptiveKalmanFilter() *AdaptiveKalmanFilter {
	return NewAdaptiveKalmanFilter(DefaultProcessNoiseQ, DefaultMeasurementNoiseR, DefaultWindowSize)
}

// Update 更新滤波器（自适应版本）
func (akf *AdaptiveKalmanFilter) Update(measurement float64) float64 {
	// 添加到窗口
	akf.window = append(akf.window, measurement)
	if len(akf.window) > akf.windowSize {
		akf.window = akf.window[1:]
	}

	logger.Debugf("[AutoBrightness::AdaptiveKalmanFilter] Window size: %d/%d, measurement: %d",
		len(akf.window), akf.windowSize, int(measurement))

	// 计算窗口内测量值的方差
	if len(akf.window) >= 2 {
		var sum float64
		for _, val := range akf.window {
			sum += val
		}
		mean := sum / float64(len(akf.window))

		var variance float64
		for _, val := range akf.window {
			diff := val - mean
			variance += diff * diff
		}
		akf.measurementVariance = variance / float64(len(akf.window))

		logger.Debugf("[AutoBrightness::AdaptiveKalmanFilter] Window stats: mean=%.2f, variance=%.4f",
			mean, akf.measurementVariance)

		// 根据测量方差自适应调整测量噪声
		// 方差越大，测量噪声越大，越信任估计值
		if akf.measurementVariance > 0 {
			oldR := akf.R
			newR := akf.measurementVariance * 0.1
			akf.SetMeasurementNoise(newR)
			logger.Debugf("[AutoBrightness::AdaptiveKalmanFilter] Adjusted measurement noise: R=%.4f -> %.4f",
				oldR, newR)
		}
	}

	// 调用基类的更新方法
	return akf.KalmanFilter1D.Update(measurement)
}

// Reset 重置滤波器
func (akf *AdaptiveKalmanFilter) Reset() {
	akf.KalmanFilter1D.Reset()
	akf.window = akf.window[:0]
	akf.measurementVariance = 0
}

// GetMeasurementVariance 获取当前测量方差
func (akf *AdaptiveKalmanFilter) GetMeasurementVariance() float64 {
	return akf.measurementVariance
}

// GetWindowSize 获取窗口大小
func (akf *AdaptiveKalmanFilter) GetWindowSize() int {
	return len(akf.window)
}

// IsInitialized 检查滤波器是否已初始化（是否收到过数据）
func (akf *AdaptiveKalmanFilter) IsInitialized() bool {
	return akf.initialized
}

// SetWindowSize 设置窗口大小
func (akf *AdaptiveKalmanFilter) SetWindowSize(size int) {
	if size < 2 {
		size = 2
	}
	akf.windowSize = size
	// 如果当前窗口超过新大小，截断
	if len(akf.window) > akf.windowSize {
		akf.window = akf.window[len(akf.window)-akf.windowSize:]
	}
}

// ExponentialMovingAverage 指数移动平均滤波器（保留用于对比）
type ExponentialMovingAverage struct {
	alpha       float64 // 平滑系数
	lastValue   float64 // 上一次的值
	initialized bool
}

// NewExponentialMovingAverage 创建指数移动平均滤波器
// alpha: 平滑系数 (0-1)，值越大对新数据响应越快
func NewExponentialMovingAverage(alpha float64) *ExponentialMovingAverage {
	return &ExponentialMovingAverage{
		alpha: alpha,
	}
}

// Update 更新滤波器
func (ema *ExponentialMovingAverage) Update(value float64) float64 {
	if !ema.initialized {
		ema.lastValue = value
		ema.initialized = true
		return value
	}

	ema.lastValue = ema.alpha*value + (1-ema.alpha)*ema.lastValue
	return ema.lastValue
}

// Reset 重置滤波器
func (ema *ExponentialMovingAverage) Reset() {
	ema.initialized = false
}

// GetValue 获取当前值
func (ema *ExponentialMovingAverage) GetValue() float64 {
	return ema.lastValue
}

// MovingAverage 移动平均滤波器
type MovingAverage struct {
	window     []float64
	windowSize int
	sum        float64
}

// NewMovingAverage 创建移动平均滤波器
func NewMovingAverage(windowSize int) *MovingAverage {
	return &MovingAverage{
		window:     make([]float64, 0, windowSize),
		windowSize: windowSize,
	}
}

// Update 更新滤波器
func (ma *MovingAverage) Update(value float64) float64 {
	ma.window = append(ma.window, value)
	ma.sum += value

	if len(ma.window) > ma.windowSize {
		ma.sum -= ma.window[0]
		ma.window = ma.window[1:]
	}

	return ma.sum / float64(len(ma.window))
}

// Reset 重置滤波器
func (ma *MovingAverage) Reset() {
	ma.window = ma.window[:0]
	ma.sum = 0
}

// GetValue 获取当前平均值
func (ma *MovingAverage) GetValue() float64 {
	if len(ma.window) == 0 {
		return 0
	}
	return ma.sum / float64(len(ma.window))
}

// clampFloat64 将值限制在指定范围内
func clampFloat64(value, min, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}
