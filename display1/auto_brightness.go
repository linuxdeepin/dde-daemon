// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package display1

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"

	"github.com/linuxdeepin/dde-daemon/display1/brightness"
)

const (
	compensationInterval  = 300 * time.Millisecond
	sensorDataTimeout     = 1 * time.Second
	compensationThreshold = 5.0
)

// AutoBrightnessConfig 自动亮度配置结构体
type AutoBrightnessConfig struct {
	Enabled                      bool    `json:"enabled"`                          // 是否启用自动亮度
	Sensitivity                  float64 `json:"sensitivity"`                      // 敏感度 (0.1-3.0)
	PollingInterval              int     `json:"polling_interval"`                 // 轮询间隔(秒) (1-60)
	ChangeThreshold              float64 `json:"change_threshold"`                 // 变化阈值 (1.0-50.0)
	BrightnessChangeThreshold    float64 `json:"brightness_change_threshold"`      // 亮度变化阈值 (0.01-1.0)
	ManualOverrideDuration       int     `json:"manual_override_duration"`         // 手动调节暂停时间(秒) (60-1800)
	ManualAdjustDisablesAutoMode bool    `json:"manual_adjust_disables_auto_mode"` // 手动调节是否禁用自动模式
	UseTransition                bool    `json:"use_transition"`                   // 自动调节时是否使用渐变效果
	KalmanProcessNoise           float64 `json:"kalman_process_noise"`             // 卡尔曼滤波器过程噪声协方差 Q
	KalmanMeasurementNoise       float64 `json:"kalman_measurement_noise"`         // 卡尔曼滤波器测量噪声协方差 R
	KalmanWindowSize             int     `json:"kalman_window_size"`               // 卡尔曼滤波器窗口大小
}

// DefaultAutoBrightnessConfig 默认自动亮度配置
var DefaultAutoBrightnessConfig = AutoBrightnessConfig{
	Enabled:                      false,
	Sensitivity:                  0.5,
	PollingInterval:              5,
	ChangeThreshold:              20.0,
	BrightnessChangeThreshold:    0.01,
	ManualOverrideDuration:       300,
	ManualAdjustDisablesAutoMode: true,
	UseTransition:                true,
	KalmanProcessNoise:           0.8,
	KalmanMeasurementNoise:       0.05,
	KalmanWindowSize:             3,
}

// DSettings键名已在manager.go中定义
// AutoBrightnessManager 自动亮度管理器
type AutoBrightnessManager struct {
	manager *Manager

	sensorClient *SensorProxyClient

	config        AutoBrightnessConfig
	configManager configManager.Manager
	sysBus        *dbus.Conn

	enabled        bool
	supported      bool
	manualOverride time.Time
	lastLightLevel int
	lastAdjustTime time.Time
	lastBrightness float64

	pollInterval time.Duration
	stopChan     chan struct{}
	ticker       *time.Ticker
	pollingWg    sync.WaitGroup

	kalmanFilter *brightness.AdaptiveKalmanFilter

	systemAdjusting bool
	running         bool
	polling         bool

	lastSensorDataTime time.Time
	compensationTicker *time.Ticker
	compensationStopCh chan struct{}
	compensationWg     sync.WaitGroup

	// 同步控制
	mutex sync.RWMutex

	// 重试和降级配置
	maxRetries          int
	retryInterval       time.Duration
	gracefulDegradation bool
}

// NewAutoBrightnessManager 创建新的自动亮度管理器
func NewAutoBrightnessManager() *AutoBrightnessManager {
	return &AutoBrightnessManager{
		lastLightLevel:      -1, // 初始化为无效值
		lastBrightness:      -1, // 初始化为无效值
		maxRetries:          3,
		retryInterval:       time.Second * 2,
		gracefulDegradation: true,
		stopChan:            make(chan struct{}), // 初始化时创建
	}
}

// Initialize 初始化自动亮度管理器
func (abm *AutoBrightnessManager) Initialize(manager *Manager) error {
	if manager == nil {
		return errors.New("manager cannot be nil")
	}

	logger.Info("[AutoBrightness] Initializing AutoBrightnessManager")

	abm.mutex.Lock()
	abm.manager = manager
	abm.sysBus = manager.sysBus
	abm.mutex.Unlock()

	err := abm.initConfigManager()
	if err != nil {
		logger.Warning("[AutoBrightness] Failed to initialize config manager:", err)
	}

	builtinMonitor := manager.getBuiltinMonitor()
	if builtinMonitor == nil {
		abm.mutex.Lock()
		abm.supported = false
		abm.mutex.Unlock()
		return fmt.Errorf("no builtin monitor found (total monitors: %d)", len(manager.getConnectedMonitors()))
	}

	canSet, _ := manager.CanSetBrightness(builtinMonitor.Name)
	if !canSet {
		abm.mutex.Lock()
		abm.supported = false
		abm.mutex.Unlock()
		return fmt.Errorf("cannot set brightness for builtin monitor: %s", builtinMonitor.Name)
	}

	sensorClient := NewSensorProxyClient(manager.sysBus)

	abm.mutex.Lock()
	abm.sensorClient = sensorClient
	abm.mutex.Unlock()

	err = abm.checkSensorAvailability()
	if err != nil {
		abm.mutex.Lock()
		abm.supported = false
		abm.mutex.Unlock()
		logger.Warning("[AutoBrightness] Sensor not available:", err)
		return fmt.Errorf("sensor not available: %w", err)
	}

	sensorClient.SetServiceChangeCallback(abm.onServiceChange)
	sensorClient.SetLightLevelChangeCallback(abm.onLightLevelChange)

	abm.mutex.Lock()
	config, err := abm.getConfig()
	if err != nil {
		logger.Warning("[AutoBrightness] Failed to load config, using default:", err)
		config = DefaultAutoBrightnessConfig
	}
	abm.config = config
	abm.supported = true
	abm.applyKalmanFilterConfig()
	abm.pollInterval = time.Duration(abm.config.PollingInterval) * time.Second
	abm.mutex.Unlock()

	logger.Info("[AutoBrightness] AutoBrightnessManager initialized successfully")
	return nil
}

// Start 启动自动亮度功能
func (abm *AutoBrightnessManager) Start() error {
	abm.mutex.Lock()
	if !abm.supported {
		abm.mutex.Unlock()
		return errors.New("auto brightness not supported")
	}

	abm.resetHistoryState()
	abm.manualOverride = time.Time{}

	if abm.running {
		abm.mutex.Unlock()
		return nil
	}

	if !abm.config.Enabled {
		abm.mutex.Unlock()
		return nil
	}

	sensorClient := abm.sensorClient
	manager := abm.manager
	abm.mutex.Unlock()

	err := sensorClient.Connect()
	if err != nil {
		logger.Warning("[AutoBrightness] Failed to connect to sensor proxy:", err)
		return fmt.Errorf("failed to connect to sensor proxy: %w", err)
	}

	err = abm.claimLightWithRetry()
	if err != nil {
		logger.Warning("[AutoBrightness] Failed to claim light sensor after retries:", err)
		sensorClient.Disconnect()
		return fmt.Errorf("failed to claim light sensor: %w", err)
	}

	abm.mutex.Lock()
	abm.running = true
	abm.enabled = true
	abm.lastSensorDataTime = time.Now()
	powerSaving := manager.powerSaving
	sessionActive := manager.sessionActive
	if !powerSaving && sessionActive {
		abm.startPolling()
		abm.startCompensationTimer()
	} else {
		logger.Warningf("[AutoBrightness] Started but powerSaving [%v], session active [%v]", powerSaving, sessionActive)
	}
	abm.mutex.Unlock()

	logger.Info("[AutoBrightness] AutoBrightnessManager started successfully")
	return nil
}

// Stop 停止自动亮度功能
func (abm *AutoBrightnessManager) Stop() error {
	abm.mutex.Lock()

	if !abm.running {
		abm.mutex.Unlock()
		return nil
	}

	abm.stopPolling()
	abm.stopCompensationTimer()
	abm.restoreSavedBrightness()

	if abm.sensorClient != nil {
		err := abm.sensorClient.ReleaseLight()
		if err != nil {
			logger.Warning("[AutoBrightness] Failed to release light sensor:", err)
		}

		err = abm.sensorClient.Disconnect()
		if err != nil {
			logger.Warning("[AutoBrightness] Failed to disconnect from sensor proxy:", err)
		}
	}

	abm.running = false
	abm.enabled = false
	abm.mutex.Unlock()

	logger.Info("[AutoBrightness] AutoBrightnessManager stopped successfully")
	return nil
}

// Cleanup 清理资源
func (abm *AutoBrightnessManager) Cleanup() error {
	abm.mutex.Lock()
	defer abm.mutex.Unlock()
	var cleanupErrors []error
	// 停止运行
	if abm.running {
		abm.stopPolling()
		abm.stopCompensationTimer()
		abm.running = false
	}
	// 断开传感器连接
	if abm.sensorClient != nil {
		err := abm.sensorClient.Disconnect()
		if err != nil {
			logger.Warning("[AutoBrightness] Failed to disconnect sensor client:", err)
			cleanupErrors = append(cleanupErrors, err)
		}
		abm.sensorClient = nil
	}
	// 注意：stopChan 和 transitionStop 不需要关闭，它们会一直存在直到对象被GC
	logger.Info("[AutoBrightness] AutoBrightnessManager cleaned up")
	if len(cleanupErrors) > 0 {
		return cleanupErrors[0] // 返回第一个错误
	}
	return nil
}

// SetEnabled 设置启用状态
func (abm *AutoBrightnessManager) SetEnabled(enabled bool) error {
	abm.mutex.Lock()
	if !abm.supported {
		abm.mutex.Unlock()
		return errors.New("[AutoBrightness] Not supported")
	}
	// 更新配置
	abm.config.Enabled = enabled
	needStart := enabled && !abm.running
	needStop := !enabled && abm.running
	abm.mutex.Unlock()
	// 保存配置
	err := abm.saveConfig()
	if err != nil {
		logger.Warning("[AutoBrightness] Failed to save config:", err)
	}
	// 在锁外执行启动/停止操作
	if needStart {
		return abm.Start()
	} else if needStop {
		return abm.Stop()
	}
	return nil
}

// setSystemAdjusting 设置系统调整标志--用于节能模式或类似功能
func (abm *AutoBrightnessManager) setSystemAdjusting(adjusting bool) {
	abm.mutex.Lock()
	defer abm.mutex.Unlock()
	abm.systemAdjusting = adjusting
	logger.Debugf("[AutoBrightness] System adjusting flag set to: %v", adjusting)
}

// OnManualBrightnessChange 处理手动亮度调节
func (abm *AutoBrightnessManager) OnManualBrightnessChange() {
	abm.mutex.Lock()
	defer abm.mutex.Unlock()
	// 如果是系统自动调整（如节能模式），忽略此次调用
	if abm.systemAdjusting {
		logger.Debug("[AutoBrightness] Ignoring brightness change from system adjustment")
		return
	}
	if !abm.running || !abm.polling {
		logger.Info("[AutoBrightness] Manual brightness change")
		return
	}
	// 检查配置：手动调节是否禁用自动模式
	if abm.config.ManualAdjustDisablesAutoMode {
		logger.Info("[AutoBrightness] Manual brightness change detected, disabling auto brightness mode")
		// 禁用自动亮度功能
		abm.config.Enabled = false
		// 异步保存配置并停止功能
		go func() {
			err := abm.saveConfig()
			if err != nil {
				logger.Warning("[AutoBrightness] Failed to save config:", err)
			}
			// 停止自动亮度功能
			err = abm.Stop()
			if err != nil {
				logger.Warning("[AutoBrightness] Failed to stop auto brightness:", err)
			}
			// 更新 Manager 属性
			if abm.manager != nil {
				abm.manager.setPropAutoBrightnessEnabled(false)
			}
		}()
		return
	}
	// 默认行为：临时暂停
	abm.resetHistoryState()
	abm.manualOverride = time.Now()
	logger.Infof("[AutoBrightness] Manual brightness change detected, pausing auto adjustment for %d seconds", abm.config.ManualOverrideDuration)
	// 释放传感器，停止收集亮度数据
	if abm.sensorClient != nil && abm.sensorClient.IsClaimed() {
		err := abm.sensorClient.ReleaseLight()
		if err != nil {
			logger.Warning("[AutoBrightness] Failed to release light sensor on manual override:", err)
		}
	}
}

// isManualOverrideActive 检查手动调节是否仍在生效
func (abm *AutoBrightnessManager) isManualOverrideActive() bool {
	abm.mutex.RLock()
	defer abm.mutex.RUnlock()
	if abm.manualOverride.IsZero() {
		return false
	}
	duration := time.Duration(abm.config.ManualOverrideDuration) * time.Second
	return time.Since(abm.manualOverride) < duration
}

// resetHistoryState 重置历史状态，使下次轮询能立即触发亮度调节
// 注意：此函数假设调用者已经持有锁
func (abm *AutoBrightnessManager) resetHistoryState() {
	abm.lastLightLevel = -1
	abm.lastBrightness = -1
	abm.lastAdjustTime = time.Time{}
}

// hold 暂停自动亮度调节
func (abm *AutoBrightnessManager) hold() {
	abm.mutex.Lock()
	defer abm.mutex.Unlock()
	if !abm.running {
		return
	}

	abm.stopPolling()
	abm.stopCompensationTimer()
}

// resume 恢复自动亮度调节
func (abm *AutoBrightnessManager) resume() {
	abm.mutex.Lock()
	defer abm.mutex.Unlock()
	if !abm.running {
		return
	}

	abm.resetHistoryState()
	abm.startPolling()
	abm.startCompensationTimer()
}

// OnConfigChanged 处理配置变更
func (abm *AutoBrightnessManager) OnConfigChanged(config AutoBrightnessConfig) {
	abm.mutex.Lock()
	oldEnabled := abm.config.Enabled
	oldSensitivity := abm.config.Sensitivity
	abm.config = config
	abm.pollInterval = time.Duration(config.PollingInterval) * time.Second
	needStart := config.Enabled && !oldEnabled && !abm.running
	needStop := !config.Enabled && oldEnabled && abm.running
	needRestart := config.Enabled == oldEnabled && abm.running

	sensitivityChanged := oldSensitivity != config.Sensitivity
	needImmediateAdjust := sensitivityChanged && abm.running && config.Enabled

	abm.applyKalmanFilterConfig()

	if needRestart {
		abm.stopPolling()
		abm.stopCompensationTimer()
		abm.startPolling()
		abm.startCompensationTimer()
	}

	abm.mutex.Unlock()

	if needStart {
		abm.Start()
	} else if needStop {
		abm.Stop()
	}

	if needImmediateAdjust {
		logger.Infof("[AutoBrightness] Sensitivity changed from %.2f to %.2f, triggering immediate adjustment", oldSensitivity, config.Sensitivity)
		go abm.adjustBrightnessOnce()
	}
	logger.Infof("[AutoBrightness] Config updated: enabled=%v, sensitivity=%.2f, polling=%ds, threshold=%.1f",
		config.Enabled, config.Sensitivity, config.PollingInterval, config.ChangeThreshold)
}

// applyKalmanFilterConfig 应用卡尔曼滤波器配置
func (abm *AutoBrightnessManager) applyKalmanFilterConfig() {
	if abm.kalmanFilter == nil {
		abm.kalmanFilter = brightness.NewAdaptiveKalmanFilter(
			abm.config.KalmanProcessNoise,
			abm.config.KalmanMeasurementNoise,
			abm.config.KalmanWindowSize,
		)
		logger.Debug("[AutoBrightness] Kalman filter created with config params")
		return
	}

	abm.kalmanFilter.SetProcessNoise(abm.config.KalmanProcessNoise)
	abm.kalmanFilter.SetMeasurementNoise(abm.config.KalmanMeasurementNoise)
	abm.kalmanFilter.SetWindowSize(abm.config.KalmanWindowSize)
	logger.Debugf("[AutoBrightness] Kalman filter params updated: Q=%.4f, R=%.4f, window=%d",
		abm.config.KalmanProcessNoise, abm.config.KalmanMeasurementNoise, abm.config.KalmanWindowSize)
}

// IsSupported 检查是否支持自动亮度
func (abm *AutoBrightnessManager) IsSupported() bool {
	abm.mutex.RLock()
	defer abm.mutex.RUnlock()
	return abm.supported
}

// IsEnabled 检查是否启用
func (abm *AutoBrightnessManager) IsEnabled() bool {
	abm.mutex.RLock()
	defer abm.mutex.RUnlock()
	return abm.enabled
}

// GetConfig 获取当前配置
func (abm *AutoBrightnessManager) GetConfig() AutoBrightnessConfig {
	abm.mutex.RLock()
	defer abm.mutex.RUnlock()
	return abm.config
}

// GetStatus 获取当前状态信息
func (abm *AutoBrightnessManager) GetStatus() map[string]interface{} {
	abm.mutex.RLock()
	defer abm.mutex.RUnlock()
	status := map[string]interface{}{
		"supported":         abm.supported,
		"enabled":           abm.enabled,
		"running":           abm.running,
		"last_light_level":  abm.lastLightLevel,
		"last_brightness":   abm.lastBrightness,
		"manual_override":   !abm.manualOverride.IsZero(),
		"service_available": false,
		"sensor_claimed":    false,
	}
	if abm.sensorClient != nil {
		status["service_available"] = abm.sensorClient.IsServiceAvailable()
		status["sensor_claimed"] = abm.sensorClient.IsClaimed()
	}
	if !abm.manualOverride.IsZero() {
		duration := time.Duration(abm.config.ManualOverrideDuration) * time.Second
		remaining := duration - time.Since(abm.manualOverride)
		if remaining > 0 {
			status["manual_override_remaining"] = remaining.Seconds()
		}
	}
	return status
}

// GetManualOverrideRemaining 获取手动调节暂停剩余时间（秒）
func (abm *AutoBrightnessManager) GetManualOverrideRemaining() float64 {
	abm.mutex.RLock()
	defer abm.mutex.RUnlock()
	if abm.manualOverride.IsZero() {
		return 0
	}
	duration := time.Duration(abm.config.ManualOverrideDuration) * time.Second
	elapsed := time.Since(abm.manualOverride)
	remaining := duration - elapsed
	if remaining <= 0 {
		return 0
	}
	return remaining.Seconds()
}

// claimLightWithRetry 带重试机制的传感器声明
func (abm *AutoBrightnessManager) claimLightWithRetry() error {
	var lastErr error
	for i := 0; i < abm.maxRetries; i++ {
		err := abm.sensorClient.ClaimLight()
		if err == nil {
			return nil
		}
		lastErr = err
		if i < abm.maxRetries-1 {
			time.Sleep(abm.retryInterval)
		}
	}
	return lastErr
}

// gracefulDegradeService 优雅降级服务
// 注意：此函数假设调用者已经持有锁
func (abm *AutoBrightnessManager) gracefulDegradeService() {
	abm.stopPolling()
	abm.stopCompensationTimer()

	if abm.sensorClient != nil {
		err := abm.sensorClient.ReleaseLight()
		if err != nil {
			logger.Warning("[AutoBrightness] Failed to release light sensor during degradation:", err)
		}
	}
	abm.running = false
	abm.supported = false
}

// recoverService 尝试恢复服务
func (abm *AutoBrightnessManager) recoverService() error {
	// 重新初始化传感器连接
	if abm.sensorClient != nil {
		err := abm.sensorClient.Connect()
		if err != nil {
			return fmt.Errorf("failed to reconnect sensor during recovery: %w", err)
		}
		hasLight, err := abm.sensorClient.HasAmbientLight()
		if err != nil || !hasLight {
			return fmt.Errorf("ambient light sensor not available during recovery: %w", err)
		}
		abm.supported = true
		if abm.config.Enabled {
			return abm.Start()
		}
	}
	return nil
}

// loadConfig 加载配置
// 注意：此函数假设调用者已经持有锁（如果需要修改 abm.config）
func (abm *AutoBrightnessManager) loadConfig() error {
	config, err := abm.getConfig()
	if err != nil {
		return err
	}
	abm.config = config
	return nil
}

// startPolling 启动轮询定时器
// 注意：此函数假设调用者已经持有锁（用于访问 abm.ticker 和 abm.pollInterval）
func (abm *AutoBrightnessManager) startPolling() {
	logger.Debug("auto brightness polling start")
	// 如果已经在运行，先停止
	if abm.polling {
		abm.stopPolling()
	}
	pollInterval := abm.pollInterval
	// 创建新的 ticker
	abm.ticker = time.NewTicker(pollInterval)
	abm.polling = true
	// 增加 WaitGroup 计数
	abm.pollingWg.Add(1)
	go func() {
		defer abm.pollingWg.Done()
		// 立即执行一次
		abm.pollLightLevel()
		// 轮询循环
		for {
			select {
			case <-abm.ticker.C:
				abm.pollLightLevel()
			case <-abm.stopChan:
				logger.Debug("auto brightness polling goroutine received stop signal")
				return
			}
		}
	}()
}

// stopPolling 停止轮询定时器
// 注意：此函数假设调用者已经持有锁（用于访问 abm.ticker）
// 重要：此函数是幂等的，可以安全地多次调用
func (abm *AutoBrightnessManager) stopPolling() {
	logger.Debug("auto brightness polling stop")
	if !abm.polling {
		return
	}
	if abm.ticker != nil {
		abm.ticker.Stop()
		abm.ticker = nil
	}
	// 发送停止信号给 goroutine（非阻塞）
	select {
	case abm.stopChan <- struct{}{}:
		logger.Debug("auto brightness stop signal sent")
	default:
		// channel 已满，说明已经有停止信号在等待，无需重复发送
		logger.Debug("auto brightness stop signal already pending")
	}
	abm.polling = false
	// 释放锁后等待 goroutine 退出
	// 注意：这里需要临时释放锁，避免死锁
	abm.mutex.Unlock()
	abm.pollingWg.Wait()
	abm.mutex.Lock()
	// 清空 stopChan 中可能残留的信号
	select {
	case <-abm.stopChan:
	default:
	}
}

// pollLightLevel 轮询环境光强度
func (abm *AutoBrightnessManager) pollLightLevel() {
	// 检查是否正在运行
	abm.mutex.Lock()
	if !abm.running {
		abm.mutex.Unlock()
		return
	}
	// 检查是否在手动调节暂停期间
	inManualOverride := abm.isInManualOverride()
	abm.mutex.Unlock()
	if inManualOverride {
		return
	}
	if abm.sensorClient == nil {
		logger.Warning("[AutoBrightness] SensorClient is nil")
		return
	}
	// 如果不在手动调节期，确保传感器已被claim
	if !abm.sensorClient.IsClaimed() {
		err := abm.sensorClient.ClaimLight()
		if err != nil {
			logger.Warning("[AutoBrightness] Failed to re-claim light sensor:", err)
			return
		}
		logger.Info("[AutoBrightness] Re-claimed light sensor after manual override period")
	}
	// 从传感器获取缓存的原始光照强度
	rawLightLevel, err := abm.sensorClient.GetCachedLightLevel()
	if err != nil {
		logger.Debugf("[AutoBrightness] Failed to get cached light level: %v", err)
		return
	}

	// 处理原始值（包括滤波和亮度调整）
	abm.processLightChange(rawLightLevel)
}

// onServiceChange 服务状态变化回调
func (abm *AutoBrightnessManager) onServiceChange(available bool) {
	var shouldStart bool

	abm.mutex.Lock()

	if available {
		logger.Info("[AutoBrightness] SensorProxy service became available")
		if abm.sensorClient != nil {
			hasLight, err := abm.sensorClient.HasAmbientLight()
			if err == nil && hasLight {
				abm.supported = true
				shouldStart = abm.config.Enabled && !abm.running
			}
		}
	} else {
		logger.Warning("[AutoBrightness] SensorProxy service became unavailable")
		if abm.running {
			abm.stopPolling()
			abm.stopCompensationTimer()
			abm.running = false
			abm.enabled = false
		}
		abm.supported = false
	}

	if abm.manager != nil {
		abm.manager.setPropAutoBrightnessSupported(abm.supported)
	}

	abm.mutex.Unlock()

	if shouldStart {
		abm.Start()
	}
}

// onLightLevelChange 光照值变化回调（推送模式）
func (abm *AutoBrightnessManager) onLightLevelChange(rawLightLevel int) {
	abm.mutex.Lock()
	running := abm.running
	polling := abm.polling
	inManualOverride := abm.isInManualOverride()
	abm.lastSensorDataTime = time.Now()
	abm.mutex.Unlock()

	if !running || !polling || inManualOverride {
		return
	}

	abm.processLightChange(rawLightLevel)
}

// adjustBrightnessOnce 立即执行一次亮度调整（用于配置变化时）
func (abm *AutoBrightnessManager) adjustBrightnessOnce() {
	abm.mutex.Lock()
	if !abm.running || !abm.enabled {
		abm.mutex.Unlock()
		return
	}
	// 使用上一次的环境光值重新计算亮度
	lastLight := abm.lastLightLevel
	// 如果没有历史光照数据，先读取一次
	if lastLight == 0 {
		abm.mutex.Unlock()
		abm.pollLightLevel()
		return
	}
	// 临时重置状态，让 shouldAdjustBrightness 的检查通过
	savedLightLevel := abm.lastLightLevel
	savedAdjustTime := abm.lastAdjustTime
	abm.lastLightLevel = -1          // 设为负数，跳过环境光变化检查
	abm.lastAdjustTime = time.Time{} // 设为零值，跳过频率检查
	abm.mutex.Unlock()
	// 使用历史光照值重新计算并应用亮度
	abm.processLightChange(lastLight)
	// 恢复状态（如果调整成功，lastLightLevel 和 lastAdjustTime 会被更新）
	abm.mutex.Lock()
	if abm.lastLightLevel < 0 {
		abm.lastLightLevel = savedLightLevel
	}
	if abm.lastAdjustTime.IsZero() {
		abm.lastAdjustTime = savedAdjustTime
	}
	abm.mutex.Unlock()
}

// processLightChange 处理环境光变化（包括卡尔曼滤波和亮度调整）
func (abm *AutoBrightnessManager) processLightChange(rawLightLevel int) {
	abm.mutex.Lock()

	if !abm.running {
		abm.mutex.Unlock()
		return
	}

	// 应用卡尔曼滤波器处理原始光值
	if abm.kalmanFilter == nil {
		logger.Warning("[AutoBrightness] Kalman filter is nil, skipping light change processing")
		abm.mutex.Unlock()
		return
	}

	estimate := abm.kalmanFilter.Update(float64(rawLightLevel))
	filteredLightLevel := int(estimate)
	logger.Infof("[AutoBrightness] Kalman filter: raw=%d lux -> filtered=%d lux", rawLightLevel, filteredLightLevel)

	// 计算目标亮度（使用滤波后的值）
	targetBrightness := abm.calculateTargetBrightness(filteredLightLevel)

	// 检查是否应该调节亮度（包含所有阈值和频率控制逻辑）
	if !abm.shouldAdjustBrightness(filteredLightLevel, targetBrightness) {
		abm.mutex.Unlock()
		return
	}

	// 释放锁后再设置亮度（避免在渐变时持有锁）
	abm.mutex.Unlock()

	// 设置亮度（不重试，下次轮询会自动重试）
	err := abm.setBrightness(targetBrightness)
	if err != nil {
		logger.Warningf("[AutoBrightness] Failed to set brightness (raw=%d, filtered=%d, target=%.1f%%): %v",
			rawLightLevel, filteredLightLevel, targetBrightness*100, err)
		return
	}

	// 更新状态
	abm.mutex.Lock()
	now := time.Now()
	abm.lastLightLevel = filteredLightLevel
	abm.lastBrightness = targetBrightness
	abm.lastAdjustTime = now
	abm.mutex.Unlock()

	logger.Infof("[AutoBrightness] Brightness adjusted: raw=%d lux -> filtered=%d lux -> brightness=%.1f%%",
		rawLightLevel, filteredLightLevel, targetBrightness*100)
}

// needCompensation checks if compensation is needed and returns the sensor value
// Note: caller must hold abm.mutex
func (abm *AutoBrightnessManager) needCompensation() (bool, int) {
	if abm.lastSensorDataTime.IsZero() {
		return false, 0
	}

	if time.Since(abm.lastSensorDataTime) < sensorDataTimeout {
		return false, 0
	}

	if abm.kalmanFilter == nil {
		return false, 0
	}

	sensorValue, err := abm.sensorClient.GetCachedLightLevel()
	if err != nil {
		return false, 0
	}

	filterOutput := abm.kalmanFilter.GetEstimate()
	diff := math.Abs(filterOutput - float64(sensorValue))

	return diff > compensationThreshold, sensorValue
}

func (abm *AutoBrightnessManager) compensationTick() {
	abm.mutex.Lock()
	if !abm.running || !abm.polling {
		abm.mutex.Unlock()
		return
	}
	inManualOverride := abm.isInManualOverride()
	needComp, sensorValue := abm.needCompensation()
	abm.mutex.Unlock()

	if inManualOverride || !needComp {
		return
	}

	logger.Infof("[AutoBrightness] Compensation: feeding sensor value %d lux to filter", sensorValue)
	abm.processLightChange(sensorValue)
}

func (abm *AutoBrightnessManager) startCompensationTimer() {
	if abm.compensationTicker != nil {
		return
	}
	abm.compensationTicker = time.NewTicker(compensationInterval)
	abm.compensationStopCh = make(chan struct{})
	abm.compensationWg.Add(1)

	go func() {
		defer abm.compensationWg.Done()
		for {
			select {
			case <-abm.compensationTicker.C:
				abm.compensationTick()
			case <-abm.compensationStopCh:
				return
			}
		}
	}()
}

func (abm *AutoBrightnessManager) stopCompensationTimer() {
	if abm.compensationTicker == nil {
		return
	}

	abm.compensationTicker.Stop()
	abm.compensationTicker = nil
	if abm.compensationStopCh != nil {
		close(abm.compensationStopCh)
		abm.compensationStopCh = nil
	}

	abm.mutex.Unlock()
	abm.compensationWg.Wait()
	abm.mutex.Lock()
}

// isInManualOverride 检查是否在手动调节暂停期间
// 注意：此函数需要读取 abm.manualOverride 和 abm.config，调用者应该持有至少读锁
func (abm *AutoBrightnessManager) isInManualOverride() bool {
	if abm.manualOverride.IsZero() {
		return false
	}
	duration := time.Duration(abm.config.ManualOverrideDuration) * time.Second
	return time.Since(abm.manualOverride) < duration
}

// calculateTargetBrightness 计算目标亮度
func (abm *AutoBrightnessManager) calculateTargetBrightness(lightLevel int) float64 {
	// 优先使用曲线配置
	if brightness.HasAutoBrightnessCurve() {
		br := brightness.GetAutoBrightnessValue(lightLevel)
		if br >= 0 {
			// 约束到有效范围
			if br < 0.0 {
				br = 0.0
			} else if br > 1.0 {
				br = 1.0
			}
			// 设置最小亮度，避免屏幕过暗
			minBrightness := 0.1 // 10% 最小亮度
			if br < minBrightness {
				br = minBrightness
			}
			return br
		}
	}

	// 应用敏感度调整
	adjustedLevel := float64(lightLevel) * abm.config.Sensitivity

	// 简单线性映射
	brightness := adjustedLevel / 1024.0

	// 约束到有效范围
	if brightness < 0.0 {
		brightness = 0.0
	} else if brightness > 1.0 {
		brightness = 1.0
	}

	// 设置最小亮度，避免屏幕过暗
	minBrightness := 0.1 // 10% 最小亮度
	if brightness < minBrightness {
		brightness = minBrightness
	}
	return brightness
}

// shouldAdjustBrightness 检查是否应该调节亮度(变化阈值和频率控制)
func (abm *AutoBrightnessManager) shouldAdjustBrightness(lightLevel int, targetBrightness float64) bool {
	now := time.Now()

	// 检查环境光变化阈值
	if abm.lastLightLevel >= 0 {
		lightChange := math.Abs(float64(lightLevel - abm.lastLightLevel))
		if lightChange < abm.config.ChangeThreshold {
			return false
		}
	}

	// 检查调节频率限制
	if !abm.lastAdjustTime.IsZero() {
		timeSinceLastAdjust := now.Sub(abm.lastAdjustTime)
		minInterval := time.Duration(abm.config.PollingInterval) * time.Second
		if timeSinceLastAdjust < minInterval {
			return false
		}
	}

	// 检查亮度变化是否足够大
	if abm.lastBrightness >= 0 {
		brightnessChange := math.Abs(targetBrightness - abm.lastBrightness)
		if brightnessChange < abm.config.BrightnessChangeThreshold {
			return false
		}
	}
	return true
}

// setBrightness 设置亮度（自动调节专用，不触发手动调节检测）
func (abm *AutoBrightnessManager) setBrightness(value float64) error {
	// 使用内置显示器
	builtinMonitor := abm.manager.getBuiltinMonitor()
	if builtinMonitor == nil {
		return errors.New("no builtin monitor")
	}
	// 根据配置决定是否使用渐变
	abm.mutex.RLock()
	useTransition := abm.config.UseTransition
	abm.mutex.RUnlock()
	var err error
	if useTransition {
		// 使用渐变效果（即使全局渐变功能关闭）
		if abm.manager.brightnessTransition != nil {
			// 使用 SetBrightness 强制启用渐变，忽略全局 enabled 标志
			err = abm.manager.brightnessTransition.SetBrightness(builtinMonitor.Name, value, true)
			if err == nil {
				// 渐变会在完成时自动同步属性，这里不需要再次同步
				return nil
			}
			// 如果渐变失败，回退到直接设置
			logger.Warning("[AutoBrightness] Transition failed, fallback to direct set:", err)
			return err
		}
	}
	// 不强制使用渐变
	err = abm.manager.setBrightness(builtinMonitor.Name, value)
	if err != nil {
		logger.Warning("[AutoBrightness] Failed to set brightness:", err)
	}
	// 不论是否设置失败，均应当向外同步亮度
	abm.manager.syncPropBrightness()
	return err
}

// restoreSavedBrightness 恢复配置中保存的亮度
// 注意：此函数假设调用者已经持有锁
func (abm *AutoBrightnessManager) restoreSavedBrightness() {
	// 使用内置显示器
	builtinMonitor := abm.manager.getBuiltinMonitor()
	if builtinMonitor == nil {
		return
	}
	// 从配置中获取保存的亮度
	savedBrightness := abm.manager.getMonitorBrightness(builtinMonitor.Name)
	if savedBrightness < 0 {
		savedBrightness = 1.0 // 默认亮度
	}
	// 恢复亮度（自动处理渐变和属性同步）
	err := abm.manager.setBrightnessAndSync(builtinMonitor.Name, savedBrightness)
	if err != nil {
		logger.Warning("[AutoBrightness] Failed to restore brightness:", err)
		return
	}
	logger.Infof("[AutoBrightness] Restored saved brightness: %.1f%%", savedBrightness*100)
}

// checkSensorAvailability 检查传感器是否可用（不连接）
func (abm *AutoBrightnessManager) checkSensorAvailability() error {
	// 临时连接以检查传感器
	err := abm.sensorClient.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to sensor proxy: %w", err)
	}
	// 检查是否有环境光传感器
	hasLight, err := abm.sensorClient.HasAmbientLight()
	if err != nil {
		abm.sensorClient.Disconnect()
		return fmt.Errorf("failed to check ambient light sensor: %w", err)
	}
	if !hasLight {
		abm.sensorClient.Disconnect()
		return errors.New("no ambient light sensor available")
	}
	// 检查完成后立即断开连接
	// 实际使用时会在Start()中重新连接
	abm.sensorClient.Disconnect()
	return nil
}

// 配置管理方法
// Validate 验证配置参数的有效性
func (config *AutoBrightnessConfig) Validate() error {
	if config.Sensitivity < 0.1 {
		return errors.New("sensitivity too small")
	}
	if config.PollingInterval < 1 {
		return errors.New("polling interval too small")
	}
	if config.ChangeThreshold < 1.0 {
		return errors.New("change threshold too small")
	}
	if config.ManualOverrideDuration < 1 {
		return errors.New("manual override duration too small")
	}
	if config.KalmanProcessNoise <= 0 || config.KalmanProcessNoise > 100 {
		return errors.New("kalman process noise must be positive and less than 100")
	}
	if config.KalmanMeasurementNoise <= 0 || config.KalmanMeasurementNoise > 100 {
		return errors.New("kalman measurement noise must be positive and less than 100")
	}
	if config.KalmanWindowSize < 2 || config.KalmanWindowSize > 100 {
		return errors.New("kalman window size must be between 2 and 100")
	}
	return nil
}

// initConfigManager 初始化配置管理器
func (abm *AutoBrightnessManager) initConfigManager() error {
	ds := configManager.NewConfigManager(abm.sysBus)
	configPath, err := ds.AcquireManager(0, DSettingsAutoBrightnessAppID, DSettingsAutoBrightnessName, "")
	if err != nil || configPath == "" {
		return fmt.Errorf("failed to acquire config manager: %w", err)
	}
	abm.configManager, err = configManager.NewManager(abm.sysBus, configPath)
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}
	// 监听配置变更
	abm.configManager.InitSignalExt(abm.manager.sysSigLoop, true)
	_, err = abm.configManager.ConnectValueChanged(func(key string) {
		abm.onConfigFileChanged()
	})
	if err != nil {
		return fmt.Errorf("failed to connect value changed: %w", err)
	}
	logger.Info("[AutoBrightness] Config manager initialized successfully")
	return nil
}

// onConfigFileChanged 配置文件变更回调
func (abm *AutoBrightnessManager) onConfigFileChanged() {
	config, err := abm.getConfig()
	if err != nil {
		logger.Warning("[AutoBrightness] Failed to reload config:", err)
		return
	}
	logger.Info("[AutoBrightness] Config file changed, reloading configuration")
	// 更新 Manager 的属性
	abm.manager.setPropAutoBrightnessEnabled(config.Enabled)
	// 通知配置变更
	abm.OnConfigChanged(config)
}

// getConfig 获取自动亮度配置
func (abm *AutoBrightnessManager) getConfig() (AutoBrightnessConfig, error) {
	if abm.configManager == nil {
		logger.Warning("[AutoBrightness] Config manager is nil, using default config")
		return DefaultAutoBrightnessConfig, nil
	}
	var config AutoBrightnessConfig
	// Enabled
	if val, err := abm.configManager.Value(0, DSettingsKeyABEnabled); err == nil {
		if b, ok := val.Value().(bool); ok {
			config.Enabled = b
		} else {
			config.Enabled = DefaultAutoBrightnessConfig.Enabled
		}
	} else {
		config.Enabled = DefaultAutoBrightnessConfig.Enabled
	}
	// Sensitivity
	if val, err := abm.configManager.Value(0, DSettingsKeyABSensitivity); err == nil {
		switch v := val.Value().(type) {
		case float64:
			config.Sensitivity = v
		case int64:
			config.Sensitivity = float64(v)
		default:
			logger.Warningf("[AutoBrightness] Invalid type for sensitivity: %T", v)
			config.Sensitivity = DefaultAutoBrightnessConfig.Sensitivity
		}
	} else {
		logger.Warning("[AutoBrightness] Config convert faild, using default sensitivity")
		config.Sensitivity = DefaultAutoBrightnessConfig.Sensitivity
	}
	// ChangeThreshold
	if val, err := abm.configManager.Value(0, DSettingsKeyABChangeThreshold); err == nil {
		switch v := val.Value().(type) {
		case float64:
			config.ChangeThreshold = v
		case int64:
			config.ChangeThreshold = float64(v)
		default:
			logger.Warningf("[AutoBrightness] Invalid type for changeThreshold: %T", v)
			config.ChangeThreshold = DefaultAutoBrightnessConfig.ChangeThreshold
		}
	} else {
		logger.Warning("[AutoBrightness] Config convert faild, using default changeThreshold")
		config.ChangeThreshold = DefaultAutoBrightnessConfig.ChangeThreshold
	}

	// BrightnessChangeThreshold
	if val, err := abm.configManager.Value(0, DSettingsKeyABBrightnessChangeThreshold); err == nil {
		config.BrightnessChangeThreshold = val.Value().(float64)
	} else {
		logger.Warning("[AutoBrightness] Config convert faild, using default brightnessChangeThreshold")
		config.BrightnessChangeThreshold = DefaultAutoBrightnessConfig.BrightnessChangeThreshold
	}

	// PollingInterval
	if val, err := abm.configManager.Value(0, DSettingsKeyABPollingInterval); err == nil {
		switch v := val.Value().(type) {
		case int64:
			config.PollingInterval = int(v)
		case float64:
			config.PollingInterval = int(v)
		default:
			config.PollingInterval = DefaultAutoBrightnessConfig.PollingInterval
		}
	} else {
		logger.Warning("[AutoBrightness] Config convert faild, using default pollingInterval")
		config.PollingInterval = DefaultAutoBrightnessConfig.PollingInterval
	}
	// ManualOverrideDuration
	if val, err := abm.configManager.Value(0, DSettingsKeyABManualOverride); err == nil {
		switch v := val.Value().(type) {
		case int64:
			config.ManualOverrideDuration = int(v)
		case float64:
			config.ManualOverrideDuration = int(v)
		default:
			config.ManualOverrideDuration = DefaultAutoBrightnessConfig.ManualOverrideDuration
		}
	} else {
		logger.Warning("[AutoBrightness] Config convert faild, using default manualOverrideDuration")
		config.ManualOverrideDuration = DefaultAutoBrightnessConfig.ManualOverrideDuration
	}
	// ManualAdjustDisablesAutoMode
	if val, err := abm.configManager.Value(0, DSettingsKeyABManualAdjustDisablesAutoMode); err == nil {
		if b, ok := val.Value().(bool); ok {
			config.ManualAdjustDisablesAutoMode = b
		} else {
			config.ManualAdjustDisablesAutoMode = DefaultAutoBrightnessConfig.ManualAdjustDisablesAutoMode
		}
	} else {
		logger.Warning("[AutoBrightness] Config convert faild, using default manualAdjustDisablesAutoMode")
		config.ManualAdjustDisablesAutoMode = DefaultAutoBrightnessConfig.ManualAdjustDisablesAutoMode
	}
	// UseTransition
	if val, err := abm.configManager.Value(0, DSettingsKeyABUseTransition); err == nil {
		if b, ok := val.Value().(bool); ok {
			config.UseTransition = b
		} else {
			config.UseTransition = DefaultAutoBrightnessConfig.UseTransition
		}
	} else {
		logger.Warning("[AutoBrightness] Config convert faild, using default useTransition")
		config.UseTransition = DefaultAutoBrightnessConfig.UseTransition
	}

	// Curve
	if val, err := abm.configManager.Value(0, DSettingsKeyABCurve); err == nil {
		itemList, ok := val.Value().([]dbus.Variant)
		if ok && len(itemList) > 0 {
			var points []brightness.AutoBrightnessCurvePoint
			for _, item := range itemList {
				pointMap, ok := item.Value().(map[string]dbus.Variant)
				if !ok {
					continue
				}
				var point brightness.AutoBrightnessCurvePoint
				if luxVal, ok := pointMap["lux"]; ok {
					switch v := luxVal.Value().(type) {
					case int64:
						point.Lux = int(v)
					case float64:
						point.Lux = int(v)
					}
				}
				if brVal, ok := pointMap["br"]; ok {
					switch v := brVal.Value().(type) {
					case int64:
						point.Br = float64(v)
					case float64:
						point.Br = v
					}
				}
				points = append(points, point)
			}
			brightness.SetAutoBrightnessCurveFromPoints(points)
		} else {
			logger.Debug("[AutoBrightness] Curve config is empty, using default linear mapping")
		}
	} else {
		logger.Debug("[AutoBrightness] No curve config found, using default linear mapping")
	}

	// KalmanProcessNoise
	if val, err := abm.configManager.Value(0, DSettingsKeyABKalmanProcessNoise); err == nil {
		config.KalmanProcessNoise = val.Value().(float64)
	} else {
		logger.Warning("[AutoBrightness] Config convert failed, using default kalmanProcessNoise")
		config.KalmanProcessNoise = DefaultAutoBrightnessConfig.KalmanProcessNoise
	}

	// KalmanMeasurementNoise
	if val, err := abm.configManager.Value(0, DSettingsKeyABKalmanMeasurementNoise); err == nil {
		config.KalmanMeasurementNoise = val.Value().(float64)
	} else {
		logger.Warning("[AutoBrightness] Config convert failed, using default kalmanMeasurementNoise")
		config.KalmanMeasurementNoise = DefaultAutoBrightnessConfig.KalmanMeasurementNoise
	}

	// KalmanWindowSize
	if val, err := abm.configManager.Value(0, DSettingsKeyABKalmanWindowSize); err == nil {
		switch v := val.Value().(type) {
		case int64:
			config.KalmanWindowSize = int(v)
		case float64:
			config.KalmanWindowSize = int(v)
		default:
			config.KalmanWindowSize = DefaultAutoBrightnessConfig.KalmanWindowSize
		}
	} else {
		logger.Warning("[AutoBrightness] Config convert failed, using default kalmanWindowSize")
		config.KalmanWindowSize = DefaultAutoBrightnessConfig.KalmanWindowSize
	}

	// 验证配置有效性
	logger.Debugf("[AutoBrightness] Apply config: %v", config)
	err := config.Validate()
	if err != nil {
		logger.Warning("[AutoBrightness] Invalid config, using default:", err)
		return DefaultAutoBrightnessConfig, nil
	}
	return config, nil
}

// saveConfig 保存自动亮度配置
func (abm *AutoBrightnessManager) saveConfig() error {
	if abm.configManager == nil {
		return errors.New("config manager is nil")
	}
	// 验证配置有效性
	err := abm.config.Validate()
	if err != nil {
		return err
	}
	// 保存各个配置项
	err = setGlobalDconfValue(DSettingsAutoBrightnessAppID, DSettingsAutoBrightnessName, "",
		DSettingsKeyABEnabled, dbus.MakeVariant(abm.config.Enabled))
	if err != nil {
		return err
	}
	err = setGlobalDconfValue(DSettingsAutoBrightnessAppID, DSettingsAutoBrightnessName, "",
		DSettingsKeyABSensitivity, dbus.MakeVariant(abm.config.Sensitivity))
	if err != nil {
		return err
	}
	err = setGlobalDconfValue(DSettingsAutoBrightnessAppID, DSettingsAutoBrightnessName, "",
		DSettingsKeyABChangeThreshold, dbus.MakeVariant(abm.config.ChangeThreshold))
	if err != nil {
		return err
	}
	err = setGlobalDconfValue(DSettingsAutoBrightnessAppID, DSettingsAutoBrightnessName, "",
		DSettingsKeyABPollingInterval, dbus.MakeVariant(abm.config.PollingInterval))
	if err != nil {
		return err
	}
	err = setGlobalDconfValue(DSettingsAutoBrightnessAppID, DSettingsAutoBrightnessName, "",
		DSettingsKeyABManualOverride, dbus.MakeVariant(abm.config.ManualOverrideDuration))
	if err != nil {
		return err
	}
	err = setGlobalDconfValue(DSettingsAutoBrightnessAppID, DSettingsAutoBrightnessName, "",
		DSettingsKeyABManualAdjustDisablesAutoMode, dbus.MakeVariant(abm.config.ManualAdjustDisablesAutoMode))
	if err != nil {
		return err
	}
	err = setGlobalDconfValue(DSettingsAutoBrightnessAppID, DSettingsAutoBrightnessName, "",
		DSettingsKeyABUseTransition, dbus.MakeVariant(abm.config.UseTransition))
	if err != nil {
		return err
	}
	return nil
}

// https://gerrit.uniontech.com/plugins/gitiles/startdde/+/2295aa3ceaae739b0afb12c38b8c879f449631e7
// https://gerrit.uniontech.com/plugins/gitiles/startdde/+/5ee59c1117d2d8dffc3e4d3e70009693e73ed576
// https://gerrit.uniontech.com/plugins/gitiles/startdde/+/c99dee92d7bbec19a9140b77d1ffed8c9fa06106
// https://gerrit.uniontech.com/plugins/gitiles/startdde/+/b2772f4cb6c7418a2f4e8307fcf4ae5ee6f81e2f
// https://gerrit.uniontech.com/plugins/gitiles/startdde/+/020e8d3486d195b6f40a543141c216e164176b01
// https://gerrit.uniontech.com/plugins/gitiles/startdde/+/7a546becfbf065a22534d44264760d2e18e1e248
