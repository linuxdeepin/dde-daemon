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
	Enabled               bool         `prop:"access:rw"`
	LeftHanded            dconfig.Bool `prop:"access:rw"`
	DisableTpad           dconfig.Bool `prop:"access:rw"`
	NaturalScroll         dconfig.Bool `prop:"access:rw"`
	MiddleButtonEmulation dconfig.Bool `prop:"access:rw"`
	AdaptiveAccelProfile  dconfig.Bool `prop:"access:rw"`

	MotionAcceleration dconfig.Float64 `prop:"access:rw"`
	MotionThreshold    dconfig.Float64 `prop:"access:rw"`
	MotionScaling      dconfig.Float64 `prop:"access:rw"`

	DoubleClick   dconfig.Int32 `prop:"access:rw"`
	DragThreshold dconfig.Int32 `prop:"access:rw"`

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

	if err := m.initMouseDConfig(); err != nil {
		logger.Errorf("Failed to initialize mouse dconfig: %v", err)
		panic("Mouse DConfig initialization failed - cannot continue without dconfig support")
	}
	m.LeftHanded.Bind(m.dsgMouseConfig, dconfigKeyLeftHanded)
	m.DisableTpad.Bind(m.dsgMouseConfig, dconfigKeyDisableTouchpad)
	m.NaturalScroll.Bind(m.dsgMouseConfig, dconfigKeyNaturalScroll)
	m.MiddleButtonEmulation.Bind(m.dsgMouseConfig, dconfigKeyMiddleButtonEnabled)
	m.AdaptiveAccelProfile.Bind(m.dsgMouseConfig, dconfigKeyAdaptiveAccelProfile)
	m.MotionAcceleration.Bind(m.dsgMouseConfig, dconfigKeyMotionAcceleration)
	m.MotionThreshold.Bind(m.dsgMouseConfig, dconfigKeyMotionThreshold)
	m.MotionScaling.Bind(m.dsgMouseConfig, dconfigKeyMotionScaling)
	m.DoubleClick.Bind(m.dsgMouseConfig, dconfigKeyDoubleClick)
	m.DragThreshold.Bind(m.dsgMouseConfig, dconfigKeyDragThreshold)

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
		if tpad.Exist && tpad.TPadEnable.Get() {
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
	if m.DisableTpad.Get() && tpad.TPadEnable.Get() {
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

	if !m.DisableTpad.Get() {
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
	enabled := m.LeftHanded.Get()
	for _, v := range m.devInfos {
		err := v.EnableLeftHanded(enabled)
		if err != nil {
			logger.Debugf("Enable left handed for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (m *Mouse) enableNaturalScroll() {
	enabled := m.NaturalScroll.Get()
	for _, v := range m.devInfos {
		err := v.EnableNaturalScroll(enabled)
		if err != nil {
			logger.Debugf("Enable natural scroll for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (m *Mouse) enableMidBtnEmu() {
	enabled := m.MiddleButtonEmulation.Get()
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
	enabled := m.AdaptiveAccelProfile.Get()
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
	accel := float32(m.MotionAcceleration.Get())
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
	thres := float32(m.MotionThreshold.Get())
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
	scaling := float32(m.MotionScaling.Get())
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
	xsSetInt32(xsPropDoubleClick, m.DoubleClick.Get())
}

func (m *Mouse) dragThreshold() {
	xsSetInt32(xsPropDragThres, m.DragThreshold.Get())
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

	m.dsgMouseConfig.ConnectValueChanged(func(key string) {
		logger.Debugf("Mouse dconfig value changed: %s", key)
		switch key {
		case dconfigKeyLeftHanded:
			m.enableLeftHanded()
		case dconfigKeyDisableTouchpad:
			m.disableTouchPad()
		case dconfigKeyNaturalScroll:
			m.enableNaturalScroll()
		case dconfigKeyMiddleButtonEnabled:
			m.enableMidBtnEmu()
		case dconfigKeyAdaptiveAccelProfile:
			m.enableAdaptiveAccelProfile()
		case dconfigKeyMotionAcceleration:
			m.motionAcceleration()
		case dconfigKeyMotionThreshold:
			m.motionThreshold()
		case dconfigKeyMotionScaling:
			m.motionScaling()
		case dconfigKeyDoubleClick:
			m.doubleClick()
		case dconfigKeyDragThreshold:
			m.dragThreshold()
		default:
			logger.Debugf("Unhandled mouse dconfig key change: %s", key)
		}
	})

	logger.Info("Mouse DConfig initialization completed successfully")
	return nil
}
