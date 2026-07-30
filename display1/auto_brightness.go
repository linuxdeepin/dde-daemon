// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package display1

import (
	"errors"
	"fmt"
	"sync"

	"github.com/linuxdeepin/dde-daemon/display1/brightness"
)

// AutoBrightnessManager 消费 AmbientBrightness1 的推荐值，并串行应用到内置屏。
// 自动亮度开关及其持久化由 AmbientBrightness1 独占管理。
type AutoBrightnessManager struct {
	manager       *Manager
	ambientClient *RecommendationClient

	enabled               bool
	supported             bool
	serviceAvailable      bool
	ambientState          string
	recommendedBrightness float64
	running               bool
	held                  bool

	transition *brightness.BrightnessTransition
	canApply   func() bool

	mutex   sync.RWMutex
	applyMu sync.Mutex
}

func NewAutoBrightnessManager() *AutoBrightnessManager {
	return &AutoBrightnessManager{
		canApply: func() bool { return true },
	}
}

func (abm *AutoBrightnessManager) Initialize(manager *Manager) error {
	if manager == nil {
		return errors.New("manager cannot be nil")
	}

	builtinMonitor := manager.getBuiltinMonitor()
	if builtinMonitor == nil {
		return fmt.Errorf("no builtin monitor found (total monitors: %d)", len(manager.getConnectedMonitors()))
	}
	canSet, busErr := manager.CanSetBrightness(builtinMonitor.Name)
	if busErr != nil {
		return fmt.Errorf("check brightness capability for %s: %w", builtinMonitor.Name, busErr)
	}
	if !canSet {
		return fmt.Errorf("cannot set brightness for builtin monitor: %s", builtinMonitor.Name)
	}

	abm.mutex.Lock()
	abm.manager = manager
	abm.transition = brightness.NewBrightnessTransition(abm.setBrightness)
	abm.transition.SetOnComplete(abm.onTransitionComplete)
	abm.canApply = manager.isSessionActive
	abm.mutex.Unlock()

	client := newRecommendationClient(manager.service.Conn(), manager.sessionSigLoop)
	if err := client.Connect(abm.onAmbientBrightnessStateChanged); err != nil {
		client.Destroy()
		return fmt.Errorf("connect ambient brightness service signals: %w", err)
	}

	abm.mutex.Lock()
	abm.ambientClient = client
	abm.mutex.Unlock()

	if err := client.RefreshSilent(); err != nil {
		logger.Info("[AutoBrightness] Recommendation service is currently unavailable:", err)
	}

	logger.Info("[AutoBrightness] Recommendation consumer initialized")
	return nil
}

func (abm *AutoBrightnessManager) Start() error {
	return abm.SetEnabled(true)
}

func (abm *AutoBrightnessManager) Stop() error {
	return abm.SetEnabled(false)
}

// stopTransition 同步取消自动亮度事务。返回后旧事务不会再写亮度。
func (abm *AutoBrightnessManager) stopTransition() {
	abm.mutex.RLock()
	transition := abm.transition
	abm.mutex.RUnlock()
	if transition != nil {
		transition.Stop()
	}
}

func (abm *AutoBrightnessManager) Cleanup() error {
	abm.stopTransition()

	abm.mutex.Lock()
	client := abm.ambientClient
	abm.ambientClient = nil
	abm.manager = nil
	abm.mutex.Unlock()
	if client != nil {
		client.Destroy()
	}
	logger.Info("[AutoBrightness] Recommendation consumer cleaned up")
	return nil
}

// SetEnabled 将 Display1 的兼容接口代理到 AmbientBrightness1.Enable。
// 不直接修改 enabled/running —— 由 onStateChanged 通过 PropertiesChanged 统一管理。
func (abm *AutoBrightnessManager) SetEnabled(enabled bool) error {
	abm.mutex.RLock()
	client := abm.ambientClient
	abm.mutex.RUnlock()
	if client == nil {
		return errors.New("ambient brightness service client is unavailable")
	}

	if !enabled {
		// 立即停止当前渐变，不再等待服务反馈
		abm.stopTransition()
	}

	if err := client.Enable(enabled); err != nil {
		return err
	}

	// Refresh 同步服务最新状态，触发 onStateChanged
	return client.Refresh()
}

// DisableForManualAdjustment 让手动事务抢占自动事务并关闭 AmbientBrightness1。
func (abm *AutoBrightnessManager) DisableForManualAdjustment() error {
	if !abm.IsRunning() {
		abm.stopTransition()
		return nil
	}

	logger.Info("[AutoBrightness] Manual adjustment disabled automatic brightness")

	return abm.SetEnabled(false)
}
func (abm *AutoBrightnessManager) hold() {
	abm.mutex.Lock()
	abm.held = true
	transition := abm.transition
	abm.mutex.Unlock()
	if transition != nil {
		transition.Stop()
	}
}

func (abm *AutoBrightnessManager) resume() {
	abm.mutex.Lock()
	abm.held = false
	shouldApply := abm.running
	abm.mutex.Unlock()

	if shouldApply {
		abm.applyRecommendedBrightness()
	}
}

func (abm *AutoBrightnessManager) onAmbientBrightnessStateChanged(state RecommendationState) {

	abm.mutex.Lock()
	oldRunning := abm.running
	abm.serviceAvailable = state.Available
	abm.enabled = state.Enabled
	abm.supported = state.Supported
	abm.ambientState = state.State
	abm.recommendedBrightness = state.RecommendedBrightness
	abm.running = state.Enabled && state.State == ambientBrightnessStateActive && state.Supported
	newRunning := abm.running
	manager := abm.manager
	transition := abm.transition
	abm.mutex.Unlock()

	if manager != nil {
		manager.setPropAutoBrightnessEnabled(state.Enabled)
		manager.setPropAutoBrightnessSupported(state.Supported)
	}

	// running 从 true → false：停止渐变，不写亮度
	if oldRunning && !newRunning && transition != nil {
		transition.Stop()
	}

	// running 为 true：应用推荐值（State=Active 时）
	if newRunning {
		abm.applyRecommendedBrightness()
	}
}

func (abm *AutoBrightnessManager) applyRecommendedBrightness() {
	abm.applyMu.Lock()
	defer abm.applyMu.Unlock()

	abm.mutex.RLock()
	recommended := abm.recommendedBrightness
	shouldApply := abm.running && !abm.held && isValidRecommendedBrightness(recommended)
	transition := abm.transition
	canApply := abm.canApply
	manager := abm.manager
	abm.mutex.RUnlock()

	if !shouldApply || transition == nil || manager == nil {
		return
	}
	if canApply != nil && !canApply() {
		return
	}

	builtinMonitor := manager.getBuiltinMonitor()
	if builtinMonitor == nil {
		logger.Warning("[AutoBrightness] no builtin monitor available")
		return
	}

	scale := manager.getBrightnessScale()
	target := scaleBrightness(recommended, scale)
	current := manager.getMonitorBrightness(builtinMonitor.Name)
	if current < 0 {
		current = target
	}

	if transition.Update(target) {
		return
	}
	transition.Run(current, target)
}

func (abm *AutoBrightnessManager) setBrightness(value float64) error {
	abm.mutex.RLock()
	manager := abm.manager
	abm.mutex.RUnlock()
	if manager == nil {
		return errors.New("manager is nil")
	}
	builtinMonitor := manager.getBuiltinMonitor()
	if builtinMonitor == nil {
		return errors.New("no builtin monitor")
	}
	return manager.setBrightnessAndSync(builtinMonitor.Name, value)
}

// onTransitionComplete 渐变正常完成后保存亮度到配置。
// 保存的是推荐原始值（未缩放），不是 transition 完成时的实际亮度值。
func (abm *AutoBrightnessManager) onTransitionComplete(value float64) {
	abm.mutex.RLock()
	manager := abm.manager
	recommended := abm.recommendedBrightness
	abm.mutex.RUnlock()
	if manager == nil {
		return
	}
	builtinMonitor := manager.getBuiltinMonitor()
	if builtinMonitor == nil {
		return
	}
	logger.Infof("[AutoBrightness] transition complete (effective=%.3f), saving recommended brightness %.3f for %s", value, recommended, builtinMonitor.Name)
	if err := manager.saveBrightnessInCfg(map[string]float64{
		builtinMonitor.Name: recommended,
	}); err != nil {
		logger.Warning("[AutoBrightness] failed to save brightness after transition:", err)
	}
}

func (abm *AutoBrightnessManager) IsSupported() bool {
	abm.mutex.RLock()
	defer abm.mutex.RUnlock()
	return abm.supported
}

func (abm *AutoBrightnessManager) IsEnabled() bool {
	abm.mutex.RLock()
	defer abm.mutex.RUnlock()
	return abm.enabled
}

// IsRunning 返回自动亮度是否正在应用推荐值。
// running = enabled && State==Active && Supported
func (abm *AutoBrightnessManager) IsRunning() bool {
	abm.mutex.RLock()
	defer abm.mutex.RUnlock()
	return abm.running
}

func (abm *AutoBrightnessManager) GetStatus() map[string]interface{} {
	abm.mutex.RLock()
	defer abm.mutex.RUnlock()
	return map[string]interface{}{
		"service_available":      abm.serviceAvailable,
		"enabled":                abm.enabled,
		"state":                  abm.ambientState,
		"supported":              abm.supported,
		"running":                abm.running,
		"held":                   abm.held,
		"recommended_brightness": abm.recommendedBrightness,
	}
}

func (m *Manager) isSessionActive() bool {
	m.sessionActiveMu.RLock()
	defer m.sessionActiveMu.RUnlock()
	return m.sessionActive
}
