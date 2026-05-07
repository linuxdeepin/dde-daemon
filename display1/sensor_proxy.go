// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package display1

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	// net.hadess.SensorProxy D-Bus 接口常量
	hadessProxyService    = "net.hadess.SensorProxy"
	hadessProxyObjectPath = "/net/hadess/SensorProxy"
	hadessProxyInterface  = "net.hadess.SensorProxy"
	// 属性名称
	propHasAmbientLight = "HasAmbientLight"
	propLightLevel      = "LightLevel"
	propLightLevelUnit  = "LightLevelUnit"
	// 方法名称
	methodClaimLight   = "ClaimLight"
	methodReleaseLight = "ReleaseLight"
	// 信号名称
	signalPropertiesChanged = "org.freedesktop.DBus.Properties.PropertiesChanged"
)

// SensorProxyClient 环境光传感器D-Bus客户端
// 职责：负责从光感服务获取数据并缓存，不负责滤波处理
type SensorProxyClient struct {
	conn            *dbus.Conn
	sensorProxy     dbus.BusObject
	hasAmbientLight bool
	claimed         bool

	// 事件处理
	signalChan         chan *dbus.Signal
	onServiceChange    func(bool)
	onLightLevelChange func(int)

	// 同步控制
	mutex sync.Mutex

	// 服务监控
	serviceAvailable bool
	ownerWatcher     chan *dbus.Signal

	// 错误处理
	maxRetries int
	retryDelay time.Duration

	// 光感数据缓存
	lastLightLevel     int       // 最后一次光感值（lux）
	lastLightLevelTime time.Time // 最后更新时间
}

// NewSensorProxyClient 创建新的传感器代理客户端
func NewSensorProxyClient(conn *dbus.Conn) *SensorProxyClient {
	return &SensorProxyClient{
		conn:           conn,
		signalChan:     make(chan *dbus.Signal, 10),
		ownerWatcher:   make(chan *dbus.Signal, 10),
		maxRetries:     3,
		retryDelay:     time.Millisecond * 500,
		lastLightLevel: -1,
	}
}

// Connect 连接到SensorProxy服务
func (c *SensorProxyClient) Connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.conn == nil {
		return errors.New("D-Bus connection is nil")
	}
	// 创建D-Bus对象
	c.sensorProxy = c.conn.Object(hadessProxyService, hadessProxyObjectPath)
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
	// 启动信号监听
	c.startSignalWatching()
	// 启动服务监控
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
	}
	// 停止信号监听
	c.stopSignalWatching()
	// 停止服务监控
	c.stopServiceWatching()

	c.sensorProxy = nil
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
		c.mutex.Lock()
		sensorProxy := c.sensorProxy
		c.mutex.Unlock()
		c.initializeCacheFromPropertyWithProxy(sensorProxy)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.lastLightLevel < 0 {
		return 0, errors.New("no light level data available")
	}

	logger.Debugf("[AutoBrightness::GetCachedLightLevel] Returning cached raw light level: %d lux", c.lastLightLevel)
	return c.lastLightLevel, nil
}

// HasAmbientLight 检查是否有环境光传感器
func (c *SensorProxyClient) HasAmbientLight() (bool, error) {
	c.mutex.Lock()
	if !c.serviceAvailable {
		c.mutex.Unlock()
		return false, errors.New("SensorProxy service not available")
	}
	sensorProxy := c.sensorProxy
	c.mutex.Unlock()

	return c.hasAmbientLightInternalWithProxy(sensorProxy)
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
		err := c.checkServiceAvailable()
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

// checkServiceAvailable 检查服务是否可用
func (c *SensorProxyClient) checkServiceAvailable() error {
	// 尝试调用一个简单的属性获取来检查服务是否可用
	_, err := c.sensorProxy.GetProperty(hadessProxyInterface + "." + propHasAmbientLight)
	return err
}

// hasAmbientLightInternalWithProxy 内部检查环境光传感器（不加锁版本）
func (c *SensorProxyClient) hasAmbientLightInternalWithProxy(sensorProxy dbus.BusObject) (bool, error) {
	if sensorProxy == nil {
		return false, errors.New("sensor proxy is nil")
	}

	variant, err := sensorProxy.GetProperty(hadessProxyInterface + "." + propHasAmbientLight)
	if err != nil {
		return false, err
	}
	hasLight, ok := variant.Value().(bool)
	if !ok {
		return false, errors.New("invalid HasAmbientLight type")
	}
	return hasLight, nil
}

// hasAmbientLightInternal 内部检查环境光传感器
func (c *SensorProxyClient) hasAmbientLightInternal() (bool, error) {
	return c.hasAmbientLightInternalWithProxy(c.sensorProxy)
}

// claimLightInternal 内部声明环境光传感器
func (c *SensorProxyClient) claimLightInternal() error {
	call := c.sensorProxy.Call(hadessProxyInterface+"."+methodClaimLight, 0)
	return call.Err
}

// releaseLightInternal 内部释放环境光传感器
func (c *SensorProxyClient) releaseLightInternal() error {
	call := c.sensorProxy.Call(hadessProxyInterface+"."+methodReleaseLight, 0)
	return call.Err
}

// startSignalWatching 启动信号监听
func (c *SensorProxyClient) startSignalWatching() {
	// 添加属性变化信号监听，只监听特定服务的特定对象路径
	matchRule := "type='signal'," +
		"sender='" + hadessProxyService + "'," +
		"interface='org.freedesktop.DBus.Properties'," +
		"member='PropertiesChanged'," +
		"path='" + hadessProxyObjectPath + "'," +
		"arg0='" + hadessProxyInterface + "'"
	err := c.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule).Err
	if err != nil {
		logger.Warning("[SensorProxy] Failed to add PropertiesChanged signal match:", err)
		return
	}
	// 启动信号处理协程
	go c.handleSignals()
}

// stopSignalWatching 停止信号监听
func (c *SensorProxyClient) stopSignalWatching() {
	// 移除信号监听
	matchRule := "type='signal'," +
		"sender='" + hadessProxyService + "'," +
		"interface='org.freedesktop.DBus.Properties'," +
		"member='PropertiesChanged'," +
		"path='" + hadessProxyObjectPath + "'," +
		"arg0='" + hadessProxyInterface + "'"
	err := c.conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, matchRule).Err
	if err != nil {
		logger.Warning("[SensorProxy] Failed to remove PropertiesChanged signal match:", err)
	}
}

// startServiceWatching 启动服务监控
func (c *SensorProxyClient) startServiceWatching() {
	// 监听特定服务的所有者变化
	matchRule := "type='signal'," +
		"sender='org.freedesktop.DBus'," +
		"interface='org.freedesktop.DBus'," +
		"member='NameOwnerChanged'," +
		"path='/org/freedesktop/DBus'," +
		"arg0='" + hadessProxyService + "'"
	err := c.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule).Err
	if err != nil {
		logger.Warning("[SensorProxy] Failed to add NameOwnerChanged signal match:", err)
		return
	}
	// 启动服务监控协程
	go c.handleServiceChanges()
}

// stopServiceWatching 停止服务监控
func (c *SensorProxyClient) stopServiceWatching() {
	// 移除服务监听
	matchRule := "type='signal'," +
		"sender='org.freedesktop.DBus'," +
		"interface='org.freedesktop.DBus'," +
		"member='NameOwnerChanged'," +
		"path='/org/freedesktop/DBus'," +
		"arg0='" + hadessProxyService + "'"
	err := c.conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, matchRule).Err
	if err != nil {
		logger.Warning("[SensorProxy] Failed to remove NameOwnerChanged signal match:", err)
	}
}

// handleSignals 处理D-Bus信号
func (c *SensorProxyClient) handleSignals() {
	c.conn.Signal(c.signalChan)
	for signal := range c.signalChan {
		if signal.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" {
			c.handlePropertiesChanged(signal)
		}
	}
}

// handleServiceChanges 处理服务状态变化
func (c *SensorProxyClient) handleServiceChanges() {
	c.conn.Signal(c.ownerWatcher)
	for signal := range c.ownerWatcher {
		if signal.Name == "org.freedesktop.DBus.NameOwnerChanged" {
			c.handleNameOwnerChanged(signal)
		}
	}
}

// handlePropertiesChanged 处理属性变化信号
func (c *SensorProxyClient) handlePropertiesChanged(signal *dbus.Signal) {
	if len(signal.Body) < 2 {
		logger.Warning("[SensorProxy] Invalid PropertiesChanged signal body")
		return
	}

	changedProps, ok := signal.Body[1].(map[string]dbus.Variant)
	if !ok {
		logger.Warning("[SensorProxy] Invalid PropertiesChanged signal format")
		return
	}

	if lightLevelVariant, exists := changedProps[propLightLevel]; exists {
		if lightLevel, ok := lightLevelVariant.Value().(float64); ok {
			c.mutex.Lock()
			claimed := c.claimed
			c.mutex.Unlock()

			if claimed {
				c.lightValueFilter(int(lightLevel))
			}
		} else {
			logger.Warning("[SensorProxy] Failed to convert LightLevel value to float")
		}
	}
}

// handleNameOwnerChanged 处理服务所有者变化
func (c *SensorProxyClient) handleNameOwnerChanged(signal *dbus.Signal) {
	if len(signal.Body) < 3 {
		logger.Warning("[SensorProxy] Invalid NameOwnerChanged signal body")
		return
	}
	serviceName, ok := signal.Body[0].(string)
	if !ok || serviceName != hadessProxyService {
		return
	}
	newOwner, ok := signal.Body[2].(string)
	if !ok {
		return
	}
	// 服务状态变化
	serviceAvailable := newOwner != ""
	c.mutex.Lock()
	oldAvailable := c.serviceAvailable
	c.serviceAvailable = serviceAvailable
	if !serviceAvailable {
		// 服务不可用，重置状态
		c.claimed = false
		c.hasAmbientLight = false
	} else if !oldAvailable {
		// 服务重新可用，重新检查传感器
		go func() {
			time.Sleep(100 * time.Millisecond) // 等待服务完全启动
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
	// 调用服务状态变化回调
	if callback != nil {
		go callback(serviceAvailable)
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

// initializeCacheFromPropertyWithProxy 从 D-Bus 属性读取当前光照值并缓存（不加锁版本）
func (c *SensorProxyClient) initializeCacheFromPropertyWithProxy(sensorProxy dbus.BusObject) {
	if sensorProxy == nil {
		return
	}

	variant, err := sensorProxy.GetProperty(hadessProxyInterface + "." + propLightLevel)
	if err != nil {
		logger.Warning("[AutoBrightness::LightSensor] Failed to get LightLevel property:", err)
		return
	}

	lightLevel, ok := variant.Value().(float64)
	if !ok {
		logger.Warning("[AutoBrightness::LightSensor] Invalid LightLevel property type")
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
