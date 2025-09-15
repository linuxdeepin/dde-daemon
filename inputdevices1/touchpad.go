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
	TPadEnable      dconfig.Bool `prop:"access:rw"`
	LeftHanded      dconfig.Bool `prop:"access:rw"`
	DisableIfTyping dconfig.Bool `prop:"access:rw"`
	NaturalScroll   dconfig.Bool `prop:"access:rw"`
	EdgeScroll      dconfig.Bool `prop:"access:rw"`
	HorizScroll     dconfig.Bool `prop:"access:rw"`
	VertScroll      dconfig.Bool `prop:"access:rw"`
	TapClick        dconfig.Bool `prop:"access:rw"`
	PalmDetect      dconfig.Bool `prop:"access:rw"`

	MotionAcceleration dconfig.Float64 `prop:"access:rw"`
	MotionThreshold    dconfig.Float64 `prop:"access:rw"`
	MotionScaling      dconfig.Float64 `prop:"access:rw"`

	DoubleClick   dconfig.Int32 `prop:"access:rw"`
	DragThreshold dconfig.Int32 `prop:"access:rw"`
	DeltaScroll   dconfig.Int64 `prop:"access:rw"`
	PalmMinWidth  dconfig.Int64 `prop:"access:rw"`
	PalmMinZ      dconfig.Int64 `prop:"access:rw"`

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

	if err := tpad.initTouchpadDConfig(); err != nil {
		logger.Errorf("Failed to initialize touchpad dconfig: %v", err)
		panic("Touchpad DConfig initialization failed - cannot continue without dconfig support")
	}

	tpad.TPadEnable.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadEnabled)
	tpad.LeftHanded.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadLeftHanded)
	tpad.DisableIfTyping.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadDisableTyping)
	tpad.NaturalScroll.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadNaturalScroll)
	tpad.EdgeScroll.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadEdgeScroll)
	tpad.HorizScroll.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadHorizScroll)
	tpad.VertScroll.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadVertScroll)
	tpad.TapClick.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadTapToClick)
	tpad.PalmDetect.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadPalmDetect)
	tpad.MotionAcceleration.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadAcceleration)
	tpad.MotionThreshold.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadThreshold)
	tpad.MotionScaling.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadScaling)
	tpad.DeltaScroll.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadDeltaScroll)
	tpad.PalmMinWidth.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadPalmMinWidth)
	tpad.PalmMinZ.Bind(tpad.dsgTouchpadConfig, dconfigKeyTouchpadPalmMinZ)

	tpad.DoubleClick.Bind(tpad.dsgMouseConfig, dconfigKeyDoubleClick)
	tpad.DragThreshold.Bind(tpad.dsgMouseConfig, dconfigKeyDragThreshold)

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
				tpad.TPadEnable.Set(enabled)
			}
		}
	}

	currentState := tpad.TPadEnable.Get()
	tpad.TPadEnable.Set(!currentState)
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
			err := v.Enable(!disable && tpad.TPadEnable.Get())
			if err != nil {
				logger.Warningf("Enable '%v - %v' failed: %v",
					v.Id, v.Name, err)
			}
		}
	}
	tpad.disableTemporary = disable
}

func (tpad *Touchpad) enable(enabled bool) {
	if enabled == tpad.TPadEnable.Get() {
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
	tpad.TPadEnable.Set(enabled)
	sysTouchPad, err := inputdevices.NewTouchpad(tpad.systemConn, "/org/deepin/dde/InputDevices1/Touchpad")
	if err == nil && sysTouchPad != nil {
		if err = sysTouchPad.SetTouchpadEnable(0, enabled); err != nil {
			logger.Warning(err)
		}
	}
}

func (tpad *Touchpad) enableLeftHanded() {
	enabled := tpad.LeftHanded.Get()
	for _, v := range tpad.devInfos {
		err := v.EnableLeftHanded(enabled)
		if err != nil {
			logger.Debugf("Enable left handed '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) enableNaturalScroll() {
	enabled := tpad.NaturalScroll.Get()
	for _, v := range tpad.devInfos {
		err := v.EnableNaturalScroll(enabled)
		if err != nil {
			logger.Debugf("Enable natural scroll '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) setScrollDistance() {
	delta := tpad.DeltaScroll.Get()
	for _, v := range tpad.devInfos {
		err := v.SetScrollDistance(int32(delta), int32(delta))
		if err != nil {
			logger.Debugf("Set natural scroll distance '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) enableEdgeScroll() {
	enabled := tpad.EdgeScroll.Get()
	for _, v := range tpad.devInfos {
		err := v.EnableEdgeScroll(enabled)
		if err != nil {
			logger.Debugf("Enable edge scroll '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) enableTwoFingerScroll() {
	vert := tpad.VertScroll.Get()
	horiz := tpad.HorizScroll.Get()
	for _, v := range tpad.devInfos {
		err := v.EnableTwoFingerScroll(vert, horiz)
		if err != nil {
			logger.Debugf("Enable two-finger scroll '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) enableTapToClick() {
	enabled := tpad.TapClick.Get()
	for _, v := range tpad.devInfos {
		err := v.EnableTapToClick(enabled)
		if err != nil {
			logger.Debugf("Enable tap to click '%v - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) motionAcceleration() {
	accel := float32(tpad.MotionAcceleration.Get())
	for _, v := range tpad.devInfos {
		err := v.SetMotionAcceleration(accel)
		if err != nil {
			logger.Debugf("Set acceleration for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) motionThreshold() {
	thres := float32(tpad.MotionThreshold.Get())
	for _, v := range tpad.devInfos {
		err := v.SetMotionThreshold(thres)
		if err != nil {
			logger.Debugf("Set threshold for '%d - %v' failed: %v",
				v.Id, v.Name, err)
		}
	}
}

func (tpad *Touchpad) motionScaling() {
	scaling := float32(tpad.MotionScaling.Get())
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
	enabled := tpad.DisableIfTyping.Get()
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
	enabled := tpad.PalmDetect.Get()
	for _, dev := range tpad.devInfos {
		err := dev.EnablePalmDetect(enabled)
		if err != nil {
			logger.Warning("[enablePalmDetect] failed to enable:", dev.Id, enabled, err)
		}
	}
}

func (tpad *Touchpad) setPalmDimensions() {
	width := tpad.PalmMinWidth.Get()
	z := tpad.PalmMinZ.Get()
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
	dconfig, err := dconfig.NewDConfig(dsettingsAppID, "org.deepin.dde.daemon.gesture", "")
	if err != nil {
		return
	}
	if value, _ := dconfig.GetValueBool("touchpadEnabled"); value == enabled {
		return
	}

	dconfig.SetValue("dconfig", enabled)
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

	tpad.dsgTouchpadConfig.ConnectValueChanged(func(key string) {
		logger.Debugf("Touchpad dconfig value changed: %s", key)
		switch key {
		case dconfigKeyTouchpadEnabled:
			enabled, err := tpad.dsgTouchpadConfig.GetValueBool(dconfigKeyTouchpadEnabled)
			if err != nil {
				logger.Warningf("Failed to get touchpad enabled value from dconfig: %v", err)
			} else {
				tpad.enable(enabled)
			}
		case dconfigKeyTouchpadLeftHanded:
			tpad.enableLeftHanded()
		case dconfigKeyTouchpadDisableTyping:
			tpad.disableWhileTyping()
		case dconfigKeyTouchpadNaturalScroll:
			tpad.enableNaturalScroll()
		case dconfigKeyTouchpadEdgeScroll:
			tpad.enableEdgeScroll()
		case dconfigKeyTouchpadHorizScroll:
			tpad.enableTwoFingerScroll()
		case dconfigKeyTouchpadVertScroll:
			tpad.enableTwoFingerScroll()
		case dconfigKeyTouchpadTapToClick:
			tpad.enableTapToClick()
		case dconfigKeyTouchpadPalmDetect:
			tpad.enablePalmDetect()
		case dconfigKeyTouchpadAcceleration:
			tpad.motionAcceleration()
		case dconfigKeyTouchpadThreshold:
			tpad.motionThreshold()
		case dconfigKeyTouchpadScaling:
			tpad.motionScaling()
		case dconfigKeyTouchpadDeltaScroll:
			tpad.setScrollDistance()
		case dconfigKeyTouchpadPalmMinWidth:
			tpad.setPalmDimensions()
		case dconfigKeyTouchpadPalmMinZ:
			tpad.setPalmDimensions()
		default:
			logger.Debugf("Unhandled touchpad dconfig key change: %s", key)
		}
	})

	logger.Info("Touchpad DConfig initialization completed successfully")
	return nil
}
