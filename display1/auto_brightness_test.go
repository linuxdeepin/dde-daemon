// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package display1

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxdeepin/dde-daemon/display1/brightness"
)

func TestParseAmbientBrightnessState(t *testing.T) {
	state, err := parseRecommendationState(map[string]dbus.Variant{
		"Enabled":               dbus.MakeVariant(true),
		"State":                 dbus.MakeVariant(ambientBrightnessStateActive),
		"Supported":             dbus.MakeVariant(true),
		"RecommendedBrightness": dbus.MakeVariant(0.37),
	})
	require.NoError(t, err)
	assert.Equal(t, RecommendationState{
		Available:             true,
		Enabled:               true,
		Supported:             true,
		State:                 ambientBrightnessStateActive,
		RecommendedBrightness: 0.37,
	}, state)

	_, err = parseRecommendationState(map[string]dbus.Variant{
		"Enabled":               dbus.MakeVariant(true),
		"State":                 dbus.MakeVariant(ambientBrightnessStateActive),
		"Supported":             dbus.MakeVariant(true),
		"RecommendedBrightness": dbus.MakeVariant(math.NaN()),
	})
	require.Error(t, err)
}

func TestRecommendationPropertiesChangedUsesState(t *testing.T) {
	initial, err := parseRecommendationState(map[string]dbus.Variant{
		"Enabled":               dbus.MakeVariant(true),
		"State":                 dbus.MakeVariant("WaitingForSample"),
		"Supported":             dbus.MakeVariant(true),
		"RecommendedBrightness": dbus.MakeVariant(0.18),
	})
	require.NoError(t, err)

	client := &RecommendationClient{}
	client.setState(initial)
	client.handlePropertiesChanged(ambientBrightnessInterface, map[string]dbus.Variant{
		"RecommendedBrightness": dbus.MakeVariant(0.41),
	}, nil)
	client.handlePropertiesChanged(ambientBrightnessInterface, map[string]dbus.Variant{
		"State": dbus.MakeVariant(ambientBrightnessStateActive),
	}, nil)

	assert.Equal(t, ambientBrightnessStateActive, client.state.State)
	assert.Equal(t, 0.41, client.state.RecommendedBrightness)
}

func TestRecommendationPropertiesChangedRecoversFromInvalidValue(t *testing.T) {
	client := &RecommendationClient{
		state: RecommendationState{
			Available:             true,
			Enabled:               true,
			Supported:             true,
			State:                 ambientBrightnessStateActive,
			RecommendedBrightness: 0.4,
		},
	}

	client.handlePropertiesChanged(ambientBrightnessInterface, map[string]dbus.Variant{
		"RecommendedBrightness": dbus.MakeVariant(math.NaN()),
	}, nil)
	assert.True(t, math.IsNaN(client.state.RecommendedBrightness))

	client.handlePropertiesChanged(ambientBrightnessInterface, map[string]dbus.Variant{
		"RecommendedBrightness": dbus.MakeVariant(0.5),
	}, nil)
	assert.Equal(t, 0.5, client.state.RecommendedBrightness)
}

func TestRecommendationPropertiesChangedUpdatesLifecycleState(t *testing.T) {
	client := &RecommendationClient{
		state: RecommendationState{
			Available:             true,
			Enabled:               true,
			Supported:             true,
			State:                 ambientBrightnessStateActive,
			RecommendedBrightness: 0.8,
		},
	}

	client.handlePropertiesChanged(ambientBrightnessInterface, map[string]dbus.Variant{
		"Enabled":   dbus.MakeVariant(false),
		"State":     dbus.MakeVariant("Disabled"),
		"Supported": dbus.MakeVariant(true),
	}, nil)

	assert.False(t, client.state.Enabled)
	assert.Equal(t, "Disabled", client.state.State)
}

func TestBrightnessTransitionUpdateReachesLatestTarget(t *testing.T) {
	var mu sync.Mutex
	var values []float64
	firstWrite := make(chan struct{})
	completed := make(chan struct{})
	var firstOnce sync.Once
	var completedOnce sync.Once

	tr := brightness.NewBrightnessTransition(func(value float64) error {
		mu.Lock()
		values = append(values, value)
		mu.Unlock()
		firstOnce.Do(func() { close(firstWrite) })
		if math.Abs(value-0.3) < 0.0001 {
			completedOnce.Do(func() { close(completed) })
		}
		return nil
	})
	defer tr.Stop()

	tr.Run(0.1, 0.9)
	select {
	case <-firstWrite:
	case <-time.After(time.Second):
		t.Fatal("automatic transaction did not write its first step")
	}
	require.True(t, tr.Update(0.3))

	select {
	case <-completed:
	case <-time.After(2 * time.Second):
		t.Fatal("updated automatic transaction did not reach the latest target")
	}
	require.Eventually(t, func() bool { return !tr.IsRunning() }, time.Second, 10*time.Millisecond)

	mu.Lock()
	last := values[len(values)-1]
	mu.Unlock()
	assert.InDelta(t, 0.3, last, 0.0001)
}

func TestBrightnessTransitionStopWaitsForInflightWrite(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	stopped := make(chan struct{})
	var enteredOnce sync.Once
	var writes atomic.Int32

	tr := brightness.NewBrightnessTransition(func(float64) error {
		writes.Add(1)
		enteredOnce.Do(func() { close(entered) })
		<-release
		return nil
	})
	tr.Run(0.1, 0.9)

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("automatic transaction did not enter the hardware writer")
	}
	go func() {
		tr.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("Stop returned before the in-flight hardware write completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after the in-flight write completed")
	}
	count := writes.Load()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, count, writes.Load(), "a stopped transaction wrote another brightness step")
	assert.False(t, tr.IsRunning())
}

func TestBrightnessTransitionStopBeforeRun(t *testing.T) {
	tr := brightness.NewBrightnessTransition(func(float64) error { return nil })
	tr.Stop()
	assert.False(t, tr.IsRunning())
}

func TestParseRecommendationStateThreeWayBranch(t *testing.T) {
	// 路径 1: 自动亮度开启 + State=Active + 推荐值有效 → 用推荐亮度
	t.Run("ambient active and valid", func(t *testing.T) {
		state, err := parseRecommendationState(map[string]dbus.Variant{
			"Enabled":               dbus.MakeVariant(true),
			"State":                 dbus.MakeVariant(ambientBrightnessStateActive),
			"Supported":             dbus.MakeVariant(true),
			"RecommendedBrightness": dbus.MakeVariant(0.75),
		})
		require.NoError(t, err)
		assert.True(t, state.Enabled)
		assert.Equal(t, ambientBrightnessStateActive, state.State)
		assert.True(t, state.Supported)
		assert.Equal(t, 0.75, state.RecommendedBrightness)
		assert.True(t, isValidRecommendedBrightness(state.RecommendedBrightness))
	})

	// 路径 2: 自动亮度开启 + State=WaitingForSample → 推荐值未就绪
	t.Run("ambient waiting for sample", func(t *testing.T) {
		state, err := parseRecommendationState(map[string]dbus.Variant{
			"Enabled":               dbus.MakeVariant(true),
			"State":                 dbus.MakeVariant("WaitingForSample"),
			"Supported":             dbus.MakeVariant(true),
			"RecommendedBrightness": dbus.MakeVariant(0.5),
		})
		require.NoError(t, err)
		assert.True(t, state.Enabled)
		assert.Equal(t, "WaitingForSample", state.State)
		assert.NotEqual(t, ambientBrightnessStateActive, state.State, "should not be Active")
	})

	// 路径 2b: 自动亮度开启 + State=Unavailable → 光感不可用
	t.Run("ambient unavailable", func(t *testing.T) {
		state, err := parseRecommendationState(map[string]dbus.Variant{
			"Enabled":               dbus.MakeVariant(true),
			"State":                 dbus.MakeVariant("Unavailable"),
			"Supported":             dbus.MakeVariant(false),
			"RecommendedBrightness": dbus.MakeVariant(0.0),
		})
		require.NoError(t, err)
		assert.True(t, state.Enabled)
		assert.Equal(t, "Unavailable", state.State)
		assert.False(t, state.Supported)
	})

	// 路径 3: 自动亮度关闭 → 不触发，用旧配置亮度
	t.Run("ambient disabled", func(t *testing.T) {
		state, err := parseRecommendationState(map[string]dbus.Variant{
			"Enabled":               dbus.MakeVariant(false),
			"State":                 dbus.MakeVariant("Disabled"),
			"Supported":             dbus.MakeVariant(true),
			"RecommendedBrightness": dbus.MakeVariant(0.5),
		})
		require.NoError(t, err)
		assert.False(t, state.Enabled)
	})
}

func TestRecommendedBrightnessRespectsApplicationLifecycle(t *testing.T) {
	tests := []struct {
		name          string
		enabled       bool
		state         string
		supported     bool
		invalidTarget bool
		held          bool
		canApply      bool
		wantApplied   bool
	}{
		// State=Active + enabled + supported → running=true → 应用
		{name: "active", enabled: true, state: ambientBrightnessStateActive, supported: true, canApply: true, wantApplied: true},
		// enabled=false → running=false → 不应用
		{name: "disabled", state: ambientBrightnessStateActive, supported: true, canApply: true},
		// State=WaitingForSample → running=false → 不应用
		{name: "waiting for sample", enabled: true, state: "WaitingForSample", supported: true, canApply: true},
		// State=Unavailable → running=false → 不应用
		{name: "unavailable", enabled: true, state: "Unavailable", supported: true, canApply: true},
		// Supported=false → running=false → 不应用
		{name: "unsupported", enabled: true, state: ambientBrightnessStateActive, supported: false, canApply: true},
		// 推荐值无效 → 不应用
		{name: "invalid recommendation", enabled: true, state: ambientBrightnessStateActive, supported: true, invalidTarget: true, canApply: true},
		// held=true → 不应用
		{name: "held", enabled: true, state: ambientBrightnessStateActive, supported: true, held: true, canApply: true},
		// 会话非活跃 → canApply=false → 不应用
		{name: "inactive session", enabled: true, state: ambientBrightnessStateActive, supported: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				builtinMonitor: &Monitor{Name: "eDP-0"},
				Brightness:     map[string]float64{"eDP-0": 0.42},
				brightnessScale: 1.0,
			}
			target := 0.42
			if tt.invalidTarget {
				target = math.NaN()
			}
			abm := NewAutoBrightnessManager()
			abm.manager = manager
			abm.enabled = tt.enabled
			abm.running = tt.enabled && tt.state == ambientBrightnessStateActive && tt.supported
			abm.ambientState = tt.state
			abm.supported = tt.supported
			abm.recommendedBrightness = target
			abm.held = tt.held
			abm.canApply = func() bool { return tt.canApply }

			applied := false
			abm.transition = brightness.NewBrightnessTransition(func(float64) error {
				applied = true
				return nil
			})
			abm.applyRecommendedBrightness()
			assert.Equal(t, tt.wantApplied, applied)
			abm.transition.Stop()
		})
	}
}
