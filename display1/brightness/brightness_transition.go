// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package brightness

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BrightnessType 亮度类型
type BrightnessType int

const (
	// TypeUnknown 未知类型
	TypeUnknown BrightnessType = iota
	// TypeBacklight 背光调节
	TypeBacklight
	// TypeGamma Gamma 调节
	TypeGamma
)

const (
	// DefaultMinStepInterval 默认最小步进间隔
	DefaultMinStepInterval = 100 * time.Millisecond

	// epsilon 用于浮点数比较的极小值
	epsilon = 1e-6
)

// TransitionConfig 统一过渡配置
type TransitionConfig struct {
	// 是否启用过渡
	Enabled bool

	// 总调节时长（毫秒）
	Duration time.Duration

	// 步进百分比（如 0.1 表示 0.1%）
	StepPercent float64

	// 最小步进间隔（默认100ms）
	MinStepInterval time.Duration
}

// DefaultTransitionConfig 默认过渡配置
func DefaultTransitionConfig() TransitionConfig {
	return TransitionConfig{
		Enabled:         true,
		Duration:        4000 * time.Millisecond,
		StepPercent:     1,                      // 1% 步进
		MinStepInterval: DefaultMinStepInterval, // 默认 100ms
	}
}

// Validate 验证配置有效性
func (c *TransitionConfig) Validate() error {
	if c.Duration <= 0 {
		c.Duration = 4000 * time.Millisecond
	}

	if c.StepPercent <= 0 {
		c.StepPercent = 1
	}

	// 确保最小步进间隔不小于默认值
	if c.MinStepInterval < 0 {
		c.MinStepInterval = DefaultMinStepInterval
	}

	return nil
}

// transitionTask 过渡任务
type transitionTask struct {
	ctx        context.Context
	cancel     context.CancelFunc
	target     float64
	startValue float64
	config     TransitionConfig
}

// TransitionExecutor 统一过渡执行器
type TransitionExecutor struct {
	mu sync.Mutex

	// 显示器名称
	monitorName string

	// 亮度类型
	brightnessType BrightnessType

	// 当前正在进行的调节任务
	currentTask *transitionTask

	// 当前实时亮度百分比
	currentPercent    float64
	hasCurrentPercent bool

	// 亮度设置函数（百分比）
	setterFunc func(percent float64) error

	// 获取当前亮度百分比函数
	getterFunc func() (float64, error)

	// 每步回调（用于同步亮度属性到 D-Bus），参数为(显示器名称, 当前亮度百分比)
	onStepFunc func(monitorName string, percent float64)

	// 配置参数
	config TransitionConfig

	// goroutine 追踪
	wg sync.WaitGroup
}

// NewTransitionExecutor 创建新的过渡执行器
func NewTransitionExecutor(monitorName string, brightnessType BrightnessType, setter func(float64) error, getter func() (float64, error), config TransitionConfig) *TransitionExecutor {
	config.Validate()

	return &TransitionExecutor{
		monitorName:    monitorName,
		brightnessType: brightnessType,
		setterFunc:     setter,
		getterFunc:     getter,
		config:         config,
	}
}

// SetOnStepFunc 设置每步回调（用于在过渡过程中同步亮度属性到 D-Bus），参数为(显示器名称, 当前亮度百分比)
func (e *TransitionExecutor) SetOnStepFunc(fn func(monitorName string, percent float64)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onStepFunc = fn
}

// SetBrightness 设置亮度（百分比，0.0 - 1.0）
func (e *TransitionExecutor) SetBrightness(targetPercent float64) error {
	return e.SetBrightnessWithForce(targetPercent, false)
}

// SetBrightnessWithForce 设置亮度，支持强制过渡
func (e *TransitionExecutor) SetBrightnessWithForce(targetPercent float64, forceTransition bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 如果过渡未启用且不是强制过渡，直接设置
	if !e.config.Enabled && !forceTransition {
		return e.setterFunc(targetPercent)
	}

	// 获取当前亮度百分比
	var currentPercent float64
	var err error

	// 如果正在过渡，使用实时值
	if e.hasCurrentPercent && e.currentTask != nil {
		currentPercent = e.currentPercent
	} else {
		currentPercent, err = e.getterFunc()
		if err != nil {
			logger.Warningf("[%s] Failed to get current brightness: %v", e.monitorName, err)
			return e.setterFunc(targetPercent)
		}
	}

	// 如果目标值等于当前值，不需要调节
	if abs(currentPercent-targetPercent) < epsilon {
		return nil
	}

	// 取消当前正在进行的任务
	if e.currentTask != nil {
		e.cancelTask(e.currentTask)
		e.currentTask = nil
	}

	// 创建新的调节任务
	ctx, cancel := e.createContext()
	task := &transitionTask{
		ctx:        ctx,
		cancel:     cancel,
		target:     targetPercent,
		startValue: currentPercent,
		config:     e.config,
	}

	e.currentTask = task
	e.currentPercent = currentPercent
	e.hasCurrentPercent = true

	// 启动过渡协程
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.runTransition(task)
	}()

	return nil
}

// createContext 创建上下文
func (e *TransitionExecutor) createContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

// cancelTask 取消任务
func (e *TransitionExecutor) cancelTask(task *transitionTask) {
	if task != nil && task.cancel != nil {
		task.cancel()
	}
}

// isTaskCancelled 检查任务是否被取消
func (e *TransitionExecutor) isTaskCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// abs 计算绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// runTransition 执行过渡
func (e *TransitionExecutor) runTransition(task *transitionTask) {
	defer func() {
		e.mu.Lock()
		if e.currentTask == task {
			e.currentTask = nil
		}
		e.mu.Unlock()
	}()

	// 如果差值很小，直接设置目标值
	if abs(task.target-task.startValue) < epsilon {
		err := e.setterFunc(task.target)
		if err != nil {
			logger.Warningf("[%s] Failed to set brightness %.2f%%: %v", e.monitorName, task.target*100, err)
		}
		e.mu.Lock()
		e.currentPercent = task.target
		e.hasCurrentPercent = false
		e.mu.Unlock()
		return
	}

	// 计算过渡参数
	totalSteps, stepSize, stepInterval := e.calculateTransitionParams(task)

	if totalSteps == 0 {
		return
	}

	logger.Debugf("[%s] Start %s transition: %.2f%% -> %.2f%%, steps: %d, step size: %.4f%%, interval: %v",
		e.monitorName, e.typeName(), task.startValue*100, task.target*100, totalSteps, stepSize*100, stepInterval)

	// 执行等间隔过渡
	currentPercent := task.startValue
	ticker := time.NewTicker(stepInterval)
	defer ticker.Stop()

	for step := 1; step <= totalSteps; step++ {
		// 检查是否被取消
		if e.isTaskCancelled(task.ctx) {
			logger.Debugf("[%s] Transition cancelled at %.2f%%", e.monitorName, currentPercent*100)
			return
		}

		// 计算当前步骤的目标百分比
		currentPercent += stepSize

		// 确保不超过最终目标值
		if (stepSize > 0 && currentPercent > task.target) || (stepSize < 0 && currentPercent < task.target) {
			currentPercent = task.target
		}

		// 设置亮度
		err := e.setterFunc(currentPercent)
		if err != nil {
			logger.Warningf("[%s] Failed to set brightness %.2f%%: %v", e.monitorName, currentPercent*100, err)
			return
		}

		// 更新实时值并通知回调
		e.mu.Lock()
		e.currentPercent = currentPercent
		onStep := e.onStepFunc
		e.mu.Unlock()

		if onStep != nil {
			onStep(e.monitorName, currentPercent)
		}

		// 如果达到目标值，结束过渡
		if abs(currentPercent-task.target) < epsilon {
			logger.Debugf("[%s] Transition completed: %.2f%% -> %.2f%%", e.monitorName, task.startValue*100, task.target*100)
			e.mu.Lock()
			e.hasCurrentPercent = false
			e.mu.Unlock()
			return
		}

		// 等待下一个步进时间
		if step < totalSteps {
			select {
			case <-task.ctx.Done():
				logger.Debugf("[%s] Transition cancelled at %.2f%%", e.monitorName, currentPercent*100)
				return
			case <-ticker.C:
				// 继续下一步
			}
		}
	}

	logger.Debugf("[%s] Transition completed: %.2f%% -> %.2f%%", e.monitorName, task.startValue*100, task.target*100)
	e.mu.Lock()
	e.hasCurrentPercent = false
	e.mu.Unlock()
}

// calculateTransitionParams 计算过渡参数
func (e *TransitionExecutor) calculateTransitionParams(task *transitionTask) (totalSteps int, stepSize float64, stepInterval time.Duration) {
	deltaPercent := task.target - task.startValue
	if abs(deltaPercent) < epsilon {
		return 0, 0, 0
	}

	// 计算总步数：总百分比变化 / 步进百分比
	// 例如：变化 50%，步进 0.1%，则步数 = 50 / 0.1 = 500 步
	totalSteps = int(abs(deltaPercent*100) / task.config.StepPercent)
	if totalSteps <= 0 {
		totalSteps = 1
	}

	// 计算每步的百分比变化
	stepSize = deltaPercent / float64(totalSteps)

	// 计算每步的时间间隔
	stepInterval = task.config.Duration / time.Duration(totalSteps)

	// 确保间隔不小于最小间隔
	// 如果间隔太小，则根据最小间隔重新计算步数
	if stepInterval < task.config.MinStepInterval {
		totalSteps = int(task.config.Duration / task.config.MinStepInterval)
		if totalSteps <= 0 {
			totalSteps = 1
		}
		stepSize = deltaPercent / float64(totalSteps)
		stepInterval = task.config.MinStepInterval
	}
	return totalSteps, stepSize, stepInterval
}

// UpdateConfig 更新配置
func (e *TransitionExecutor) UpdateConfig(config TransitionConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.currentTask != nil && (e.config.Duration != config.Duration) {
		e.cancelTask(e.currentTask)
		e.currentTask = nil
		logger.Debugf("[%s] Config updated, cancelled current transition", e.monitorName)
	}

	config.Validate()
	e.config = config
}

// Stop 停止当前过渡并等待 goroutine 退出
func (e *TransitionExecutor) Stop() {
	e.mu.Lock()
	if e.currentTask != nil {
		e.cancelTask(e.currentTask)
		e.currentTask = nil
	}
	e.hasCurrentPercent = false
	e.mu.Unlock()

	// 等待 goroutine 退出（需要在锁外等待，否则死锁）
	e.wg.Wait()
}

// IsRunning 检查是否正在执行过渡
func (e *TransitionExecutor) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentTask != nil
}

// typeName 获取类型名称
func (e *TransitionExecutor) typeName() string {
	switch e.brightnessType {
	case TypeBacklight:
		return "Backlight"
	case TypeGamma:
		return "Gamma"
	default:
		return "Unknown"
	}
}

// TransitionManager 统一过渡管理器
// 按显示器名称管理所有显示器的亮度过渡
type TransitionManager struct {
	mu sync.Mutex

	// 执行器映射（按显示器名称）
	executors map[string]*TransitionExecutor

	// 配置
	config TransitionConfig

	// 获取显示器亮度类型的回调
	getBrightnessTypeFunc func(monitorName string) BrightnessType

	// 设置 Backlight 亮度的回调（百分比）
	setBacklightFunc func(percent float64) error

	// 获取 Backlight 当前亮度的回调（百分比）
	getBacklightFunc func() (float64, error)

	// 设置 Gamma 亮度的回调
	setGammaFunc func(monitorName string, percent float64) error

	// 获取 Gamma 当前亮度的回调
	getGammaFunc func(monitorName string) (float64, error)

	// 每步回调（用于同步亮度属性到 D-Bus），参数为(显示器名称, 当前亮度百分比)
	onStepFunc func(monitorName string, percent float64)
}

// NewTransitionManager 创建统一过渡管理器
func NewTransitionManager() *TransitionManager {
	return &TransitionManager{
		executors: make(map[string]*TransitionExecutor),
		config:    DefaultTransitionConfig(),
	}
}

// SetEnabled 设置是否启用过渡
func (m *TransitionManager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Enabled = enabled
	for _, executor := range m.executors {
		executor.UpdateConfig(m.config)
	}
}

// SetOnStepFunc 设置每步回调（用于在过渡过程中同步亮度属性到 D-Bus），参数为(显示器名称, 当前亮度百分比)
func (m *TransitionManager) SetOnStepFunc(fn func(monitorName string, percent float64)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStepFunc = fn
	for _, executor := range m.executors {
		executor.SetOnStepFunc(fn)
	}
}

// SetDuration 设置过渡时长（毫秒）
func (m *TransitionManager) SetDuration(durationMs int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Duration = time.Duration(durationMs) * time.Millisecond
	for _, executor := range m.executors {
		executor.UpdateConfig(m.config)
	}
}

// SetStepPercent 设置步进百分比
func (m *TransitionManager) SetStepPercent(stepPercent float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.StepPercent = stepPercent
	for _, executor := range m.executors {
		executor.UpdateConfig(m.config)
	}
}

// SetMinStepInterval 设置最小步进间隔（毫秒）
func (m *TransitionManager) SetMinStepInterval(intervalMs int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.MinStepInterval = time.Duration(intervalMs) * time.Millisecond
	if m.config.MinStepInterval < 0 {
		m.config.MinStepInterval = DefaultMinStepInterval
	}
	for _, executor := range m.executors {
		executor.UpdateConfig(m.config)
	}
}

// SetGetBrightnessTypeFunc 设置获取亮度类型的回调
func (m *TransitionManager) SetGetBrightnessTypeFunc(fn func(monitorName string) BrightnessType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getBrightnessTypeFunc = fn
}

// SetBacklightFuncs 设置 Backlight 回调
func (m *TransitionManager) SetBacklightFuncs(
	setFunc func(percent float64) error,
	getFunc func() (float64, error),
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setBacklightFunc = setFunc
	m.getBacklightFunc = getFunc
}

// SetGammaFuncs 设置 Gamma 回调
func (m *TransitionManager) SetGammaFuncs(
	setFunc func(monitorName string, percent float64) error,
	getFunc func(monitorName string) (float64, error),
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setGammaFunc = setFunc
	m.getGammaFunc = getFunc
}

// SetBrightness 设置指定显示器的亮度（百分比，0.0 - 1.0）
func (m *TransitionManager) SetBrightness(monitorName string, targetPercent float64, forceTransition bool) error {
	m.mu.Lock()
	executor, exists := m.executors[monitorName]
	config := m.config
	getBrightnessType := m.getBrightnessTypeFunc
	m.mu.Unlock()

	// 如果执行器不存在，创建新的
	if !exists {
		var brightnessType BrightnessType = TypeGamma // 默认 Gamma
		if getBrightnessType != nil {
			brightnessType = getBrightnessType(monitorName)
		}

		executor = m.createExecutor(monitorName, brightnessType, config)
		if executor == nil {
			return fmt.Errorf("failed to create executor for monitor %s", monitorName)
		}

		m.mu.Lock()
		m.executors[monitorName] = executor
		onStep := m.onStepFunc
		m.mu.Unlock()

		if onStep != nil {
			executor.SetOnStepFunc(onStep)
		}
	}

	return executor.SetBrightnessWithForce(targetPercent, forceTransition)
}

// createExecutor 创建执行器
func (m *TransitionManager) createExecutor(monitorName string, brightnessType BrightnessType, config TransitionConfig) *TransitionExecutor {
	m.mu.Lock()
	setBacklight := m.setBacklightFunc
	getBacklight := m.getBacklightFunc
	setGamma := m.setGammaFunc
	getGamma := m.getGammaFunc
	m.mu.Unlock()

	switch brightnessType {
	case TypeBacklight:
		setter := func(percent float64) error {
			if setBacklight == nil {
				return fmt.Errorf("backlight setter not configured")
			}
			return setBacklight(percent)
		}

		getter := func() (float64, error) {
			if getBacklight == nil {
				return 0.5, fmt.Errorf("backlight getter not configured")
			}
			return getBacklight()
		}

		return NewTransitionExecutor(monitorName, TypeBacklight, setter, getter, config)

	default: // TypeGamma
		setter := func(percent float64) error {
			if setGamma != nil {
				return setGamma(monitorName, percent)
			}
			return fmt.Errorf("gamma setter not configured")
		}

		getter := func() (float64, error) {
			if getGamma == nil {
				return 0.5, fmt.Errorf("gamma getter not configured")
			}
			return getGamma(monitorName)
		}

		return NewTransitionExecutor(monitorName, TypeGamma, setter, getter, config)
	}
}

// Stop 停止所有过渡
func (m *TransitionManager) Stop() {
	m.mu.Lock()
	executors := make([]*TransitionExecutor, 0, len(m.executors))
	for _, executor := range m.executors {
		executors = append(executors, executor)
	}
	m.mu.Unlock()

	for _, executor := range executors {
		executor.Stop()
	}
}

// StopMonitor 停止指定显示器的过渡
func (m *TransitionManager) StopMonitor(monitorName string) {
	m.mu.Lock()
	executor, exists := m.executors[monitorName]
	m.mu.Unlock()

	if exists {
		executor.Stop()
	}
}

// GetConfig 获取当前配置
func (m *TransitionManager) GetConfig() TransitionConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.config
}
