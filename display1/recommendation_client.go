// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package display1

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-dbus-factory/session/org.deepin.dde.ambientbrightness1"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/session/org.freedesktop.dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
)

const (
	ambientBrightnessInterface   = "org.deepin.dde.AmbientBrightness1"
	ambientBrightnessStateActive = "Active"
)

type RecommendationState struct {
	Available             bool
	Enabled               bool
	Supported             bool
	State                 string
	RecommendedBrightness float64
}

// RecommendationClient 通过生成的 go-dbus-factory 客户端与 AmbientBrightness1 服务通信。
type RecommendationClient struct {
	ambient    ambientbrightness1.AmbientBrightness
	dbusDaemon ofdbus.DBus

	mu       sync.Mutex
	state    RecommendationState
	callback func(RecommendationState)
}

func newRecommendationClient(conn *dbus.Conn, signalLoop *dbusutil.SignalLoop) *RecommendationClient {
	ambient := ambientbrightness1.NewAmbientBrightness(conn)
	ambient.InitSignalExt(signalLoop, true)

	dbusDaemon := ofdbus.NewDBus(conn)
	dbusDaemon.InitSignalExt(signalLoop, true)

	return &RecommendationClient{
		ambient:    ambient,
		dbusDaemon: dbusDaemon,
	}
}

func (c *RecommendationClient) Connect(callback func(RecommendationState)) error {
	if callback == nil {
		return errors.New("ambient brightness callback is nil")
	}

	c.mu.Lock()
	c.callback = callback
	c.mu.Unlock()

	_, propertiesErr := c.ambient.ConnectPropertiesChanged(c.handlePropertiesChanged)
	_, ownerErr := c.dbusDaemon.ConnectNameOwnerChanged(func(name, _, newOwner string) {
		if name != c.ambient.ServiceName_() {
			return
		}
		if newOwner == "" {
			c.setState(RecommendationState{})
			return
		}
		if err := c.Refresh(); err != nil {
			logger.Warning("[AutoBrightness] failed to refresh recommendation state:", err)
		}
	})

	return errors.Join(propertiesErr, ownerErr)
}

func (c *RecommendationClient) Refresh() error {
	state, err := c.readState()
	if err != nil {
		c.setState(RecommendationState{})
		return err
	}
	c.setState(state)
	return nil
}

// RefreshSilent 读取完整状态并缓存，不触发 onStateChanged 回调。
// 用于初始化阶段：先建立状态缓存，等 PropertiesChanged 信号
// 触发时才决定是否应用推荐值。
func (c *RecommendationClient) RefreshSilent() error {
	state, err := c.readState()
	if err != nil {
		c.mu.Lock()
		c.state = RecommendationState{}
		c.mu.Unlock()
		return err
	}
	c.mu.Lock()
	c.state = state
	c.mu.Unlock()
	return nil
}

func (c *RecommendationClient) Enable(enabled bool) error {
	return c.ambient.Enable(0, enabled)
}

func (c *RecommendationClient) Destroy() {
	c.ambient.RemoveAllHandlers()
	c.dbusDaemon.RemoveHandler(proxy.RemoveAllHandlers)

	c.mu.Lock()
	c.callback = nil
	c.state = RecommendationState{}
	c.mu.Unlock()
}

// readState 通过 factory 客户端逐个读取属性。
func (c *RecommendationClient) readState() (RecommendationState, error) {
	var state RecommendationState

	supported, err := c.ambient.Supported().Get(0)
	if err != nil {
		return RecommendationState{}, fmt.Errorf("get Supported: %w", err)
	}
	state.Supported = supported

	enabled, err := c.ambient.Enabled().Get(0)
	if err != nil {
		return RecommendationState{}, fmt.Errorf("get Enabled: %w", err)
	}
	state.Enabled = enabled

	stateName, err := c.ambient.State().Get(0)
	if err != nil {
		return RecommendationState{}, fmt.Errorf("get State: %w", err)
	}
	state.State = stateName

	recommended, err := c.ambient.RecommendedBrightness().Get(0)
	if err != nil {
		return RecommendationState{}, fmt.Errorf("get RecommendedBrightness: %w", err)
	}
	if !isValidRecommendedBrightness(recommended) {
		return RecommendationState{}, fmt.Errorf("invalid RecommendedBrightness value %v", recommended)
	}
	state.RecommendedBrightness = recommended

	state.Available = true
	return state, nil
}

// parseRecommendationState 从原始 D-Bus 属性映射解析状态。
// 保留给 manager.go 中的 readAmbientStateFromBus 和测试使用。
func parseRecommendationState(values map[string]dbus.Variant) (RecommendationState, error) {
	enabledValue, ok := values["Enabled"]
	if !ok {
		return RecommendationState{}, errors.New("missing Enabled property")
	}
	enabled, ok := enabledValue.Value().(bool)
	if !ok {
		return RecommendationState{}, fmt.Errorf("invalid Enabled property type %T", enabledValue.Value())
	}

	stateValue, ok := values["State"]
	if !ok {
		return RecommendationState{}, errors.New("missing State property")
	}
	stateName, ok := stateValue.Value().(string)
	if !ok {
		return RecommendationState{}, fmt.Errorf("invalid State property type %T", stateValue.Value())
	}

	supportedValue, ok := values["Supported"]
	if !ok {
		return RecommendationState{}, errors.New("missing Supported property")
	}
	supported, ok := supportedValue.Value().(bool)
	if !ok {
		return RecommendationState{}, fmt.Errorf("invalid Supported property type %T", supportedValue.Value())
	}

	recommendedValue, ok := values["RecommendedBrightness"]
	if !ok {
		return RecommendationState{}, errors.New("missing RecommendedBrightness property")
	}
	recommended, ok := recommendedValue.Value().(float64)
	if !ok {
		return RecommendationState{}, fmt.Errorf("invalid RecommendedBrightness property type %T", recommendedValue.Value())
	}
	if !isValidRecommendedBrightness(recommended) {
		return RecommendationState{}, fmt.Errorf("invalid RecommendedBrightness value %v", recommended)
	}

	return RecommendationState{
		Available:             true,
		Enabled:               enabled,
		Supported:             supported,
		State:                 stateName,
		RecommendedBrightness: recommended,
	}, nil
}

func isValidRecommendedBrightness(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

// handlePropertiesChanged 处理推荐服务属性变化信号。
func (c *RecommendationClient) handlePropertiesChanged(interfaceName string,
	changed map[string]dbus.Variant, invalidated []string) {
	if interfaceName != ambientBrightnessInterface {
		return
	}

	c.mu.Lock()
	state := c.state
	state.Available = true

	for _, name := range invalidated {
		switch name {
		case "Enabled":
			state.Enabled = false
		case "State":
			state.State = ""
		case "Supported":
			state.Supported = false
		case "RecommendedBrightness":
			state.RecommendedBrightness = math.NaN()
		}
	}

	if value, ok := changed["Enabled"]; ok {
		enabled, validType := value.Value().(bool)
		if !validType {
			logger.Warningf("[AutoBrightness] Invalid Enabled property type %T", value.Value())
			state.Enabled = false
		} else {
			state.Enabled = enabled
		}
	}

	if value, ok := changed["State"]; ok {
		stateName, validType := value.Value().(string)
		if !validType {
			logger.Warningf("[AutoBrightness] Invalid State property type %T", value.Value())
			state.State = ""
		} else {
			state.State = stateName
		}
	}

	if value, ok := changed["Supported"]; ok {
		supported, validType := value.Value().(bool)
		if !validType {
			logger.Warningf("[AutoBrightness] Invalid Supported property type %T", value.Value())
			state.Supported = false
		} else {
			state.Supported = supported
		}
	}

	if value, ok := changed["RecommendedBrightness"]; ok {
		recommended, validType := value.Value().(float64)
		if !validType || !isValidRecommendedBrightness(recommended) {
			logger.Warningf("[AutoBrightness] Invalid RecommendedBrightness value %v", value.Value())
			state.RecommendedBrightness = math.NaN()
		} else {
			state.RecommendedBrightness = recommended
		}
	}

	if state == c.state {
		c.mu.Unlock()
		return
	}
	c.state = state
	callback := c.callback
	c.mu.Unlock()

	if callback != nil {
		callback(state)
	}
}

func (c *RecommendationClient) setState(state RecommendationState) {
	c.mu.Lock()
	if state == c.state {
		c.mu.Unlock()
		return
	}
	c.state = state
	callback := c.callback
	c.mu.Unlock()

	if callback != nil {
		callback(state)
	}
}
