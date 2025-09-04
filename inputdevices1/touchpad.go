// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	inputdevices "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.inputdevices1"
	power "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.power1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

const (
	dsettingsTouchpadName = "org.deepin.dde.daemon.touchpad"

	dconfigKeyTouchpadEnabled       = "touchpadEnabled"
	dconfigKeyTouchpadLeftHanded    = "leftHanded"
	dconfigKeyTouchpadDisableTyping = "disableWhileTyping"
	dconfigKeyTouchpadNaturalScroll = "naturalScroll"
	dconfigKeyTouchpadEdgeScroll    = "edgeScrollEnabled"
	dconfigKeyTouchpadHorizScroll   = "horizScrollEnabled"
	dconfigKeyTouchpadVertScroll    = "vertScrollEnabled"
	dconfigKeyTouchpadAcceleration  = "motionAcceleration"
	dconfigKeyTouchpadThreshold     = "motionThreshold"
	dconfigKeyTouchpadScaling       = "motionScaling"
	dconfigKeyTouchpadTapToClick    = "tapToClick"
	dconfigKeyTouchpadDeltaScroll   = "deltaScroll"
	dconfigKeyTouchpadDisableCmd    = "disableWhileTypingCmd"
	dconfigKeyTouchpadPalmDetect    = "palmDetect"
	dconfigKeyTouchpadPalmMinWidth  = "palmMinWidth"
	dconfigKeyTouchpadPalmMinZ      = "palmMinPressure"

	dsettingsData = "ps2MouseAsTouchPadEnabled"
)

const (
	syndaemonPidFile = "/tmp/syndaemon.pid"
)

type Touchpad struct {
	service    *dbusutil.Service
	PropsMu    sync.RWMutex
	Exist      bool
	DeviceList string

	// dbusutil-gen: ignore-below
	TPadEnable      bool `prop:"access:rw"`
	LeftHanded      bool `prop:"access:rw"`
	DisableIfTyping bool `prop:"access:rw"`
	NaturalScroll   bool `prop:"access:rw"`
	EdgeScroll      bool `prop:"access:rw"`
	HorizScroll     bool `prop:"access:rw"`
	VertScroll      bool `prop:"access:rw"`
	TapClick        bool `prop:"access:rw"`
	PalmDetect      bool `prop:"access:rw"`

	MotionAcceleration float64 `prop:"access:rw"`
	MotionThreshold    float64 `prop:"access:rw"`
	MotionScaling      float64 `prop:"access:rw"`

	DoubleClick   int32 `prop:"access:rw"`
	DragThreshold int32 `prop:"access:rw"`
	DeltaScroll   int64 `prop:"access:rw"`
	PalmMinWidth  int64 `prop:"access:rw"`
	PalmMinZ      int64 `prop:"access:rw"`

	devInfos          Touchpads
	dsgTouchpadConfig *dconfig.DConfig
	dsgMouseConfig    *dconfig.DConfig

	systemConn    *dbus.Conn
	systemSigLoop *dbusutil.SignalLoop

	// 受鼠标禁用触控板影响，临时关闭触控板
	disableTemporary bool
}

func newTouchpad(service *dbusutil.Service) *Touchpad {
	var tpad = new(Touchpad)

	tpad.service = service
	tpad.disableTemporary = false

	// 初始化dconfig（必须成功）
	if err := tpad.initTouchpadDConfig(); err != nil {
		logger.Errorf("Failed to initialize touchpad dconfig: %v", err)
		panic("Touchpad DConfig initialization failed - cannot continue without dconfig support")
	}

	// TODO: treeland环境暂不支持
	if hasTreeLand {
		return tpad
	}
	tpad.updateDXTpads()

	if conn, err := dbus.SystemBus(); err != nil {
		logger.Warning(err)
	} else {
		tpad.systemConn = conn
		tpad.systemSigLoop = dbusutil.NewSignalLoop(conn, 10)
	}

	return tpad
}

func (tpad *Touchpad) getDsgPS2MouseAsTouchPadEnable() bool {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return false
	}
	ds := configManager.NewConfigManager(sysBus)

	inputdevicesPath, err := ds.AcquireManager(0, dsettingsAppID, dsettingsInputdevices, "")
	if err != nil {
		logger.Warning(err)
		return false
	}
	inputdevicesDsg, err := configManager.NewManager(sysBus, inputdevicesPath)
	if err != nil {
		logger.Warning(err)
		return false
	}
	value, err := inputdevicesDsg.Value(0, dsettingsData)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return value.Value().(bool)
}

func (tpad *Touchpad) needCheckPS2Mouse() bool {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return false
	}
	sysPower := power.NewPower(sysBus)
	hasBattery, err := sysPower.HasBattery().Get(0)
	if err != nil {
		logger.Warning(err)
		return false
	}

	logger.Info("isPS2Mouse hasBattery : ", hasBattery)
	return hasBattery && tpad.getDsgPS2MouseAsTouchPadEnable()
}

func (tpad *Touchpad) init() {
	if !tpad.Exist {
		return
	}

	if tpad.systemConn != nil {
		sysTouchPad, err := inputdevices.NewTouchpad(tpad.systemConn, "/org/deepin/dde/InputDevices1/Touchpad")
		if err != nil {
			logger.Warning(err)
		} else {
			sysTouchPad.InitSignalExt(tpad.systemSigLoop, true)
			sysTouchPad.Enable().ConnectChanged(func(hasValue bool, value bool) {
				if !hasValue {
					return
				}
				tpad.enable(value)
			})
			if enabled, err := sysTouchPad.Enable().Get(0); err != nil {
				logger.Warning(err)
			} else {
				tpad.TPadEnable = enabled
			}
		}
	}

	currentState := tpad.TPadEnable
	tpad.TPadEnable = !currentState
	tpad.enable(currentState)
	tpad.enableLeftHanded()
	tpad.enableNaturalScroll()
	tpad.enableEdgeScroll()
	tpad.enableTapToClick()
	tpad.enableTwoFingerScroll()
	tpad.motionAcceleration()
	tpad.motionThreshold()
	tpad.motionScaling()
	tpad.disableWhileTyping()
	tpad.enablePalmDetect()
	tpad.setPalmDimensions()

	if tpad.systemSigLoop != nil {
		tpad.systemSigLoop.Start()
	}
}

func (tpad *Touchpad) handleDeviceChanged() {
	tpad.updateDXTpads()
	tpad.init()
}

func (tpad *Touchpad) updateDXTpads() {
	tpad.devInfos = Touchpads{}
	for _, info := range getTPadInfos(false, tpad.needCheckPS2Mouse()) {
		if !globalWayland {
			tmp := tpad.devInfos.get(info.Id)
			if tmp != nil {
				continue
			}
		}
		tpad.devInfos = append(tpad.devInfos, info)
	}

	tpad.PropsMu.Lock()
	var v string
	if len(tpad.devInfos) == 0 {
		tpad.setPropExist(false)
	} else {
		tpad.setPropExist(true)
		v = tpad.devInfos.string()
	}
	tpad.setPropDeviceList(v)
	tpad.PropsMu.Unlock()
}

// 受鼠标禁用触控板影响，临时关闭触控板
func (tpad *Touchpad) setDisableTemporary(disable bool) {
	if disable == tpad.disableTemporary {
		return
	}
	if len(tpad.devInfos) > 0 {
		for _, v := range tpad.devInfos {
			err := v.Enable(!disable && tpad.TPadEnable)
			if err != nil {
				logger.Warningf("Enable '%v - %v' failed: %v",
					v.Id, v.Name, err)
			}
		}
	}
	tpad.disableTemporary = disable
}

func (tpad *Touchpad) enable(enabled bool) {
	if enabled == tpad.TPadEnable {
		return
	}
	if len(tpad.devInfos) > 0 {
		for _, v := range tpad.devInfos {
			err := v.Enable(!tpad.disableTemporary && enabled)
			if err != nil {
				logger.Warningf("Enable '%v - %v' failed: %v",
					v.Id, v.Name, err)
			}
		}
	}

	enableGesture(enabled)
	tpad.TPadEnable = enabled
	sysTouchPad, err := inputdevices.NewTouchpad(tpad.systemConn, "/org/deepin/dde/InputDevices1/Touchpad")
	if err == nil && sysTouchPad != nil {
		if err = sysTouchPad.SetTouchpadEnable(0, enabled); err != nil {
			logger.Warning(err)
		}
	}
}

func (tpad *Touchpad) enableLeftHanded() {
	enabled := tpad.LeftHanded
	for _, v := range tpad.devInfos {
		err := v.EnableLeftHanded(enabled)
		if err != nil {
			logger.Debugf("Enable left handed '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
	setWMTPadBoolKey(wmTPadKeyLeftHanded, enabled)
}

func (tpad *Touchpad) enableNaturalScroll() {
	enabled := tpad.NaturalScroll
	for _, v := range tpad.devInfos {
		err := v.EnableNaturalScroll(enabled)
		if err != nil {
			logger.Debugf("Enable natural scroll '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
	setWMTPadBoolKey(wmTPadKeyNaturalScroll, enabled)
}

func (tpad *Touchpad) setScrollDistance() {
	delta := tpad.DeltaScroll
	for _, v := range tpad.devInfos {
		err := v.SetScrollDistance(int32(delta), int32(delta))
		if err != nil {
			logger.Debugf("Set natural scroll distance '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) enableEdgeScroll() {
	enabled := tpad.EdgeScroll
	for _, v := range tpad.devInfos {
		err := v.EnableEdgeScroll(enabled)
		if err != nil {
			logger.Debugf("Enable edge scroll '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
	setWMTPadBoolKey(wmTPadKeyEdgeScroll, enabled)
}

func (tpad *Touchpad) enableTwoFingerScroll() {
	vert := tpad.VertScroll
	horiz := tpad.HorizScroll
	for _, v := range tpad.devInfos {
		err := v.EnableTwoFingerScroll(vert, horiz)
		if err != nil {
			logger.Debugf("Enable two-finger scroll '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) enableTapToClick() {
	enabled := tpad.TapClick
	for _, v := range tpad.devInfos {
		err := v.EnableTapToClick(enabled)
		if err != nil {
			logger.Debugf("Enable tap to click '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
	setWMTPadBoolKey(wmTPadKeyTapClick, enabled)
}

func (tpad *Touchpad) motionAcceleration() {
	accel := float32(tpad.MotionAcceleration)
	for _, v := range tpad.devInfos {
		err := v.SetMotionAcceleration(accel)
		if err != nil {
			logger.Debugf("Set acceleration for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) motionThreshold() {
	thres := float32(tpad.MotionThreshold)
	for _, v := range tpad.devInfos {
		err := v.SetMotionThreshold(thres)
		if err != nil {
			logger.Debugf("Set threshold for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) motionScaling() {
	scaling := float32(tpad.MotionScaling)
	for _, v := range tpad.devInfos {
		err := v.SetMotionScaling(scaling)
		if err != nil {
			logger.Debugf("Set scaling for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) disableWhileTyping() {
	if !tpad.Exist {
		return
	}

	var usedLibinput bool = false
	enabled := tpad.DisableIfTyping
	for _, v := range tpad.devInfos {
		err := v.EnableDisableWhileTyping(enabled)
		if err != nil {
			continue
		}
		usedLibinput = true
	}
	if usedLibinput {
		return
	}

	if enabled {
		tpad.startSyndaemon()
	} else {
		tpad.stopSyndaemon()
	}
}

func (tpad *Touchpad) startSyndaemon() {
	if isSyndaemonExist(syndaemonPidFile) {
		logger.Debug("Syndaemon has running")
		return
	}

	syncmd, err := tpad.dsgTouchpadConfig.GetValueString(dconfigKeyTouchpadDisableCmd)
	if err != nil {
		logger.Warningf("Failed to get value for %s: %v", dconfigKeyTouchpadDisableCmd, err)
		return
	}
	if syncmd == "" {
		logger.Warning("Failed to start syndaemon, because no cmd is specified")
		return
	}
	logger.Debug("[startSyndaemon] will exec:", syncmd)
	args := strings.Split(syncmd, " ")
	argsLen := len(args)
	var cmd *exec.Cmd
	if argsLen == 1 {
		// pidfile will be created only in daemon mode
		cmd = exec.Command(args[0], "-d", "-p", syndaemonPidFile)
	} else {
		list := strv.Strv(args)
		if !list.Contains("-p") {
			if !list.Contains("-d") {
				args = append(args, "-d")
			}
			args = append(args, "-p", syndaemonPidFile)
		}
		argsLen = len(args)
		cmd = exec.Command(args[0], args[1:argsLen]...)
	}
	err = cmd.Start()
	if err != nil {
		err = os.Remove(syndaemonPidFile)
		if err != nil {
			logger.Warning("Remove error:", err)
		}
		logger.Debug("[disableWhileTyping] start syndaemon failed:", err)
		return
	}

	go func() {
		_ = cmd.Wait()
	}()
}

func (tpad *Touchpad) stopSyndaemon() {
	out, err := exec.Command("killall", "syndaemon").CombinedOutput()
	if err != nil {
		logger.Warning("[stopSyndaemon] failed:", string(out), err)
	}
	err = os.Remove(syndaemonPidFile)
	if err != nil {
		logger.Warning("remove error:", err)
	}
}

func (tpad *Touchpad) enablePalmDetect() {
	enabled := tpad.PalmDetect
	for _, dev := range tpad.devInfos {
		err := dev.EnablePalmDetect(enabled)
		if err != nil {
			logger.Warning("[enablePalmDetect] failed to enable:", dev.Id, enabled, err)
		}
	}
}

func (tpad *Touchpad) setPalmDimensions() {
	width := tpad.PalmMinWidth
	z := tpad.PalmMinZ
	for _, dev := range tpad.devInfos {
		err := dev.SetPalmDimensions(int32(width), int32(z))
		if err != nil {
			logger.Warning("[setPalmDimensions] failed to set:", dev.Id, width, z, err)
		}
	}
}

func (tpad *Touchpad) destroy() {
	if tpad.systemSigLoop != nil {
		tpad.systemSigLoop.Stop()
	}
}

func isSyndaemonExist(pidFile string) bool {
	if !dutils.IsFileExist(pidFile) {
		out, err := exec.Command("pgrep", "syndaemon").CombinedOutput()
		if err != nil || len(out) < 2 {
			return false
		}
		return true
	}

	context, err := os.ReadFile(pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.ParseInt(strings.TrimSpace(string(context)), 10, 64)
	if err != nil {
		return false
	}
	var file = fmt.Sprintf("/proc/%v/cmdline", pid)
	return isProcessExist(file, "syndaemon")
}

func isProcessExist(file, name string) bool {
	context, err := os.ReadFile(file)
	if err != nil {
		return false
	}

	return strings.Contains(string(context), name)
}

func enableGesture(enabled bool) {
	s, err := dutils.CheckAndNewGSettings("com.deepin.dde.gesture")
	if err != nil {
		return
	}
	if s.GetBoolean("touch-pad-enabled") == enabled {
		return
	}

	s.SetBoolean("touch-pad-enabled", enabled)
	s.Unref()
}

func (tpad *Touchpad) initTouchpadDConfig() error {
	var err error
	tpad.dsgTouchpadConfig, err = dconfig.NewDConfig(dsettingsAppID, dsettingsTouchpadName, "")
	if err != nil {
		return fmt.Errorf("create touchpad config manager failed: %v", err)
	}

	tpad.dsgMouseConfig, err = dconfig.NewDConfig(dsettingsAppID, dsettingsMouseName, "")
	if err != nil {
		return fmt.Errorf("create mouse config manager failed: %v", err)
	}

	// 从dconfig初始化所有属性，使用默认值如果读取失败
	if err := tpad.initTouchpadPropsFromDConfig(); err != nil {
		logger.Warningf("Failed to initialize touchpad properties from dconfig, using defaults: %v", err)
	}

	tpad.dsgTouchpadConfig.ConnectValueChanged(func(key string) {
		logger.Debugf("Touchpad dconfig value changed: %s", key)
		switch key {
		case dconfigKeyTouchpadEnabled:
			tpad.updateTPadEnableFromDConfig()
		case dconfigKeyTouchpadLeftHanded:
			tpad.updateLeftHandedFromDConfig()
		case dconfigKeyTouchpadDisableTyping:
			tpad.updateDisableWhileTypingFromDConfig()
		case dconfigKeyTouchpadNaturalScroll:
			tpad.updateNaturalScrollFromDConfig()
		case dconfigKeyTouchpadEdgeScroll:
			tpad.updateEdgeScrollFromDConfig()
		case dconfigKeyTouchpadHorizScroll:
			tpad.updateHorizScrollFromDConfig()
		case dconfigKeyTouchpadVertScroll:
			tpad.updateVertScrollFromDConfig()
		case dconfigKeyTouchpadTapToClick:
			tpad.updateTapToClickFromDConfig()
		case dconfigKeyTouchpadPalmDetect:
			tpad.updatePalmDetectFromDConfig()
		case dconfigKeyTouchpadAcceleration:
			tpad.updateMotionAccelerationFromDConfig()
		case dconfigKeyTouchpadThreshold:
			tpad.updateMotionThresholdFromDConfig()
		case dconfigKeyTouchpadScaling:
			tpad.updateMotionScalingFromDConfig()
		case dconfigKeyTouchpadDeltaScroll:
			tpad.updateDeltaScrollFromDConfig()
		case dconfigKeyTouchpadPalmMinWidth:
			tpad.updatePalmMinWidthFromDConfig()
		case dconfigKeyTouchpadPalmMinZ:
			tpad.updatePalmMinZFromDConfig()
		default:
			logger.Debugf("Unhandled touchpad dconfig key change: %s", key)
		}
	})

	tpad.dsgMouseConfig.ConnectValueChanged(func(key string) {
		logger.Debugf("Mouse dconfig value changed for touchpad: %s", key)
		switch key {
		case dconfigKeyDoubleClick:
			tpad.updateDoubleClickFromDConfig()
		case dconfigKeyDragThreshold:
			tpad.updateDragThresholdFromDConfig()
		}
	})

	logger.Info("Touchpad DConfig initialization completed successfully")
	return nil
}

// initTouchpadPropsFromDConfig 从dconfig初始化touchpad属性
func (tpad *Touchpad) initTouchpadPropsFromDConfig() error {
	var err error
	// TPadEnable
	tpad.TPadEnable, err = tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadEnabled)
	if err != nil {
		logger.Warning(err)
	}

	// LeftHanded
	tpad.LeftHanded, err = tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadLeftHanded)
	if err != nil {
		logger.Warning(err)
	}

	// DisableIfTyping
	tpad.DisableIfTyping, err = tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadDisableTyping)
	if err != nil {
		logger.Warning(err)
	}

	// NaturalScroll
	tpad.NaturalScroll, err = tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadNaturalScroll)
	if err != nil {
		logger.Warning(err)
	}

	// EdgeScroll
	tpad.EdgeScroll, err = tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadEdgeScroll)
	if err != nil {
		logger.Warning(err)
	}

	// HorizScroll
	tpad.HorizScroll, err = tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadHorizScroll)
	if err != nil {
		logger.Warning(err)
	}

	// VertScroll
	tpad.VertScroll, err = tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadVertScroll)
	if err != nil {
		logger.Warning(err)
	}

	// TapClick
	tpad.TapClick, err = tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadTapToClick)
	if err != nil {
		logger.Warning(err)
	}

	// PalmDetect
	tpad.PalmDetect, err = tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadPalmDetect)
	if err != nil {
		logger.Warning(err)
	}

	// MotionAcceleration
	tpad.MotionAcceleration, err = tpad.dsgTouchpadConfig.GetValueFloat64(dconfigKeyTouchpadAcceleration)
	if err != nil {
		logger.Warning(err)
	}

	// MotionThreshold
	tpad.MotionThreshold, err = tpad.dsgTouchpadConfig.GetValueFloat64(dconfigKeyTouchpadThreshold)
	if err != nil {
		logger.Warning(err)
	}

	// MotionScaling
	tpad.MotionScaling, err = tpad.dsgTouchpadConfig.GetValueFloat64(dconfigKeyTouchpadScaling)
	if err != nil {
		logger.Warning(err)
	}

	// DeltaScroll
	tpad.DeltaScroll, err = tpad.dsgTouchpadConfig.GetValueInt64(dconfigKeyTouchpadDeltaScroll)
	if err != nil {
		logger.Warning(err)
	}

	// PalmMinWidth
	tpad.PalmMinWidth, err = tpad.dsgTouchpadConfig.GetValueInt64(dconfigKeyTouchpadPalmMinWidth)
	if err != nil {
		logger.Warning(err)
	}

	// PalmMinZ (使用PalmMinPressure键)
	tpad.PalmMinZ, err = tpad.dsgTouchpadConfig.GetValueInt64(dconfigKeyTouchpadPalmMinZ)
	if err != nil {
		logger.Warning(err)
	}

	// DoubleClick (从mouse配置读取)
	tpad.DoubleClick, err = tpad.dsgMouseConfig.GetValueInt32(dconfigKeyDoubleClick)
	if err != nil {
		logger.Warning(err)
	}

	// DragThreshold (从mouse配置读取)
	tpad.DragThreshold, err = tpad.dsgMouseConfig.GetValueInt32(dconfigKeyDragThreshold)
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

// SetTouchpadWriteCallbacks 为touchpad属性设置DBus写回调
func (tpad *Touchpad) SetTouchpadWriteCallbacks(service *dbusutil.Service) error {
	tpadServerObj := service.GetServerObject(tpad)
	var err error

	// TPadEnable 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "TPadEnable",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid TPadEnable type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadEnabled, value); saveErr != nil {
				logger.Warning("Failed to save TPadEnable to dconfig:", saveErr)
			}
			tpad.enable(value)
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set TPadEnable write callback:", err)
	}

	// LeftHanded 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "LeftHanded",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid LeftHanded type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadLeftHanded, value); saveErr != nil {
				logger.Warning("Failed to save LeftHanded to dconfig:", saveErr)
			}
			tpad.LeftHanded = value
			tpad.enableLeftHanded()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set LeftHanded write callback:", err)
	}

	// DisableIfTyping 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "DisableIfTyping",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid DisableIfTyping type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadDisableTyping, value); saveErr != nil {
				logger.Warning("Failed to save DisableIfTyping to dconfig:", saveErr)
			}
			tpad.DisableIfTyping = value
			tpad.disableWhileTyping()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set DisableIfTyping write callback:", err)
	}

	// NaturalScroll 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "NaturalScroll",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid NaturalScroll type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadNaturalScroll, value); saveErr != nil {
				logger.Warning("Failed to save NaturalScroll to dconfig:", saveErr)
			}
			tpad.NaturalScroll = value
			tpad.enableNaturalScroll()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set NaturalScroll write callback:", err)
	}

	// EdgeScroll 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "EdgeScroll",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid EdgeScroll type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadEdgeScroll, value); saveErr != nil {
				logger.Warning("Failed to save EdgeScroll to dconfig:", saveErr)
			}
			tpad.EdgeScroll = value
			tpad.enableEdgeScroll()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set EdgeScroll write callback:", err)
	}

	// HorizScroll 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "HorizScroll",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid HorizScroll type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadHorizScroll, value); saveErr != nil {
				logger.Warning("Failed to save HorizScroll to dconfig:", saveErr)
			}
			tpad.HorizScroll = value
			tpad.enableTwoFingerScroll()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set HorizScroll write callback:", err)
	}

	// VertScroll 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "VertScroll",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid VertScroll type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadVertScroll, value); saveErr != nil {
				logger.Warning("Failed to save VertScroll to dconfig:", saveErr)
			}
			tpad.VertScroll = value
			tpad.enableTwoFingerScroll()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set VertScroll write callback:", err)
	}

	// TapClick 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "TapClick",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid TapClick type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadTapToClick, value); saveErr != nil {
				logger.Warning("Failed to save TapClick to dconfig:", saveErr)
			}
			tpad.TapClick = value
			tpad.enableTapToClick()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set TapClick write callback:", err)
	}

	// PalmDetect 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "PalmDetect",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid PalmDetect type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadPalmDetect, value); saveErr != nil {
				logger.Warning("Failed to save PalmDetect to dconfig:", saveErr)
			}
			tpad.PalmDetect = value
			tpad.enablePalmDetect()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set PalmDetect write callback:", err)
	}

	// MotionAcceleration 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "MotionAcceleration",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(float64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MotionAcceleration type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadAcceleration, value); saveErr != nil {
				logger.Warning("Failed to save MotionAcceleration to dconfig:", saveErr)
			}
			tpad.MotionAcceleration = value
			tpad.motionAcceleration()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MotionAcceleration write callback:", err)
	}

	// MotionThreshold 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "MotionThreshold",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(float64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MotionThreshold type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadThreshold, value); saveErr != nil {
				logger.Warning("Failed to save MotionThreshold to dconfig:", saveErr)
			}
			tpad.MotionThreshold = value
			tpad.motionThreshold()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MotionThreshold write callback:", err)
	}

	// MotionScaling 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "MotionScaling",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(float64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid MotionScaling type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadScaling, value); saveErr != nil {
				logger.Warning("Failed to save MotionScaling to dconfig:", saveErr)
			}
			tpad.MotionScaling = value
			tpad.motionScaling()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set MotionScaling write callback:", err)
	}

	// DeltaScroll 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "DeltaScroll",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid DeltaScroll type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadDeltaScroll, value); saveErr != nil {
				logger.Warning("Failed to save DeltaScroll to dconfig:", saveErr)
			}
			tpad.DeltaScroll = value
			tpad.setScrollDistance()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set DeltaScroll write callback:", err)
	}

	// PalmMinWidth 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "PalmMinWidth",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid PalmMinWidth type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadPalmMinWidth, value); saveErr != nil {
				logger.Warning("Failed to save PalmMinWidth to dconfig:", saveErr)
			}
			tpad.PalmMinWidth = value
			tpad.setPalmDimensions()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set PalmMinWidth write callback:", err)
	}

	// PalmMinZ 写回调
	err = tpadServerObj.SetWriteCallback(tpad, "PalmMinZ",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int64)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid PalmMinZ type: %T", write.Value))
			}
			if saveErr := tpad.saveToTouchpadDConfig(dconfigKeyTouchpadPalmMinZ, value); saveErr != nil {
				logger.Warning("Failed to save PalmMinZ to dconfig:", saveErr)
			}
			tpad.PalmMinZ = value
			tpad.setPalmDimensions()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set PalmMinZ write callback:", err)
	}

	// DoubleClick 写回调 (保存到mouse配置)
	err = tpadServerObj.SetWriteCallback(tpad, "DoubleClick",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid DoubleClick type: %T", write.Value))
			}
			if saveErr := tpad.saveToMouseDConfig(dconfigKeyDoubleClick, value); saveErr != nil {
				logger.Warning("Failed to save DoubleClick to dconfig:", saveErr)
			}
			tpad.DoubleClick = value
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set DoubleClick write callback:", err)
	}

	// DragThreshold 写回调 (保存到mouse配置)
	err = tpadServerObj.SetWriteCallback(tpad, "DragThreshold",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid DragThreshold type: %T", write.Value))
			}
			if saveErr := tpad.saveToMouseDConfig(dconfigKeyDragThreshold, value); saveErr != nil {
				logger.Warning("Failed to save DragThreshold to dconfig:", saveErr)
			}
			tpad.DragThreshold = value
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set DragThreshold write callback:", err)
	}

	return nil
}

// saveToTouchpadDConfig 保存配置值到touchpad dconfig
func (tpad *Touchpad) saveToTouchpadDConfig(key string, value interface{}) error {
	if tpad.dsgTouchpadConfig == nil {
		return fmt.Errorf("touchpad dconfig not initialized")
	}

	err := tpad.dsgTouchpadConfig.SetValue(key, value)
	if err != nil {
		return fmt.Errorf("failed to save %s to touchpad dconfig: %v", key, err)
	}

	logger.Debugf("Saved %s = %v to touchpad dconfig", key, value)
	return nil
}

// saveToMouseDConfig 保存配置值到mouse dconfig
func (tpad *Touchpad) saveToMouseDConfig(key string, value interface{}) error {
	if tpad.dsgMouseConfig == nil {
		return fmt.Errorf("mouse dconfig not initialized")
	}

	err := tpad.dsgMouseConfig.SetValue(key, dbus.MakeVariant(value))
	if err != nil {
		return fmt.Errorf("failed to save %s to mouse dconfig: %v", key, err)
	}

	logger.Debugf("Saved %s = %v to mouse dconfig", key, value)
	return nil
}

// 从dconfig更新属性的方法实现（只实现几个关键的，其他可以根据需要添加）
func (tpad *Touchpad) updateTPadEnableFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadEnabled)
	if err != nil {
		logger.Warning(err)
	}
	tpad.TPadEnable = value
	tpad.enable(value)
}

func (tpad *Touchpad) updateLeftHandedFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadLeftHanded)
	if err != nil {
		logger.Warning(err)
	}
	tpad.LeftHanded = value
	tpad.enableLeftHanded()
}

func (tpad *Touchpad) updateDisableWhileTypingFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadDisableTyping)
	if err != nil {
		logger.Warning(err)
	}
	tpad.DisableIfTyping = value
	tpad.disableWhileTyping()
}

func (tpad *Touchpad) updateNaturalScrollFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadNaturalScroll)
	if err != nil {
		logger.Warning(err)
	}
	tpad.NaturalScroll = value
	tpad.enableNaturalScroll()
}

func (tpad *Touchpad) updateEdgeScrollFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadEdgeScroll)
	if err != nil {
		logger.Warning(err)
	}
	tpad.EdgeScroll = value
	tpad.enableEdgeScroll()
}

func (tpad *Touchpad) updateHorizScrollFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadHorizScroll)
	if err != nil {
		logger.Warning(err)
	}
	tpad.HorizScroll = value
	tpad.enableTwoFingerScroll()
}

func (tpad *Touchpad) updateVertScrollFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadVertScroll)
	if err != nil {
		logger.Warning(err)
	}
	tpad.VertScroll = value
	tpad.enableTwoFingerScroll()
}

func (tpad *Touchpad) updateTapToClickFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadTapToClick)
	if err != nil {
		logger.Warning(err)
	}
	tpad.TapClick = value
	tpad.enableTapToClick()
}

func (tpad *Touchpad) updatePalmDetectFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadPalmDetect)
	if err != nil {
		logger.Warning(err)
	}
	tpad.PalmDetect = value
	tpad.enablePalmDetect()
}

func (tpad *Touchpad) updateMotionAccelerationFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueFloat64(dconfigKeyTouchpadAcceleration)
	if err != nil {
		logger.Warning(err)
		return
	}
	tpad.MotionAcceleration = value
	tpad.motionAcceleration()
}

func (tpad *Touchpad) updateMotionThresholdFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueFloat64(dconfigKeyTouchpadThreshold)
	if err != nil {
		logger.Warning(err)
	}
	tpad.MotionThreshold = value
	tpad.motionThreshold()
}

func (tpad *Touchpad) updateMotionScalingFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueFloat64(dconfigKeyTouchpadScaling)
	if err != nil {
		logger.Warning(err)
	}
	tpad.MotionScaling = value
	tpad.motionScaling()
}

func (tpad *Touchpad) updateDeltaScrollFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueInt64(dconfigKeyTouchpadDeltaScroll)
	if err != nil {
		logger.Warning(err)
		return
	}
	tpad.DeltaScroll = value
	tpad.setScrollDistance()
}

func (tpad *Touchpad) updatePalmMinWidthFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueInt64(dconfigKeyTouchpadPalmMinWidth)
	if err != nil {
		logger.Warning(err)
		return
	}
	tpad.PalmMinWidth = value
	tpad.setPalmDimensions()
}

func (tpad *Touchpad) updatePalmMinZFromDConfig() {
	value, err := tpad.dsgTouchpadConfig.GetValueInt64(dconfigKeyTouchpadPalmMinZ)
	if err != nil {
		logger.Warning(err)
		return
	}
	tpad.PalmMinZ = value
	tpad.setPalmDimensions()
}

func (tpad *Touchpad) updateDoubleClickFromDConfig() {
	value, err := tpad.dsgMouseConfig.GetValueInt32(dconfigKeyDoubleClick)
	if err != nil {
		logger.Warning(err)
		return
	}
	tpad.DoubleClick = value
}

func (tpad *Touchpad) updateDragThresholdFromDConfig() {
	value, err := tpad.dsgMouseConfig.GetValueInt32(dconfigKeyDragThreshold)
	if err != nil {
		logger.Warning(err)
		return
	}
	tpad.DragThreshold = value
}
