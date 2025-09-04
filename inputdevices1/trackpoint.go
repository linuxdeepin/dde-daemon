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
	dsettingsTrackPointName = "org.deepin.dde.daemon.trackpoint"

	// DConfig键值常量 - 对应trackpoint.json中的配置项
	dconfigKeyTPMiddleButtonEnabled   = "middleButtonEnabled"
	dconfigKeyTPMiddleButtonTimeout   = "middleButtonTimeout"
	dconfigKeyTPWheelEmulation        = "wheelEmulation"
	dconfigKeyTPWheelEmulationButton  = "wheelEmulationButton"
	dconfigKeyTPWheelEmulationTimeout = "wheelEmulationTimeout"
	dconfigKeyTPWheelHorizScroll      = "wheelHorizScroll"
	dconfigKeyTPMotionAcceleration    = "motionAcceleration"
	dconfigKeyTPMotionThreshold       = "motionThreshold"
	dconfigKeyTPMotionScaling         = "motionScaling"
	dconfigKeyTPLeftHanded            = "leftHanded"
)

type TrackPoint struct {
	service    *dbusutil.Service
	PropsMu    sync.RWMutex
	DeviceList string
	Exist      bool

	// dbusutil-gen: ignore-below
	MiddleButtonEnabled bool `prop:"access:rw"`
	WheelEmulation      bool `prop:"access:rw"`
	WheelHorizScroll    bool `prop:"access:rw"`

	MiddleButtonTimeout   int64 `prop:"access:rw"`
	WheelEmulationButton  int64 `prop:"access:rw"`
	WheelEmulationTimeout int64 `prop:"access:rw"`

	MotionAcceleration float64 `prop:"access:rw"`
	MotionThreshold    float64 `prop:"access:rw"`
	MotionScaling      float64 `prop:"access:rw"`

	LeftHanded bool `prop:"access:rw"`

	devInfos            Mouses
	dsgTrackPointConfig *dconfig.DConfig
	sessionSigLoop      *dbusutil.SignalLoop
}

func newTrackPoint(service *dbusutil.Service) *TrackPoint {
	var tp = new(TrackPoint)

	tp.service = service

	if err := tp.initTrackPointDConfig(); err != nil {
		logger.Errorf("Failed to initialize trackpoint dconfig: %v", err)
		panic("TrackPoint DConfig initialization failed - cannot continue without dconfig support")
	}

	// TODO: treeland环境暂不支持
	if hasTreeLand {
		return tp
	}
	tp.updateDXMouses()

	return tp
}

func (tp *TrackPoint) init() {
	if !tp.Exist {
		return
	}

	tp.enableMiddleButton()
	tp.enableWheelEmulation()
	tp.enableWheelHorizScroll()
	tp.enableLeftHanded()
	tp.middleButtonTimeout()
	tp.wheelEmulationButton()
	tp.wheelEmulationTimeout()
	tp.motionAcceleration()
	tp.motionThreshold()
	tp.motionScaling()
}

func (tp *TrackPoint) updateDXMouses() {
	tp.devInfos = Mouses{}
	for _, info := range getMouseInfos(false) {
		if !info.TrackPoint {
			continue
		}

		tmp := tp.devInfos.get(info.Id)
		if tmp != nil {
			continue
		}
		tp.devInfos = append(tp.devInfos, info)
	}

	tp.PropsMu.Lock()
	var v string
	if len(tp.devInfos) == 0 {
		tp.setPropExist(false)
	} else {
		tp.setPropExist(true)
		v = tp.devInfos.string()
	}
	tp.setPropDeviceList(v)
	tp.PropsMu.Unlock()
}

func (tp *TrackPoint) enableMiddleButton() {
	enabled := tp.MiddleButtonEnabled
	for _, info := range tp.devInfos {
		err := info.EnableMiddleButtonEmulation(enabled)
		if err != nil {
			logger.Warningf("Enable middle button for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) enableWheelEmulation() {
	enabled := tp.WheelEmulation
	for _, info := range tp.devInfos {
		err := info.EnableWheelEmulation(enabled)
		if err != nil {
			logger.Warningf("Enable wheel emulation for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) enableWheelHorizScroll() {
	enabled := tp.WheelHorizScroll
	for _, info := range tp.devInfos {
		err := info.EnableWheelHorizScroll(enabled)
		if err != nil {
			logger.Warningf("Enable wheel horiz scroll for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) enableLeftHanded() {
	enabled := tp.LeftHanded
	for _, info := range tp.devInfos {
		err := info.EnableLeftHanded(enabled)
		if err != nil {
			logger.Warningf("Enable left-handed for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) middleButtonTimeout() {
	timeout := tp.MiddleButtonTimeout
	for _, info := range tp.devInfos {
		err := info.SetMiddleButtonEmulationTimeout(int16(timeout))
		if err != nil {
			logger.Warningf("Set middle button timeout for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) wheelEmulationButton() {
	button := tp.WheelEmulationButton
	for _, info := range tp.devInfos {
		err := info.SetWheelEmulationButton(int8(button))
		if err != nil {
			logger.Warningf("Set wheel button for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) wheelEmulationTimeout() {
	timeout := tp.WheelEmulationTimeout
	for _, info := range tp.devInfos {
		err := info.SetWheelEmulationTimeout(int16(timeout))
		if err != nil {
			logger.Warningf("Enable wheel timeout for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) motionAcceleration() {
	accel := float32(tp.MotionAcceleration)
	for _, v := range tp.devInfos {
		err := v.SetMotionAcceleration(accel)
		if err != nil {
			logger.Debugf("Set acceleration for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tp *TrackPoint) motionThreshold() {
	thres := float32(tp.MotionThreshold)
	for _, v := range tp.devInfos {
		err := v.SetMotionThreshold(thres)
		if err != nil {
			logger.Debugf("Set threshold for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tp *TrackPoint) motionScaling() {
	scaling := float32(tp.MotionScaling)
	for _, v := range tp.devInfos {
		err := v.SetMotionScaling(scaling)
		if err != nil {
			logger.Debugf("Set scaling for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tp *TrackPoint) initTrackPointDConfig() error {
	var err error
	tp.dsgTrackPointConfig, err = dconfig.NewDConfig(dsettingsAppID, dsettingsTrackPointName, "")
	if err != nil {
		return fmt.Errorf("create trackpoint config manager failed: %v", err)
	}

	// 从dconfig初始化所有属性，使用默认值如果读取失败
	if err := tp.initTrackPointPropsFromDConfig(); err != nil {
		logger.Warningf("Failed to initialize trackpoint properties from dconfig, using defaults: %v", err)
	}

	tp.dsgTrackPointConfig.ConnectValueChanged(func(key string) {
		logger.Debugf("TrackPoint dconfig value changed: %s", key)
		switch key {
		case dconfigKeyTPMiddleButtonEnabled:
			tp.updateMiddleButtonEnabledFromDConfig()
		case dconfigKeyTPMiddleButtonTimeout:
			tp.updateMiddleButtonTimeoutFromDConfig()
		case dconfigKeyTPWheelEmulation:
			tp.updateWheelEmulationFromDConfig()
		case dconfigKeyTPWheelEmulationButton:
			tp.updateWheelEmulationButtonFromDConfig()
		case dconfigKeyTPWheelEmulationTimeout:
			tp.updateWheelEmulationTimeoutFromDConfig()
		case dconfigKeyTPWheelHorizScroll:
			tp.updateWheelHorizScrollFromDConfig()
		case dconfigKeyTPMotionAcceleration:
			tp.updateMotionAccelerationFromDConfig()
		case dconfigKeyTPMotionThreshold:
			tp.updateMotionThresholdFromDConfig()
		case dconfigKeyTPMotionScaling:
			tp.updateMotionScalingFromDConfig()
		case dconfigKeyTPLeftHanded:
			tp.updateLeftHandedFromDConfig()
		default:
			logger.Debugf("Unhandled trackpoint dconfig key change: %s", key)
		}
	})

	logger.Info("TrackPoint DConfig initialization completed successfully")
	return nil
}

// initTrackPointPropsFromDConfig 从dconfig初始化trackpoint属性
func (tp *TrackPoint) initTrackPointPropsFromDConfig() error {
	var err error
	// MiddleButtonEnabled
	tp.MiddleButtonEnabled, err = tp.dsgTrackPointConfig.GetValueBool(dconfigKeyTPMiddleButtonEnabled)
	if err != nil {
		logger.Warning(err)
	}

	// MiddleButtonTimeout
	tp.MiddleButtonTimeout, err = tp.dsgTrackPointConfig.GetValueInt64(dconfigKeyTPMiddleButtonTimeout)
	if err != nil {
		logger.Warning(err)
	}

	// WheelEmulation
	tp.WheelEmulation, err = tp.dsgTrackPointConfig.GetValueBool(dconfigKeyTPWheelEmulation)
	if err != nil {
		logger.Warning(err)
	}

	// WheelEmulationButton
	tp.WheelEmulationButton, err = tp.dsgTrackPointConfig.GetValueInt64(dconfigKeyTPWheelEmulationButton)
	if err != nil {
		logger.Warning(err)
	}

	// WheelEmulationTimeout
	tp.WheelEmulationTimeout, err = tp.dsgTrackPointConfig.GetValueInt64(dconfigKeyTPWheelEmulationTimeout)
	if err != nil {
		logger.Warning(err)
	}

	// WheelHorizScroll
	tp.WheelHorizScroll, err = tp.dsgTrackPointConfig.GetValueBool(dconfigKeyTPWheelHorizScroll)
	if err != nil {
		logger.Warning(err)
	}

	// MotionAcceleration
	tp.MotionAcceleration, err = tp.dsgTrackPointConfig.GetValueFloat64(dconfigKeyTPMotionAcceleration)
	if err != nil {
		logger.Warning(err)
	}

	// MotionThreshold
	tp.MotionThreshold, err = tp.dsgTrackPointConfig.GetValueFloat64(dconfigKeyTPMotionThreshold)
	if err != nil {
		logger.Warning(err)
	}

	// MotionScaling
	tp.MotionScaling, err = tp.dsgTrackPointConfig.GetValueFloat64(dconfigKeyTPMotionScaling)
	if err != nil {
		logger.Warning(err)
	}

	// LeftHanded
	tp.LeftHanded, err = tp.dsgTrackPointConfig.GetValueBool(dconfigKeyTPLeftHanded)
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

// SetTrackPointWriteCallbacks 为trackpoint属性设置DBus写回调
func (tp *TrackPoint) SetTrackPointWriteCallbacks(service *dbusutil.Service) error {
	tpServerObj := service.GetServerObject(tp)
	var err error

	// MiddleButtonEnabled 写回调
	err = tpServerObj.SetWriteCallback(tp, "MiddleButtonEnabled",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MiddleButtonEnabled type: %T", write.Value))
			}
			if saveErr := tp.saveToTrackPointDConfig(dconfigKeyTPMiddleButtonEnabled, value); saveErr != nil {
				logger.Warning("Failed to save MiddleButtonEnabled to dconfig:", saveErr)
			}
			tp.enableMiddleButton()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MiddleButtonEnabled write callback:", err)
	}

	// WheelEmulation 写回调
	err = tpServerObj.SetWriteCallback(tp, "WheelEmulation",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid WheelEmulation type: %T", write.Value))
			}
			if saveErr := tp.saveToTrackPointDConfig(dconfigKeyTPWheelEmulation, value); saveErr != nil {
				logger.Warning("Failed to save WheelEmulation to dconfig:", saveErr)
			}
			tp.enableWheelEmulation()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set WheelEmulation write callback:", err)
	}

	// WheelHorizScroll 写回调
	err = tpServerObj.SetWriteCallback(tp, "WheelHorizScroll",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid WheelHorizScroll type: %T", write.Value))
			}
			if saveErr := tp.saveToTrackPointDConfig(dconfigKeyTPWheelHorizScroll, value); saveErr != nil {
				logger.Warning("Failed to save WheelHorizScroll to dconfig:", saveErr)
			}
			tp.enableWheelHorizScroll()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set WheelHorizScroll write callback:", err)
	}

	// LeftHanded 写回调
	err = tpServerObj.SetWriteCallback(tp, "LeftHanded",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid LeftHanded type: %T", write.Value))
			}
			if saveErr := tp.saveToTrackPointDConfig(dconfigKeyTPLeftHanded, value); saveErr != nil {
				logger.Warning("Failed to save LeftHanded to dconfig:", saveErr)
			}
			tp.enableLeftHanded()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set LeftHanded write callback:", err)
	}

	// MiddleButtonTimeout 写回调
	err = tpServerObj.SetWriteCallback(tp, "MiddleButtonTimeout",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MiddleButtonTimeout type: %T", write.Value))
			}
			if saveErr := tp.saveToTrackPointDConfig(dconfigKeyTPMiddleButtonTimeout, value); saveErr != nil {
				logger.Warning("Failed to save MiddleButtonTimeout to dconfig:", saveErr)
			}
			tp.middleButtonTimeout()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MiddleButtonTimeout write callback:", err)
	}

	// WheelEmulationButton 写回调
	err = tpServerObj.SetWriteCallback(tp, "WheelEmulationButton",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid WheelEmulationButton type: %T", write.Value))
			}
			if saveErr := tp.saveToTrackPointDConfig(dconfigKeyTPWheelEmulationButton, value); saveErr != nil {
				logger.Warning("Failed to save WheelEmulationButton to dconfig:", saveErr)
			}
			tp.wheelEmulationButton()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set WheelEmulationButton write callback:", err)
	}

	// WheelEmulationTimeout 写回调
	err = tpServerObj.SetWriteCallback(tp, "WheelEmulationTimeout",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid WheelEmulationTimeout type: %T", write.Value))
			}
			if saveErr := tp.saveToTrackPointDConfig(dconfigKeyTPWheelEmulationTimeout, value); saveErr != nil {
				logger.Warning("Failed to save WheelEmulationTimeout to dconfig:", saveErr)
			}
			tp.wheelEmulationTimeout()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set WheelEmulationTimeout write callback:", err)
	}

	// MotionAcceleration 写回调
	err = tpServerObj.SetWriteCallback(tp, "MotionAcceleration",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(float64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MotionAcceleration type: %T", write.Value))
			}
			if saveErr := tp.saveToTrackPointDConfig(dconfigKeyTPMotionAcceleration, value); saveErr != nil {
				logger.Warning("Failed to save MotionAcceleration to dconfig:", saveErr)
			}
			tp.motionAcceleration()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MotionAcceleration write callback:", err)
	}

	// MotionThreshold 写回调
	err = tpServerObj.SetWriteCallback(tp, "MotionThreshold",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(float64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MotionThreshold type: %T", write.Value))
			}
			if saveErr := tp.saveToTrackPointDConfig(dconfigKeyTPMotionThreshold, value); saveErr != nil {
				logger.Warning("Failed to save MotionThreshold to dconfig:", saveErr)
			}
			tp.motionThreshold()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MotionThreshold write callback:", err)
	}

	// MotionScaling 写回调
	err = tpServerObj.SetWriteCallback(tp, "MotionScaling",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(float64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MotionScaling type: %T", write.Value))
			}
			if saveErr := tp.saveToTrackPointDConfig(dconfigKeyTPMotionScaling, value); saveErr != nil {
				logger.Warning("Failed to save MotionScaling to dconfig:", saveErr)
			}
			tp.motionScaling()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MotionScaling write callback:", err)
	}

	return nil
}

// saveToTrackPointDConfig 保存配置值到trackpoint dconfig
func (tp *TrackPoint) saveToTrackPointDConfig(key string, value interface{}) error {
	if tp.dsgTrackPointConfig == nil {
		return fmt.Errorf("trackpoint dconfig not initialized")
	}

	err := tp.dsgTrackPointConfig.SetValue(key, dbus.MakeVariant(value))
	if err != nil {
		return fmt.Errorf("failed to save %s to trackpoint dconfig: %v", key, err)
	}

	logger.Debugf("Saved %s = %v to trackpoint dconfig", key, value)
	return nil
}

// 从dconfig更新属性的方法实现
func (tp *TrackPoint) updateMiddleButtonEnabledFromDConfig() {
	middleButtonEnabled, err := tp.dsgTrackPointConfig.GetValueBool(dconfigKeyTPMiddleButtonEnabled)
	if err != nil {
		logger.Warning(err)
	}
	tp.MiddleButtonEnabled = middleButtonEnabled
	tp.enableMiddleButton()
}

func (tp *TrackPoint) updateMiddleButtonTimeoutFromDConfig() {
	middleButtonTimeout, err := tp.dsgTrackPointConfig.GetValueInt64(dconfigKeyTPMiddleButtonTimeout)
	if err != nil {
		logger.Warning(err)
	}
	tp.MiddleButtonTimeout = middleButtonTimeout
	tp.middleButtonTimeout()
}

func (tp *TrackPoint) updateWheelEmulationFromDConfig() {
	wheelEmulation, err := tp.dsgTrackPointConfig.GetValueBool(dconfigKeyTPWheelEmulation)
	if err != nil {
		logger.Warning(err)
	}
	tp.WheelEmulation = wheelEmulation
	tp.enableWheelEmulation()
}

func (tp *TrackPoint) updateWheelEmulationButtonFromDConfig() {
	wheelEmulationButton, err := tp.dsgTrackPointConfig.GetValueInt64(dconfigKeyTPWheelEmulationButton)
	if err != nil {
		logger.Warning(err)
	}
	tp.WheelEmulationButton = wheelEmulationButton
	tp.wheelEmulationButton()
}

func (tp *TrackPoint) updateWheelEmulationTimeoutFromDConfig() {
	wheelEmulationTimeout, err := tp.dsgTrackPointConfig.GetValueInt64(dconfigKeyTPWheelEmulationTimeout)
	if err != nil {
		logger.Warning(err)
	}
	tp.WheelEmulationTimeout = wheelEmulationTimeout
	tp.wheelEmulationTimeout()
}

func (tp *TrackPoint) updateWheelHorizScrollFromDConfig() {
	wheelHorizScroll, err := tp.dsgTrackPointConfig.GetValueBool(dconfigKeyTPWheelHorizScroll)
	if err != nil {
		logger.Warning(err)
	}
	tp.WheelHorizScroll = wheelHorizScroll
	tp.enableWheelHorizScroll()
}

func (tp *TrackPoint) updateMotionAccelerationFromDConfig() {
	motionAcceleration, err := tp.dsgTrackPointConfig.GetValueFloat64(dconfigKeyTPMotionAcceleration)
	if err != nil {
		logger.Warning(err)
	}
	tp.MotionAcceleration = motionAcceleration
	tp.motionAcceleration()
}

func (tp *TrackPoint) updateMotionThresholdFromDConfig() {
	motionThreshold, err := tp.dsgTrackPointConfig.GetValueFloat64(dconfigKeyTPMotionThreshold)
	if err != nil {
		logger.Warning(err)
	}
	tp.MotionThreshold = motionThreshold
	tp.motionThreshold()
}

func (tp *TrackPoint) updateMotionScalingFromDConfig() {
	motionScaling, err := tp.dsgTrackPointConfig.GetValueFloat64(dconfigKeyTPMotionScaling)
	if err != nil {
		logger.Warning(err)
	}
	tp.MotionScaling = motionScaling
	tp.motionScaling()
}

func (tp *TrackPoint) updateLeftHandedFromDConfig() {
	leftHanded, err := tp.dsgTrackPointConfig.GetValueBool(dconfigKeyTPLeftHanded)
	if err != nil {
		logger.Warning(err)
	}
	tp.LeftHanded = leftHanded
	tp.enableLeftHanded()
}
