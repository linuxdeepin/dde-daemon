// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package display1

import (
	"math"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

// transitionState 单个显示器的渐变状态
type transitionState struct {
	mu           sync.Mutex     // 保护 running 和 currentValue
	running      bool           // 是否正在执行渐变
	currentValue float64        // 当前渐变的实时亮度值
	stopCh       chan struct{}  // 停止信号
	wg           sync.WaitGroup // 等待渐变完成
	setter       func(float64) error
}

const (
	defaultDuration     = 4
	defaultStepInterval = 100
	durationMin         = 1
	stepIntervalMin     = 20
)

// BrightnessTransition 亮度渐变管理器
type BrightnessTransition struct {
	manager *Manager
	// 渐变配置
	enabled      bool
	duration     int // 从0%到100%的渐变时长（秒）
	stepInterval int // 步进间隔（毫秒）
	// 每个显示器的渐变状态
	states map[string]*transitionState
	// 配置管理器
	cfgManager configManager.Manager
	mu         sync.Mutex // 保护配置和状态
}

// NewBrightnessTransition 创建亮度渐变管理器
func NewBrightnessTransition(manager *Manager) *BrightnessTransition {
	return &BrightnessTransition{
		manager:      manager,
		enabled:      false,
		duration:     defaultDuration,
		stepInterval: defaultStepInterval,
		states:       make(map[string]*transitionState),
	}
}
func (bt *BrightnessTransition) SetBrightnessSetter(monitorName string, setter func(float64) error) {
	// 初始化配置管理器
	state := bt.getState(monitorName)
	state.setter = setter
}

// getState 获取或创建显示器的渐变状态
func (bt *BrightnessTransition) getState(monitorName string) *transitionState {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	state, exists := bt.states[monitorName]
	if !exists {
		state = &transitionState{
			stopCh: make(chan struct{}, 1),
		}
		bt.states[monitorName] = state
	}
	return state
}

// SetEnabled 设置是否启用渐变
func (bt *BrightnessTransition) SetEnabled(enabled bool) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.enabled = enabled
}

// IsEnabled 获取是否启用渐变
func (bt *BrightnessTransition) IsEnabled() bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	return bt.enabled
}

// SetDuration 设置渐变时长（秒）
func (bt *BrightnessTransition) SetDuration(duration int) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	if duration >= durationMin {
		bt.duration = duration
	} else {
		logger.Warningf("[BrightnessTransition] duration must be >= %d, got %d", durationMin, duration)
	}
}

// SetStepInterval 设置步进间隔（毫秒）
func (bt *BrightnessTransition) SetStepInterval(interval int) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	if interval >= stepIntervalMin {
		bt.stepInterval = interval
	} else {
		logger.Warningf("[BrightnessTransition] stepInterval must be >= %d, got %d", stepIntervalMin, interval)
	}
}

// IsRunning 检查是否正在执行渐变
func (bt *BrightnessTransition) IsRunning() bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	for _, state := range bt.states {
		if state.running {
			return true
		}
	}
	return false
}

// Stop 停止所有渐变（公开方法）
func (bt *BrightnessTransition) Stop() {
	bt.mu.Lock()
	states := make([]*transitionState, 0, len(bt.states))
	for _, state := range bt.states {
		states = append(states, state)
	}
	bt.mu.Unlock()
	for _, state := range states {
		bt.stopState(state)
	}
}

// stopState 停止指定显示器的渐变
func (bt *BrightnessTransition) stopState(state *transitionState) {
	if state == nil {
		return
	}
	state.mu.Lock()
	isRunning := state.running
	state.mu.Unlock()
	if !isRunning {
		return
	}
	// 发送停止信号（非阻塞）
	select {
	case state.stopCh <- struct{}{}:
	default:
	}
	// 等待渐变完成
	state.wg.Wait()
	// 清空可能残留的停止信号
	select {
	case <-state.stopCh:
	default:
	}
}

// SetBrightness 使用渐变效果设置亮度
func (bt *BrightnessTransition) SetBrightness(monitorName string, targetValue float64, forceTransition bool) error {
	return bt.setBrightnessInternal(monitorName, targetValue, forceTransition)
}

// setBrightnessInternal 内部方法，实现渐变逻辑
// forceTransition: 是否强制使用渐变（忽略 enabled 标志）
func (bt *BrightnessTransition) setBrightnessInternal(monitorName string, targetValue float64, forceTransition bool) error {
	bt.mu.Lock()
	enabled := bt.enabled
	duration := bt.duration
	stepInterval := bt.stepInterval
	bt.mu.Unlock()
	state := bt.getState(monitorName)
	brightnessSetter := state.setter
	if brightnessSetter == nil {
		logger.Warningf("No setter for monitro %v brightness transition", monitorName)
		return nil
	}
	// 如果未启用渐变且不是强制渐变，直接设置
	if !enabled && !forceTransition {
		return brightnessSetter(targetValue)
	}
	// 获取当前亮度：如果正在渐变，使用实时值
	var currentBrightness float64
	var hasCurrentValue bool
	state.mu.Lock()
	if state.running {
		currentBrightness = state.currentValue
		hasCurrentValue = true
	}
	state.mu.Unlock()
	// 如果没有实时值，从 Manager 获取
	if !hasCurrentValue {
		currentBrightness = bt.manager.getMonitorBrightness(monitorName)
		if currentBrightness < 0 {
			currentBrightness = 0.5 // 如果无法获取，使用默认值
		}
	}
	// 计算亮度差值
	delta := targetValue - currentBrightness
	if math.Abs(delta) < 0.001 {
		return nil // 无需调整（差值太小）
	}
	// 停止该显示器之前的渐变（如果有）
	bt.stopState(state)
	// 计算渐变参数
	// duration 是 0-100% 的时间，实际时间按比例计算
	actualDuration := time.Duration(float64(duration)*math.Abs(delta)*1000) * time.Millisecond
	// 计算步进参数
	stepIntervalDuration := time.Duration(stepInterval) * time.Millisecond
	// 如果渐变时间太短（小于 2 个步进周期），直接设置
	// 这样可以避免过短的渐变（例如：默认配置下小于 200ms 的渐变）
	minDuration := stepIntervalDuration * 2
	if actualDuration < minDuration {
		// 变化太小，直接设置
		return brightnessSetter(targetValue)
	}
	totalSteps := int(actualDuration / stepIntervalDuration)
	if totalSteps < 2 {
		totalSteps = 2
	}
	stepSize := delta / float64(totalSteps)
	// 标记渐变开始
	state.mu.Lock()
	state.running = true
	state.currentValue = currentBrightness
	state.mu.Unlock()
	state.wg.Add(1)
	// 启动渐变 goroutine
	go func() {
		defer state.wg.Done()
		defer func() {
			state.mu.Lock()
			state.running = false
			state.mu.Unlock()
		}()
		currentValue := currentBrightness
		ticker := time.NewTicker(stepIntervalDuration)
		defer ticker.Stop()
		for i := 0; i < totalSteps; i++ {
			// 先检查停止信号（优先级更高）
			select {
			case <-state.stopCh:
				// 更新最终的实时值
				state.mu.Lock()
				state.currentValue = currentValue
				state.mu.Unlock()
				logger.Debugf("[BrightnessTransition] %s transition stopped at %.1f%%", monitorName, currentValue*100)
				// 即使被停止，也同步最后的亮度值
				return
			default:
			}
			// 计算下一个亮度值
			currentValue += stepSize
			// 确保不超出范围
			if (stepSize > 0 && currentValue > targetValue) || (stepSize < 0 && currentValue < targetValue) {
				currentValue = targetValue
			}
			// 更新实时亮度值
			state.mu.Lock()
			state.currentValue = currentValue
			state.mu.Unlock()
			// 设置亮度（只更新底层硬件，不同步属性）
			err := brightnessSetter(currentValue)
			if err != nil {
				logger.Warningf("[BrightnessTransition] Failed to set brightness during transition: %v", err)
				// 失败时也要同步一次，确保属性与实际状态一致
				return
			}
			// 如果已经到达目标值，同步属性并结束
			if currentValue == targetValue {
				logger.Debugf("[BrightnessTransition] %s transition completed: %.1f%% -> %.1f%%",
					monitorName, currentBrightness*100, targetValue*100)
				return
			}
			// 如果不是最后一步，等待下一个步进时间或停止信号
			if i < totalSteps-1 {
				select {
				case <-state.stopCh:
					// 更新最终的实时值（已经在上面更新过了）
					state.mu.Lock()
					state.currentValue = currentValue
					state.mu.Unlock()
					logger.Debugf("[BrightnessTransition] %s transition stopped at %.1f%%", monitorName, currentValue*100)
					// 即使被停止，也同步最后的亮度值
					return
				case <-ticker.C:
					// 继续下一次循环
				}
			}
		}
		// 确保最终值精确
		if currentValue != targetValue {
			err := brightnessSetter(targetValue)
			if err != nil {
				logger.Warningf("[BrightnessTransition] Failed to set final brightness: %v", err)
			}
			state.mu.Lock()
			state.currentValue = targetValue
			state.mu.Unlock()
		}
		logger.Debugf("[BrightnessTransition] %s transition completed: %.1f%% -> %.1f%%",
			monitorName, currentBrightness*100, targetValue*100)
	}()
	return nil
}

// LoadConfig 从配置文件加载渐变配置
func (bt *BrightnessTransition) LoadConfig(sysBus *dbus.Conn) error {
	logger.Debug("[BrightnessTransition] Loading config")
	// 获取配置管理器 - 使用 Display 配置文件，而不是 AutoBrightness 配置文件
	ds := configManager.NewConfigManager(sysBus)
	configPath, err := ds.AcquireManager(0, DSettingsAppID, DSettingsDisplayName, "")
	if err != nil || configPath == "" {
		logger.Warning("[BrightnessTransition] Failed to acquire config manager:", err)
		return err
	}
	cfgManager, err := configManager.NewManager(sysBus, configPath)
	if err != nil {
		logger.Warning("[BrightnessTransition] Failed to create config manager:", err)
		return err
	}
	bt.mu.Lock()
	bt.cfgManager = cfgManager
	bt.mu.Unlock()
	// 读取配置
	enabled := true
	duration := 4
	stepInterval := 100
	if val, err := cfgManager.Value(0, DSettingsKeyABTransitionEnabled); err == nil {
		enabled = val.Value().(bool)
	} else {
		logger.Warning("[BrightnessTransition] Failed to read transition-enabled:", err)
	}
	if val, err := cfgManager.Value(0, DSettingsKeyABTransitionDuration); err == nil {
		switch v := val.Value().(type) {
		case int64:
			duration = int(v)
		case float64:
			duration = int(v)
		}
	} else {
		logger.Warning("[BrightnessTransition] Failed to read transition-duration:", err)
	}
	if val, err := cfgManager.Value(0, DSettingsKeyABTransitionStepInterval); err == nil {
		switch v := val.Value().(type) {
		case int64:
			stepInterval = int(v)
		case float64:
			stepInterval = int(v)
		}
	} else {
		logger.Warning("[BrightnessTransition] Failed to read transition-step-interval:", err)
	}
	// 应用配置
	bt.SetEnabled(enabled)
	bt.SetDuration(duration)
	bt.SetStepInterval(stepInterval)
	logger.Infof("[BrightnessTransition] Config loaded: enabled=%v, duration=%ds, stepInterval=%dms",
		enabled, duration, stepInterval)
	return nil
}

// WatchConfigChanges 监听配置变化
func (bt *BrightnessTransition) WatchConfigChanges(sysSigLoop *dbusutil.SignalLoop) error {
	bt.mu.Lock()
	cfgManager := bt.cfgManager
	bt.mu.Unlock()
	if cfgManager == nil {
		return nil
	}
	cfgManager.InitSignalExt(sysSigLoop, true)
	_, err := cfgManager.ConnectValueChanged(func(key string) {
		switch key {
		case DSettingsKeyABTransitionEnabled:
			if val, err := cfgManager.Value(0, key); err == nil {
				enabled := val.Value().(bool)
				bt.SetEnabled(enabled)
				logger.Info("[BrightnessTransition] Enabled changed:", enabled)
			} else {
				logger.Warning("[BrightnessTransition] Failed to read transition-enabled on change:", err)
			}
		case DSettingsKeyABTransitionDuration:
			if val, err := cfgManager.Value(0, key); err == nil {
				var duration int
				switch v := val.Value().(type) {
				case int64:
					duration = int(v)
				case float64:
					duration = int(v)
				}
				bt.SetDuration(duration)
				logger.Info("[BrightnessTransition] Duration changed:", duration)
			} else {
				logger.Warning("[BrightnessTransition] Failed to read transition-duration on change:", err)
			}
		case DSettingsKeyABTransitionStepInterval:
			if val, err := cfgManager.Value(0, key); err == nil {
				var interval int
				switch v := val.Value().(type) {
				case int64:
					interval = int(v)
				case float64:
					interval = int(v)
				}
				bt.SetStepInterval(interval)
				logger.Info("[BrightnessTransition] Step interval changed:", interval)
			} else {
				logger.Warning("[BrightnessTransition] Failed to read transition-step-interval on change:", err)
			}
		}
	})
	if err != nil {
		logger.Warning("[BrightnessTransition] Failed to watch config changes:", err)
		return err
	}
	logger.Debug("[BrightnessTransition] Config change watcher started")
	return nil
}
