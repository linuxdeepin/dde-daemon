// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"fmt"
	"sync"

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
	MiddleButtonEnabled dconfig.Bool `prop:"access:rw"`
	WheelEmulation      dconfig.Bool `prop:"access:rw"`
	WheelHorizScroll    dconfig.Bool `prop:"access:rw"`

	MiddleButtonTimeout   dconfig.Int64 `prop:"access:rw"`
	WheelEmulationButton  dconfig.Int64 `prop:"access:rw"`
	WheelEmulationTimeout dconfig.Int64 `prop:"access:rw"`

	MotionAcceleration dconfig.Float64 `prop:"access:rw"`
	MotionThreshold    dconfig.Float64 `prop:"access:rw"`
	MotionScaling      dconfig.Float64 `prop:"access:rw"`

	LeftHanded dconfig.Bool `prop:"access:rw"`

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
	enabled := tp.MiddleButtonEnabled.Get()
	for _, info := range tp.devInfos {
		err := info.EnableMiddleButtonEmulation(enabled)
		if err != nil {
			logger.Warningf("Enable middle button for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) enableWheelEmulation() {
	enabled := tp.WheelEmulation.Get()
	for _, info := range tp.devInfos {
		err := info.EnableWheelEmulation(enabled)
		if err != nil {
			logger.Warningf("Enable wheel emulation for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) enableWheelHorizScroll() {
	enabled := tp.WheelHorizScroll.Get()
	for _, info := range tp.devInfos {
		err := info.EnableWheelHorizScroll(enabled)
		if err != nil {
			logger.Warningf("Enable wheel horiz scroll for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) enableLeftHanded() {
	enabled := tp.LeftHanded.Get()
	for _, info := range tp.devInfos {
		err := info.EnableLeftHanded(enabled)
		if err != nil {
			logger.Warningf("Enable left-handed for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) middleButtonTimeout() {
	timeout := tp.MiddleButtonTimeout.Get()
	for _, info := range tp.devInfos {
		err := info.SetMiddleButtonEmulationTimeout(int16(timeout))
		if err != nil {
			logger.Warningf("Set middle button timeout for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) wheelEmulationButton() {
	button := tp.WheelEmulationButton.Get()
	for _, info := range tp.devInfos {
		err := info.SetWheelEmulationButton(int8(button))
		if err != nil {
			logger.Warningf("Set wheel button for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) wheelEmulationTimeout() {
	timeout := tp.WheelEmulationTimeout.Get()
	for _, info := range tp.devInfos {
		err := info.SetWheelEmulationTimeout(int16(timeout))
		if err != nil {
			logger.Warningf("Enable wheel timeout for '%v %s' failed: %v",
				info.Id, info.Name, err)
		}
	}
}

func (tp *TrackPoint) motionAcceleration() {
	accel := float32(tp.MotionAcceleration.Get())
	for _, v := range tp.devInfos {
		err := v.SetMotionAcceleration(accel)
		if err != nil {
			logger.Debugf("Set acceleration for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tp *TrackPoint) motionThreshold() {
	thres := float32(tp.MotionThreshold.Get())
	for _, v := range tp.devInfos {
		err := v.SetMotionThreshold(thres)
		if err != nil {
			logger.Debugf("Set threshold for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tp *TrackPoint) motionScaling() {
	scaling := float32(tp.MotionScaling.Get())
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

	// 绑定所有 dcprop 属性
	tp.MiddleButtonEnabled.Bind(tp.dsgTrackPointConfig, dconfigKeyTPMiddleButtonEnabled)
	tp.WheelEmulation.Bind(tp.dsgTrackPointConfig, dconfigKeyTPWheelEmulation)
	tp.WheelHorizScroll.Bind(tp.dsgTrackPointConfig, dconfigKeyTPWheelHorizScroll)
	tp.LeftHanded.Bind(tp.dsgTrackPointConfig, dconfigKeyTPLeftHanded)
	tp.MiddleButtonTimeout.Bind(tp.dsgTrackPointConfig, dconfigKeyTPMiddleButtonTimeout)
	tp.WheelEmulationButton.Bind(tp.dsgTrackPointConfig, dconfigKeyTPWheelEmulationButton)
	tp.WheelEmulationTimeout.Bind(tp.dsgTrackPointConfig, dconfigKeyTPWheelEmulationTimeout)
	tp.MotionAcceleration.Bind(tp.dsgTrackPointConfig, dconfigKeyTPMotionAcceleration)
	tp.MotionThreshold.Bind(tp.dsgTrackPointConfig, dconfigKeyTPMotionThreshold)
	tp.MotionScaling.Bind(tp.dsgTrackPointConfig, dconfigKeyTPMotionScaling)

	tp.dsgTrackPointConfig.ConnectValueChanged(func(key string) {
		logger.Debugf("TrackPoint dconfig value changed: %s", key)
		switch key {
		case dconfigKeyTPMiddleButtonEnabled:
			tp.enableMiddleButton()
		case dconfigKeyTPMiddleButtonTimeout:
			tp.middleButtonTimeout()
		case dconfigKeyTPWheelEmulation:
			tp.enableWheelEmulation()
		case dconfigKeyTPWheelEmulationButton:
			tp.wheelEmulationButton()
		case dconfigKeyTPWheelEmulationTimeout:
			tp.wheelEmulationTimeout()
		case dconfigKeyTPWheelHorizScroll:
			tp.enableWheelHorizScroll()
		case dconfigKeyTPMotionAcceleration:
			tp.motionAcceleration()
		case dconfigKeyTPMotionThreshold:
			tp.motionThreshold()
		case dconfigKeyTPMotionScaling:
			tp.motionScaling()
		case dconfigKeyTPLeftHanded:
			tp.enableLeftHanded()
		default:
			logger.Debugf("Unhandled trackpoint dconfig key change: %s", key)
		}
	})

	logger.Info("TrackPoint DConfig initialization completed successfully")
	return nil
}
