// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	// DConfig相关常量
	dsettingsMouseName = "org.deepin.dde.daemon.mouse"

	// DConfig键值常量 - 对应mouse.json中的配置项
	dconfigKeyLeftHanded           = "leftHanded"
	dconfigKeyDisableTouchpad      = "disableTouchpad"
	dconfigKeyMiddleButtonEnabled  = "middleButtonEnabled"
	dconfigKeyNaturalScroll        = "naturalScroll"
	dconfigKeyMotionAcceleration   = "motionAcceleration"
	dconfigKeyMotionThreshold      = "motionThreshold"
	dconfigKeyMotionScaling        = "motionScaling"
	dconfigKeyDoubleClick          = "doubleClick"
	dconfigKeyDragThreshold        = "dragThreshold"
	dconfigKeyAdaptiveAccelProfile = "adaptiveAccelProfile"
	dconfigKeyLocatePointer        = "locatePointer"
)

type Mouse struct {
	service    *dbusutil.Service
	PropsMu    sync.RWMutex
	DeviceList string
	Exist      bool

	// dbusutil-gen: ignore-below
	Enabled               bool `prop:"access:rw"`
	LeftHanded            bool `prop:"access:rw"`
	DisableTpad           bool `prop:"access:rw"`
	NaturalScroll         bool `prop:"access:rw"`
	MiddleButtonEmulation bool `prop:"access:rw"`
	AdaptiveAccelProfile  bool `prop:"access:rw"`

	MotionAcceleration float64 `prop:"access:rw"`
	MotionThreshold    float64 `prop:"access:rw"`
	MotionScaling      float64 `prop:"access:rw"`

	DoubleClick   int32 `prop:"access:rw"`
	DragThreshold int32 `prop:"access:rw"`

	devInfos       Mouses
	dsgMouseConfig *dconfig.DConfig
	sessionSigLoop *dbusutil.SignalLoop
	touchPad       *Touchpad
}

func newMouse(service *dbusutil.Service, touchPad *Touchpad, sessionSigLoop *dbusutil.SignalLoop) *Mouse {
	var m = new(Mouse)

	m.service = service
	m.touchPad = touchPad
	m.sessionSigLoop = sessionSigLoop

	// 初始化dconfig（必须成功）
	if err := m.initMouseDConfig(); err != nil {
		logger.Errorf("Failed to initialize mouse dconfig: %v", err)
		panic("Mouse DConfig initialization failed - cannot continue without dconfig support")
	}

	// TODO: treeland环境暂不支持
	if hasTreeLand {
		return m
	}
	m.updateDXMouses()

	return m
}

func (m *Mouse) init() {
	//触摸板和鼠标都是用mouse的doubleClick，检测到只有触摸板时也同步到xsettings
	m.syncConfigToXsettings()

	tpad := m.touchPad

	if !m.Exist {
		if tpad.Exist && tpad.TPadEnable {
			tpad.setDisableTemporary(false)
		}
		return
	}

	m.enableLeftHanded()
	m.enableMidBtnEmu()
	m.enableNaturalScroll()
	m.enableAdaptiveAccelProfile()
	m.motionAcceleration()
	m.motionThreshold()
	if m.DisableTpad && tpad.TPadEnable {
		m.disableTouchPad()
	}
}

func (m *Mouse) handleDeviceChanged() {
	m.updateDXMouses()
	m.init()
}

func (m *Mouse) updateDXMouses() {
	m.devInfos = Mouses{}
	// 在有鼠标连接下，len(_mouseInfos)不等于0，传参false，
	// 切换用户后，handleDeviceChanged，getMouseInfos直接返回，未进行忽略触摸板设备的判断
	// 导致识别到2个鼠标，拔掉鼠标后，还剩一个，在设置‘插入鼠标时禁用触控板’后，触摸板无法使用
	for _, info := range getMouseInfos(true) {
		if info.TrackPoint {
			continue
		}

		if info.IsEnabled() {
			m.Enabled = true
		}

		if !globalWayland {
			tmp := m.devInfos.get(info.Id)
			if tmp != nil {
				continue
			}
		}
		m.devInfos = append(m.devInfos, info)
	}

	m.PropsMu.Lock()
	var v string
	if len(m.devInfos) == 0 {
		m.setPropExist(false)
	} else {
		m.setPropExist(true)
		v = m.devInfos.string()
	}
	m.setPropDeviceList(v)
	m.PropsMu.Unlock()
}

func (m *Mouse) disableTouchPad() {
	m.PropsMu.RLock()
	mouseExist := m.Exist
	m.PropsMu.RUnlock()
	if !mouseExist {
		return
	}

	touchPad := m.touchPad
	touchPad.PropsMu.RLock()
	touchPadExist := touchPad.Exist
	touchPad.PropsMu.RUnlock()
	if !touchPadExist {
		return
	}

	if !m.DisableTpad {
		touchPad.setDisableTemporary(false)
		return
	}

	touchPad.setDisableTemporary(true)
}

func (m *Mouse) enable(enabled bool) error {
	for _, v := range m.devInfos {
		err := v.Enable(enabled)
		if err != nil {
			logger.Debugf("Enable left handed for '%d - %v' failed: %v",
				v.Id, v.Name, err)
			return err
		}
	}

	m.Enabled = enabled

	return nil
}

func (m *Mouse) enableLeftHanded() {
	enabled := m.LeftHanded
	for _, v := range m.devInfos {
		err := v.EnableLeftHanded(enabled)
		if err != nil {
			logger.Debugf("Enable left handed for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (m *Mouse) enableNaturalScroll() {
	enabled := m.NaturalScroll
	for _, v := range m.devInfos {
		err := v.EnableNaturalScroll(enabled)
		if err != nil {
			logger.Debugf("Enable natural scroll for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (m *Mouse) enableMidBtnEmu() {
	enabled := m.MiddleButtonEmulation
	for _, v := range m.devInfos {
		if v.TrackPoint {
			continue
		}

		err := v.EnableMiddleButtonEmulation(enabled)
		if err != nil {
			logger.Debugf("Enable mid btn emulation for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (m *Mouse) enableAdaptiveAccelProfile() {
	enabled := m.AdaptiveAccelProfile
	for _, v := range m.devInfos {
		if !v.CanChangeAccelProfile() {
			continue
		}

		err := v.SetUseAdaptiveAccelProfile(enabled)
		if err != nil {
			logger.Debugf("Enable adaptive accel profile for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (m *Mouse) motionAcceleration() {
	accel := float32(m.MotionAcceleration)
	for _, v := range m.devInfos {
		if v.TrackPoint {
			continue
		}

		err := v.SetMotionAcceleration(accel)
		if err != nil {
			logger.Debugf("Set acceleration for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (m *Mouse) motionThreshold() {
	thres := float32(m.MotionThreshold)
	for _, v := range m.devInfos {
		if v.TrackPoint {
			continue
		}

		err := v.SetMotionThreshold(thres)
		if err != nil {
			logger.Debugf("Set threshold for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (m *Mouse) motionScaling() {
	scaling := float32(m.MotionScaling)
	for _, v := range m.devInfos {
		if v.TrackPoint {
			continue
		}

		err := v.SetMotionScaling(scaling)
		if err != nil {
			logger.Debugf("Set scaling for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (m *Mouse) doubleClick() {
	xsSetInt32(xsPropDoubleClick, int32(m.DoubleClick))
}

func (m *Mouse) dragThreshold() {
	xsSetInt32(xsPropDragThres, int32(m.DragThreshold))
}

func (m *Mouse) syncConfigToXsettings() {
	m.doubleClick() // 初始化时,将默认配置同步到xsettings中
}

func (m *Mouse) initMouseDConfig() error {
	var err error
	m.dsgMouseConfig, err = dconfig.NewDConfig(dsettingsAppID, dsettingsMouseName, "")
	if err != nil {
		return fmt.Errorf("create mouse config manager failed: %v", err)
	}

	// 从dconfig初始化所有属性，使用默认值如果读取失败
	if err := m.initMousePropsFromDConfig(); err != nil {
		logger.Warningf("Failed to initialize mouse properties from dconfig, using defaults: %v", err)
	}

	m.dsgMouseConfig.ConnectValueChanged(func(key string) {
		logger.Debugf("Mouse dconfig value changed: %s", key)
		switch key {
		case dconfigKeyLeftHanded:
			m.updateLeftHandedFromDConfig()
		case dconfigKeyDisableTouchpad:
			m.updateDisableTpadFromDConfig()
		case dconfigKeyNaturalScroll:
			m.updateNaturalScrollFromDConfig()
		case dconfigKeyMiddleButtonEnabled:
			m.updateMiddleButtonFromDConfig()
		case dconfigKeyAdaptiveAccelProfile:
			m.updateAdaptiveAccelFromDConfig()
		case dconfigKeyMotionAcceleration:
			m.updateMotionAccelerationFromDConfig()
		case dconfigKeyMotionThreshold:
			m.updateMotionThresholdFromDConfig()
		case dconfigKeyMotionScaling:
			m.updateMotionScalingFromDConfig()
		case dconfigKeyDoubleClick:
			m.updateDoubleClickFromDConfig()
		case dconfigKeyDragThreshold:
			m.updateDragThresholdFromDConfig()
		default:
			logger.Debugf("Unhandled mouse dconfig key change: %s", key)
		}
	})

	logger.Info("Mouse DConfig initialization completed successfully")
	return nil
}

// initMousePropsFromDConfig 从dconfig初始化鼠标属性
func (m *Mouse) initMousePropsFromDConfig() error {
	var err error
	// LeftHanded
	m.LeftHanded, err = m.dsgMouseConfig.GetValueBool(dconfigKeyLeftHanded)
	if err != nil {
		logger.Warning(err)
		return err
	}

	// DisableTpad
	m.DisableTpad, err = m.dsgMouseConfig.GetValueBool(dconfigKeyDisableTouchpad)
	if err != nil {
		logger.Warning(err)
		return err
	}

	// NaturalScroll
	m.NaturalScroll, err = m.dsgMouseConfig.GetValueBool(dconfigKeyNaturalScroll)
	if err != nil {
		logger.Warning(err)
	}

	// MiddleButtonEmulation
	m.MiddleButtonEmulation, err = m.dsgMouseConfig.GetValueBool(dconfigKeyMiddleButtonEnabled)
	if err != nil {
		logger.Warning(err)
	}

	// AdaptiveAccelProfile
	m.AdaptiveAccelProfile, err = m.dsgMouseConfig.GetValueBool(dconfigKeyAdaptiveAccelProfile)
	if err != nil {
		logger.Warning(err)
	}

	// MotionAcceleration
	m.MotionAcceleration, err = m.dsgMouseConfig.GetValueFloat64(dconfigKeyMotionAcceleration)
	if err != nil {
		logger.Warning(err)
		return err
	}

	// MotionThreshold
	m.MotionThreshold, err = m.dsgMouseConfig.GetValueFloat64(dconfigKeyMotionThreshold)
	if err != nil {
		logger.Warning(err)
	}

	// MotionScaling
	m.MotionScaling, err = m.dsgMouseConfig.GetValueFloat64(dconfigKeyMotionScaling)
	if err != nil {
		logger.Warning(err)
	}

	// DoubleClick
	m.DoubleClick, err = m.dsgMouseConfig.GetValueInt32(dconfigKeyDoubleClick)
	if err != nil {
		logger.Warning(err)
	}

	// DragThreshold
	m.DragThreshold, err = m.dsgMouseConfig.GetValueInt32(dconfigKeyDragThreshold)
	if err != nil {
		logger.Warning(err)
	}

	// MotionScaling
	m.MotionScaling, err = m.dsgMouseConfig.GetValueFloat64(dconfigKeyMotionScaling)
	if err != nil {
		logger.Warning(err)
	}

	// DoubleClick
	m.DoubleClick, err = m.dsgMouseConfig.GetValueInt32(dconfigKeyDoubleClick)
	if err != nil {
		logger.Warning(err)
	}

	// DragThreshold
	m.DragThreshold, err = m.dsgMouseConfig.GetValueInt32(dconfigKeyDragThreshold)
	if err != nil {
		logger.Warning(err)
	}

	return nil
}

// SetMouseWriteCallbacks 为鼠标属性设置DBus写回调
func (m *Mouse) SetMouseWriteCallbacks(service *dbusutil.Service) error {
	mouseServerObj := service.GetServerObject(m)
	var err error

	// LeftHanded 写回调
	err = mouseServerObj.SetWriteCallback(m, "LeftHanded",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid LeftHanded type: %T", write.Value))
			}
			if saveErr := m.saveToMouseDConfig(dconfigKeyLeftHanded, value); saveErr != nil {
				logger.Warning("Failed to save LeftHanded to dconfig:", saveErr)
			}
			m.enableLeftHanded()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set LeftHanded write callback:", err)
	}

	// DisableTpad 写回调
	err = mouseServerObj.SetWriteCallback(m, "DisableTpad",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid DisableTpad type: %T", write.Value))
			}
			if saveErr := m.saveToMouseDConfig(dconfigKeyDisableTouchpad, value); saveErr != nil {
				logger.Warning("Failed to save DisableTpad to dconfig:", saveErr)
			}
			m.disableTouchPad()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set DisableTpad write callback:", err)
	}

	// NaturalScroll 写回调
	err = mouseServerObj.SetWriteCallback(m, "NaturalScroll",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid NaturalScroll type: %T", write.Value))
			}
			if saveErr := m.saveToMouseDConfig(dconfigKeyNaturalScroll, value); saveErr != nil {
				logger.Warning("Failed to save NaturalScroll to dconfig:", saveErr)
			}
			m.enableNaturalScroll()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set NaturalScroll write callback:", err)
	}

	// MiddleButtonEmulation 写回调
	err = mouseServerObj.SetWriteCallback(m, "MiddleButtonEmulation",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MiddleButtonEmulation type: %T", write.Value))
			}
			if saveErr := m.saveToMouseDConfig(dconfigKeyMiddleButtonEnabled, value); saveErr != nil {
				logger.Warning("Failed to save MiddleButtonEmulation to dconfig:", saveErr)
			}
			m.enableMidBtnEmu()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MiddleButtonEmulation write callback:", err)
	}

	// AdaptiveAccelProfile 写回调
	err = mouseServerObj.SetWriteCallback(m, "AdaptiveAccelProfile",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid AdaptiveAccelProfile type: %T", write.Value))
			}
			if saveErr := m.saveToMouseDConfig(dconfigKeyAdaptiveAccelProfile, value); saveErr != nil {
				logger.Warning("Failed to save AdaptiveAccelProfile to dconfig:", saveErr)
			}
			m.enableAdaptiveAccelProfile()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set AdaptiveAccelProfile write callback:", err)
	}

	// MotionAcceleration 写回调
	err = mouseServerObj.SetWriteCallback(m, "MotionAcceleration",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(float64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MotionAcceleration type: %T", write.Value))
			}
			if saveErr := m.saveToMouseDConfig(dconfigKeyMotionAcceleration, value); saveErr != nil {
				logger.Warning("Failed to save MotionAcceleration to dconfig:", saveErr)
			}
			m.motionAcceleration()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MotionAcceleration write callback:", err)
	}

	// MotionThreshold 写回调
	err = mouseServerObj.SetWriteCallback(m, "MotionThreshold",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(float64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MotionThreshold type: %T", write.Value))
			}
			if saveErr := m.saveToMouseDConfig(dconfigKeyMotionThreshold, value); saveErr != nil {
				logger.Warning("Failed to save MotionThreshold to dconfig:", saveErr)
			}
			m.motionThreshold()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MotionThreshold write callback:", err)
	}

	// MotionScaling 写回调
	err = mouseServerObj.SetWriteCallback(m, "MotionScaling",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(float64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MotionScaling type: %T", write.Value))
			}
			if saveErr := m.saveToMouseDConfig(dconfigKeyMotionScaling, value); saveErr != nil {
				logger.Warning("Failed to save MotionScaling to dconfig:", saveErr)
			}
			m.motionScaling()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MotionScaling write callback:", err)
	}

	// DoubleClick 写回调
	err = mouseServerObj.SetWriteCallback(m, "DoubleClick",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid DoubleClick type: %T", write.Value))
			}
			if saveErr := m.saveToMouseDConfig(dconfigKeyDoubleClick, value); saveErr != nil {
				logger.Warning("Failed to save DoubleClick to dconfig:", saveErr)
			}
			m.doubleClick()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set DoubleClick write callback:", err)
	}

	// DragThreshold 写回调
	err = mouseServerObj.SetWriteCallback(m, "DragThreshold",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid DragThreshold type: %T", write.Value))
			}
			if saveErr := m.saveToMouseDConfig(dconfigKeyDragThreshold, value); saveErr != nil {
				logger.Warning("Failed to save DragThreshold to dconfig:", saveErr)
			}
			m.dragThreshold()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set DragThreshold write callback:", err)
	}

	return nil
}

// saveToMouseDConfig 保存配置值到mouse dconfig
func (m *Mouse) saveToMouseDConfig(key string, value interface{}) error {
	if m.dsgMouseConfig == nil {
		return fmt.Errorf("mouse dconfig not initialized")
	}

	err := m.dsgMouseConfig.SetValue(key, value)
	if err != nil {
		return fmt.Errorf("failed to save %s to mouse dconfig: %v", key, err)
	}

	logger.Debugf("Saved %s = %v to mouse dconfig", key, value)
	return nil
}

// 从dconfig更新属性的方法实现
func (m *Mouse) updateLeftHandedFromDConfig() {
	value, err := m.dsgMouseConfig.GetValueBool(dconfigKeyLeftHanded)
	if err != nil {
		logger.Warning(err)
	}
	m.LeftHanded = value
	m.enableLeftHanded()
}

func (m *Mouse) updateDisableTpadFromDConfig() {
	value, err := m.dsgMouseConfig.GetValueBool(dconfigKeyDisableTouchpad)
	if err != nil {
		logger.Warning(err)
	}
	m.DisableTpad = value
	m.disableTouchPad()
}

func (m *Mouse) updateNaturalScrollFromDConfig() {
	value, err := m.dsgMouseConfig.GetValueBool(dconfigKeyNaturalScroll)
	if err != nil {
		logger.Warning(err)
	}
	m.NaturalScroll = value
	m.enableNaturalScroll()
}

func (m *Mouse) updateMiddleButtonFromDConfig() {
	value, err := m.dsgMouseConfig.GetValueBool(dconfigKeyMiddleButtonEnabled)
	if err != nil {
		logger.Warning(err)
	}
	m.MiddleButtonEmulation = value
	m.enableMidBtnEmu()
}

func (m *Mouse) updateAdaptiveAccelFromDConfig() {
	value, err := m.dsgMouseConfig.GetValueBool(dconfigKeyAdaptiveAccelProfile)
	if err != nil {
		logger.Warning(err)
	}
	m.AdaptiveAccelProfile = value
	m.enableAdaptiveAccelProfile()
}

func (m *Mouse) updateMotionAccelerationFromDConfig() {
	value, err := m.dsgMouseConfig.GetValueFloat64(dconfigKeyMotionAcceleration)
	if err != nil {
		logger.Warning(err)
	}
	m.MotionAcceleration = value
	m.motionAcceleration()
}

func (m *Mouse) updateMotionThresholdFromDConfig() {
	value, err := m.dsgMouseConfig.GetValueFloat64(dconfigKeyMotionThreshold)
	if err != nil {
		logger.Warning(err)
	}
	m.MotionThreshold = value
	m.motionThreshold()
}

func (m *Mouse) updateMotionScalingFromDConfig() {
	value, err := m.dsgMouseConfig.GetValueFloat64(dconfigKeyMotionScaling)
	if err != nil {
		logger.Warning(err)
	}
	m.MotionScaling = value
	m.motionScaling()
}

func (m *Mouse) updateDoubleClickFromDConfig() {
	value, err := m.dsgMouseConfig.GetValueInt32(dconfigKeyDoubleClick)
	if err != nil {
		logger.Warning(err)
	}
	m.DoubleClick = value
	m.doubleClick()
}

func (m *Mouse) updateDragThresholdFromDConfig() {
	value, err := m.dsgMouseConfig.GetValueInt32(dconfigKeyDragThreshold)
	if err != nil {
		logger.Warning(err)
	}
	m.DragThreshold = value
	m.dragThreshold()
}
