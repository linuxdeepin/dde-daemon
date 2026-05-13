// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package display1

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	sensorproxy "github.com/linuxdeepin/go-dbus-factory/system/net.hadess.sensorproxy"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
)

// SensorProxyClient 环境光传感器D-Bus客户端
// 职责：负责从光感服务获取数据并缓存，不负责滤波处理
type SensorProxyClient struct {
	sensorProxy     sensorproxy.SensorProxy
	dbusDaemon      ofdbus.DBus
	hasAmbientLight bool
	claimed         bool

	// 事件处理
	onServiceChange    func(bool)
	onLightLevelChange func(int)

	// 同步控制
	mutex sync.Mutex

	// 服务监控
	serviceAvailable bool
	serviceSigLoop   *dbusutil.SignalLoop // 服务监控的 SignalLoop

	// 错误处理
	maxRetries int
	retryDelay time.Duration

	// 光感数据缓存
	lastLightLevel     int       // 最后一次光感值（lux）
	lastLightLevelTime time.Time // 最后更新时间
}

// NewSensorProxyClient 创建新的传感器代理客户端
func NewSensorProxyClient(proxy sensorproxy.SensorProxy, dbusDaemon ofdbus.DBus) *SensorProxyClient {
	return &SensorProxyClient{
		sensorProxy:    proxy,
		dbusDaemon:     dbusDaemon,
		maxRetries:     3,
		retryDelay:     time.Millisecond * 500,
		lastLightLevel: -1,
	}
}

// Connect 连接到SensorProxy服务
func (c *SensorProxyClient) Connect(sigLoop *dbusutil.SignalLoop) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.sensorProxy == nil {
		return errors.New("SensorProxy is nil")
	}
	// 初始化信号处理
	c.sensorProxy.InitSignalExt(sigLoop, true)
	// 检查服务是否可用（带重试机制）
	err := c.checkServiceAvailableWithRetry()
	if err != nil {
		c.serviceAvailable = false
		return fmt.Errorf("SensorProxy service not available after retries: %w", err)
	}
	c.serviceAvailable = true
	// 检查是否有环境光传感器
	hasLight, err := c.hasAmbientLightInternal()
	if err != nil {
		return fmt.Errorf("failed to check ambient light sensor: %w", err)
	}
	c.hasAmbientLight = hasLight
	if !hasLight {
		return errors.New("no ambient light sensor available")
	}
	// 订阅属性变化信号（LightLevel 等）
	c.startSignalWatching()
	// 订阅服务所有者变化信号
	c.startServiceWatching()
	return nil
}

// Disconnect 断开连接
func (c *SensorProxyClient) Disconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	// 释放环境光传感器
	if c.claimed {
		err := c.releaseLightInternal()
		if err != nil {
			logger.Warning("[SensorProxy] Failed to release light sensor:", err)
		}
		c.claimed = false
	}
	// 停止服务监控的 SignalLoop
	if c.serviceSigLoop != nil {
		c.serviceSigLoop.Stop()
		c.serviceSigLoop = nil
	}
	// 移除所有信号处理器
	c.sensorProxy.RemoveHandler(proxy.RemoveAllHandlers)
	c.dbusDaemon.RemoveHandler(proxy.RemoveAllHandlers)

	c.serviceAvailable = false
	c.hasAmbientLight = false
	c.lastLightLevel = -1
	return nil
}

// ClaimLight 声明对环境光传感器的使用
func (c *SensorProxyClient) ClaimLight() error {
	logger.Debug("[SensorProxy] Claiming light sensor")
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if !c.serviceAvailable {
		return errors.New("SensorProxy service not available")
	}
	if !c.hasAmbientLight {
		return errors.New("no ambient light sensor")
	}
	if c.claimed {
		logger.Debug("[SensorProxy] Light sensor already claimed")
		return nil
	}
	err := c.claimLightWithRetry()
	if err != nil {
		return fmt.Errorf("failed to claim light sensor after retries: %w", err)
	}
	c.claimed = true
	return nil
}

// ReleaseLight 释放环境光传感器
func (c *SensorProxyClient) ReleaseLight() error {
	logger.Debug("[SensorProxy] Releasing light sensor")
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if !c.claimed {
		logger.Debug("[SensorProxy] Light sensor not claimed")
		return nil
	}
	err := c.releaseLightInternal()
	if err != nil {
		logger.Warning("[SensorProxy] Failed to release light sensor:", err)
		return err
	}
	c.claimed = false
	c.lastLightLevel = -1
	return nil
}

// GetCachedLightLevel 获取缓存的环境光强度（返回原始值，不滤波）
func (c *SensorProxyClient) GetCachedLightLevel() (int, error) {
	c.mutex.Lock()
	if !c.serviceAvailable {
		c.mutex.Unlock()
		return 0, errors.New("SensorProxy service not available")
	}

	if !c.hasAmbientLight {
		c.mutex.Unlock()
		return 0, errors.New("no ambient light sensor")
	}

	if !c.claimed {
		c.mutex.Unlock()
		return 0, errors.New("light sensor not claimed")
	}

	needInit := c.lastLightLevel < 0
	c.mutex.Unlock()

	if needInit {
		c.initializeCacheFromProperty()
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.lastLightLevel < 0 {
		return 0, errors.New("no light level data available")
	}

	logger.Debugf("[AutoBrightness::GetCachedLightLevel] Returning cached raw light level: %d lux", c.lastLightLevel)
	return c.lastLightLevel, nil
}

// GetLightLevel 获取实时的环境光强度（从 D-Bus 属性读取）
func (c *SensorProxyClient) GetLightLevel() (int, error) {
	c.mutex.Lock()
	if !c.serviceAvailable {
		c.mutex.Unlock()
		return 0, errors.New("SensorProxy service not available")
	}

	if !c.hasAmbientLight {
		c.mutex.Unlock()
		return 0, errors.New("no ambient light sensor")
	}

	if !c.claimed {
		c.mutex.Unlock()
		return 0, errors.New("light sensor not claimed")
	}
	c.mutex.Unlock()

	lightLevel, err := c.sensorProxy.LightLevel().Get(0)
	if err != nil {
		return 0, fmt.Errorf("failed to get LightLevel property: %w", err)
	}

	return int(lightLevel), nil
}

// HasAmbientLight 检查是否有环境光传感器
func (c *SensorProxyClient) HasAmbientLight() (bool, error) {
	c.mutex.Lock()
	if !c.serviceAvailable {
		c.mutex.Unlock()
		return false, errors.New("SensorProxy service not available")
	}
	c.mutex.Unlock()

	return c.hasAmbientLightInternal()
}

// SetServiceChangeCallback 设置服务状态变化回调
func (c *SensorProxyClient) SetServiceChangeCallback(callback func(bool)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.onServiceChange = callback
}

// SetLightLevelChangeCallback 设置光照值变化回调
func (c *SensorProxyClient) SetLightLevelChangeCallback(callback func(int)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.onLightLevelChange = callback
}

// IsServiceAvailable 检查服务是否可用
func (c *SensorProxyClient) IsServiceAvailable() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.serviceAvailable
}

// IsClaimed 检查是否已声明传感器
func (c *SensorProxyClient) IsClaimed() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.claimed
}

// 内部方法 - 不加锁
// checkServiceAvailableWithRetry 带重试机制检查服务是否可用
func (c *SensorProxyClient) checkServiceAvailableWithRetry() error {
	var lastErr error
	for i := 0; i < c.maxRetries; i++ {
		_, err := c.hasAmbientLightInternal()
		if err == nil {
			return nil
		}
		lastErr = err
		if i < c.maxRetries-1 {
			time.Sleep(c.retryDelay)
		}
	}
	return lastErr
}

// claimLightWithRetry 带重试机制声明环境光传感器
func (c *SensorProxyClient) claimLightWithRetry() error {
	var lastErr error
	for i := 0; i < c.maxRetries; i++ {
		err := c.claimLightInternal()
		if err == nil {
			return nil
		}
		lastErr = err
		if i < c.maxRetries-1 {
			time.Sleep(c.retryDelay)
		}
	}
	return lastErr
}

// hasAmbientLightInternal 内部检查环境光传感器
func (c *SensorProxyClient) hasAmbientLightInternal() (bool, error) {
	hasLight, err := c.sensorProxy.HasAmbientLight().Get(0)
	if err != nil {
		return false, err
	}
	return hasLight, nil
}

// claimLightInternal 内部声明环境光传感器
func (c *SensorProxyClient) claimLightInternal() error {
	return c.sensorProxy.ClaimLight(0)
}

// releaseLightInternal 内部释放环境光传感器
func (c *SensorProxyClient) releaseLightInternal() error {
	return c.sensorProxy.ReleaseLight(0)
}

// startSignalWatching 通过 proxy.Object 订阅属性变化信号
func (c *SensorProxyClient) startSignalWatching() {
	_, err := c.sensorProxy.ConnectPropertiesChanged(
		func(interfaceName string, changedProperties map[string]dbus.Variant,
			invalidatedProperties []string) {
			lightLevelVariant, exists := changedProperties["LightLevel"]
			if !exists {
				return
			}
			lightLevel, ok := lightLevelVariant.Value().(float64)
			if !ok {
				logger.Warning("[SensorProxy] Failed to convert LightLevel value to float")
				return
			}
			c.mutex.Lock()
			claimed := c.claimed
			c.mutex.Unlock()
			if claimed {
				c.lightValueFilter(int(lightLevel))
			}
		})
	if err != nil {
		logger.Warning("[SensorProxy] Failed to connect PropertiesChanged signal:", err)
	}
}

// startServiceWatching 通过 ofdbus.DBus 订阅服务所有者变化信号
func (c *SensorProxyClient) startServiceWatching() {
	// 如果已有 sigLoop，先停止它
	if c.serviceSigLoop != nil {
		c.serviceSigLoop.Stop()
		c.serviceSigLoop = nil
	}

	systemBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning("[SensorProxy] Failed to connect to system bus for service watching:", err)
		return
	}
	sigLoop := dbusutil.NewSignalLoop(systemBus, 10)
	sigLoop.Start()
	c.serviceSigLoop = sigLoop
	c.dbusDaemon.InitSignalExt(sigLoop, true)
	_, err = c.dbusDaemon.ConnectNameOwnerChanged(
		func(name, oldOwner, newOwner string) {
			if name != "net.hadess.SensorProxy" {
				return
			}
			serviceAvailable := newOwner != ""
			c.mutex.Lock()
			oldAvailable := c.serviceAvailable
			c.serviceAvailable = serviceAvailable
			if !serviceAvailable {
				c.claimed = false
				c.hasAmbientLight = false
			} else if !oldAvailable {
				go func() {
					time.Sleep(100 * time.Millisecond)
					hasLight, err := c.HasAmbientLight()
					if err == nil {
						c.mutex.Lock()
						c.hasAmbientLight = hasLight
						c.mutex.Unlock()
					}
				}()
			}
			callback := c.onServiceChange
			c.mutex.Unlock()
			if callback != nil {
				go callback(serviceAvailable)
			}
		})
	if err != nil {
		logger.Warning("[SensorProxy] Failed to connect NameOwnerChanged signal:", err)
	}
}
func (c *SensorProxyClient) lightValueFilter(newValue int) {
	logger.Infof("[AutoBrightness::RawLightSensor] Raw light sensor value: %d lux", newValue)

	c.mutex.Lock()
	c.lastLightLevel = newValue
	c.lastLightLevelTime = time.Now()
	callback := c.onLightLevelChange
	c.mutex.Unlock()

	logger.Debugf("[AutoBrightness::CachedLightValue] Cached raw light value: %d lux", newValue)

	if callback != nil {
		go callback(newValue)
	}
}

// initializeCacheFromProperty 从 D-Bus 属性读取当前光照值并缓存
func (c *SensorProxyClient) initializeCacheFromProperty() {
	lightLevel, err := c.sensorProxy.LightLevel().Get(0)
	if err != nil {
		logger.Warning("[AutoBrightness::LightSensor] Failed to get LightLevel property:", err)
		return
	}

	if lightLevel <= 0 {
		logger.Debug("[AutoBrightness::LightSensor] LightLevel property is zero or negative, skipping cache")
		return
	}

	c.mutex.Lock()
	c.lastLightLevel = int(lightLevel)
	c.lastLightLevelTime = time.Now()
	c.mutex.Unlock()

	logger.Infof("[AutoBrightness::LightSensor] Cached raw light value from property: %d lux", int(lightLevel))
}
