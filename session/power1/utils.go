// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"math"
	"os/exec"
	"time"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/soundutils"
	. "github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/gsettings"
	"github.com/linuxdeepin/go-lib/pulse"
	"github.com/linuxdeepin/go-x11-client/ext/dpms"
)

const (
	dpmsStateOn int32 = iota
	dpmsStateStandBy
	dpmsStateSuspend
	dpmsStateOff
)

func (m *Manager) waitLockShowing(timeout time.Duration) {
	ticker := time.NewTicker(time.Millisecond * 300)
	timer := time.NewTimer(timeout)
	for {
		select {
		case _, ok := <-ticker.C:
			if !ok {
				logger.Error("Invalid ticker event")
				return
			}

			logger.Debug("waitLockShowing tick")
			locked, err := m.helper.SessionManager.Locked().Get(0)
			if err != nil {
				logger.Warning(err)
				continue
			}
			if locked {
				logger.Debug("waitLockShowing found")
				ticker.Stop()
				return
			}

		case <-timer.C:
			logger.Debug("waitLockShowing failed timeout!")
			ticker.Stop()
			return
		}
	}
}

func (m *Manager) lockWaitShow(timeout time.Duration, autoStartAuth bool) {
	m.doLock(autoStartAuth)
	if m.UseWayland {
		logger.Debug("In wayland environment, unsupported check lock whether showin")
		return
	}
	m.waitLockShowing(timeout)
}

func (m *Manager) isWmBlackScreenActive() bool {
	bus, err := dbus.SessionBus()
	if err == nil {
		kwinInter := bus.Object("org.kde.KWin", "/BlackScreen")
		var active bool
		err = kwinInter.Call("org.kde.kwin.BlackScreen.getActive",
			dbus.FlagNoAutoStart).Store(&active)
		if err != nil {
			logger.Warning("failed to get kwin blackscreen effect active:", err)
			return false
		}
		return active
	} else {
		return false
	}
}

func (m *Manager) setWmBlackScreenActive(active bool) {
	logger.Info("set blackScreen effect active: ", active)
	bus, err := dbus.SessionBus()
	if err == nil {
		kwinInter := bus.Object("org.kde.KWin", "/BlackScreen")
		err = kwinInter.Call("org.kde.kwin.BlackScreen.setActive", 0, active).Err
		if err != nil {
			logger.Warning("set blackScreen active failed:", err)
		}
	}
}

func (m *Manager) getDPMSMode() int32 {
	logger.Debug("get DPMS Mode")

	var err error
	var mode int32

	if m.UseWayland {
		mode, err = m.getDpmsModeByKwin()
	} else {
		var dpmsInfo *dpms.InfoReply
		c := m.helper.xConn
		dpmsInfo, err = dpms.Info(c).Reply(c)
		if err != nil {
			mode = int32(dpmsInfo.PowerLevel)
		}
	}

	if err != nil {
		logger.Warning("get DPMS mode error:", err)
	}

	return mode
}

func (m *Manager) setDPMSModeOn() {
	logger.Info("DPMS On")

	var err error

	if m.UseWayland {
		err = m.setDpmsModeByKwin(dpmsStateOn)
	} else {
		c := m.helper.xConn
		err = dpms.ForceLevelChecked(c, dpms.DPMSModeOn).Check(c)
	}

	if err != nil {
		logger.Warning("set DPMS on error:", err)
	}
}

func (m *Manager) setDPMSModeOff() {
	logger.Info("DPMS Off")

	var err error
	if m.UseWayland {
		err = m.setDpmsModeByKwin(dpmsStateOff)
	} else {
		c := m.helper.xConn
		err = dpms.ForceLevelChecked(c, dpms.DPMSModeOff).Check(c)
	}
	if err != nil {
		logger.Warning("set DPMS off error:", err)
	}
}

const (
	lockFrontServiceName = "org.deepin.dde.LockFront1"
	lockFrontIfc         = lockFrontServiceName
	lockFrontObjPath     = "/org/deepin/dde/LockFront1"
)

func (m *Manager) doLock(autoStartAuth bool) {
	logger.Info("Lock Screen")
	bus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}
	lockFrontObj := bus.Object(lockFrontServiceName, lockFrontObjPath)
	err = lockFrontObj.Call(lockFrontIfc+".ShowAuth", 0, autoStartAuth).Err
	if err != nil {
		logger.Warning("failed to call lockFront ShowAuth:", err)
	}
}

func (m *Manager) canSuspend() bool {
	can, err := m.helper.SessionManager.CanSuspend(0)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return can
}

func (m *Manager) doSuspend() {
	if !m.canSuspend() {
		logger.Info("can not suspend")
		return
	}

	logger.Debug("suspend")
	err := m.helper.SessionManager.RequestSuspend(0)
	if err != nil {
		logger.Warning("failed to suspend:", err)
	}
}

// 为了处理待机闪屏的问题，通过前端进行待机，前端会在待机前显示一个纯黑的界面
func (m *Manager) doSuspendByFront() {
	if !m.canSuspend() {
		logger.Info("can not suspend")
		return
	}

	logger.Debug("suspend")
	err := m.helper.ShutdownFront.Suspend(0)
	if err != nil {
		logger.Warning("failed to suspend:", err)
	}
}

func (m *Manager) canShutdown() bool {
	can, err := m.helper.SessionManager.CanShutdown(0)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return can
}

func (m *Manager) doShutdown() {
	sessionManager := m.helper.SessionManager
	can, err := sessionManager.CanShutdown(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	if !can {
		logger.Info("can not Shutdown")
		return
	}

	logger.Debug("Shutdown")
	err = sessionManager.RequestShutdown(0)
	if err != nil {
		logger.Warning("failed to Shutdown:", err)
	}
}

func (m *Manager) canHibernate() bool {
	can, err := m.helper.SessionManager.CanHibernate(0)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return can
}

func (m *Manager) doHibernate() {
	if !m.canHibernate() {
		logger.Info("can not hibernate")
		return
	}
	logger.Debug("hibernate")
	err := m.helper.SessionManager.RequestHibernate(0)
	if err != nil {
		logger.Warning("failed to hibernate:", err)
	}
}

func (m *Manager) doHibernateByFront() {
	if !m.canHibernate() {
		logger.Info("can not hibernate")
		return
	}

	logger.Debug("hibernate")
	err := m.helper.ShutdownFront.Hibernate(0)
	if err != nil {
		logger.Warning("failed to hibernate:", err)
	}
}

func (m *Manager) doTurnOffScreen() {
	if m.ScreenBlackLock.Get() {
		logger.Info("Show lock")
		m.doLock(true)
		time.Sleep(1 * time.Second)
	}

	logger.Info("Turn off screen")
	m.setDPMSModeOff()
}

func (m *Manager) setDisplayBrightness(brightnessTable map[string]float64) {
	display := m.helper.Display
	for output, brightness := range brightnessTable {
		logger.Infof("Change output %q brightness to %.2f", output, brightness)
		err := display.SetBrightness(0, output, brightness)
		if err != nil {
			logger.Warningf("Change output %q brightness to %.2f failed: %v", output, brightness, err)
		} else {
			logger.Infof("Change output %q brightness to %.2f done!", output, brightness)
		}
	}
}

func (m *Manager) setAndSaveDisplayBrightness(brightnessTable map[string]float64) {
	display := m.helper.Display
	for output, brightness := range brightnessTable {
		logger.Infof("Change output %q brightness to %.2f", output, brightness)
		err := display.SetAndSaveBrightness(0, output, brightness)
		if err != nil {
			logger.Warningf("Change output %q brightness to %.2f failed: %v", output, brightness, err)
		} else {
			logger.Infof("Change output %q brightness to %.2f done!", output, brightness)
		}
	}
}

func doShowDDELowPower() {
	logger.Info("Show dde low power")
	go func() {
		err := exec.Command(cmdDDELowPower, "--raise").Run()
		if err != nil {
			logger.Warning(err)
		}
	}()
}

func doCloseDDELowPower() {
	logger.Info("Close low power")
	go func() {
		err := exec.Command(cmdDDELowPower, "--quit").Run()
		if err != nil {
			logger.Warning(err)
		}
	}()
}

func (m *Manager) sendNotify(icon, summary, body string) {
	if !m.LowPowerNotifyEnable.Get() {
		logger.Info("notify switch is off ")
		return
	}
	n := m.helper.Notifications
	_, err := n.Notify(0, Tr("dde-control-center"), 0, icon, summary, body, nil, nil, -1)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) sendChangeNotify(icon, summary, body string) {
	n := m.helper.Notifications
	_, err := n.Notify(0, Tr("dde-control-center"), 0, icon, summary, body, nil, nil, -1)
	if err != nil {
		logger.Warning(err)
	}
}

const iconBatteryLow = "notification-battery-low"

func playSound(name string) {
	logger.Debug("play system sound", name)
	go func() {
		err := soundutils.PlaySystemSound(name, "")
		if err != nil {
			logger.Warning(err)
		}
	}()
}

const (
	// deepin-screensaver dbus
	deepinScreensaverDBusServiceName = "com.deepin.ScreenSaver"
	deepinScreensaverDBusPath        = "/com/deepin/ScreenSaver"
	deepinScreensaverDBusInterface   = deepinScreensaverDBusServiceName
)

func startScreensaver() {
	logger.Info("start screensaver")
	bus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}

	obj := bus.Object(deepinScreensaverDBusServiceName, deepinScreensaverDBusPath)
	err = obj.Call(deepinScreensaverDBusInterface+".Start", 0).Err
	if err != nil {
		logger.Warning(err)
	}
}

func stopScreensaver() {
	logger.Info("stop screensaver")
	bus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}

	obj := bus.Object(deepinScreensaverDBusServiceName, deepinScreensaverDBusPath)
	err = obj.Call(deepinScreensaverDBusInterface+".Stop", 0).Err
	if err != nil {
		logger.Warning(err)
	}
}

// TODO(jouyouyun): move to common library
func suspendPulseSinks(suspend int) {
	var ctx = pulse.GetContext()
	if ctx == nil {
		logger.Error("Failed to connect pulseaudio server")
		return
	}
	for _, sink := range ctx.GetSinkList() {
		ctx.SuspendSinkById(sink.Index, suspend)
	}
}

// TODO(jouyouyun): move to common library
func suspendPulseSources(suspend int) {
	var ctx = pulse.GetContext()
	if ctx == nil {
		logger.Error("Failed to connect pulseaudio server")
		return
	}
	for _, source := range ctx.GetSourceList() {
		ctx.SuspendSourceById(source.Index, suspend)
	}
}

func (m *Manager) initGSettingsConnectChanged() {
	const powerSettingsIcon = "preferences-system"

	isIllegalAction := func(action int32) bool {
		return (action == powerActionHibernate && !m.canHibernate()) ||
			(action == powerActionSuspend && !m.canSuspend())
	}

	// 监听 session power 的属性的改变,并发送通知
	gsettings.ConnectChanged(gsSchemaPower, "*", func(key string) {
		logger.Debug("Power Settings Changed :", key)
		switch key {
		case settingKeyLinePowerLidClosedAction:
			value := m.LinePowerLidClosedAction.Get()
			if isIllegalAction(value) {
				break
			}
			notifyString := getNotifyString(settingKeyLinePowerLidClosedAction, value)
			m.sendChangeNotify(powerSettingsIcon, Tr("Power settings changed"), notifyString)
		case settingKeyLinePowerPressPowerBtnAction:
			value := m.LinePowerPressPowerBtnAction.Get()
			if isIllegalAction(value) {
				break
			}
			notifyString := getNotifyString(settingKeyLinePowerPressPowerBtnAction, value)
			m.sendChangeNotify(powerSettingsIcon, Tr("Power settings changed"), notifyString)
		case settingKeyBatteryLidClosedAction:
			value := m.BatteryLidClosedAction.Get()
			if isIllegalAction(value) {
				break
			}
			notifyString := getNotifyString(settingKeyBatteryLidClosedAction, value)
			m.sendChangeNotify(powerSettingsIcon, Tr("Power settings changed"), notifyString)
		case settingKeyBatteryPressPowerBtnAction:
			value := m.BatteryPressPowerBtnAction.Get()
			if isIllegalAction(value) {
				break
			}
			notifyString := getNotifyString(settingKeyBatteryPressPowerBtnAction, value)
			m.sendChangeNotify(powerSettingsIcon, Tr("Power settings changed"), notifyString)
		case settingKeyHighPerformanceEnabled:
			//根据systemPower::IsHighPerformanceEnabled GSetting::settingKeyHighPerformanceEnabled
			bSettingKeyHighPerformanceEnabled := m.settings.GetBoolean(settingKeyHighPerformanceEnabled)
			if bSettingKeyHighPerformanceEnabled == m.gsHighPerformanceEnabled {
				return
			}
			m.gsHighPerformanceEnabled = bSettingKeyHighPerformanceEnabled
			isHighPerformanceSupported, _ := m.systemPower.IsHighPerformanceSupported().Get(0)
			m.setPropIsHighPerformanceSupported(isHighPerformanceSupported && bSettingKeyHighPerformanceEnabled)
		}
	})
}

func getNotifyString(option string, action int32) string {
	var (
		notifyString string
		firstPart    string
		secondPart   string
	)
	switch option {
	case
		settingKeyLinePowerLidClosedAction,
		settingKeyBatteryLidClosedAction:
		firstPart = Tr("When the lid is closed, ")
	case
		settingKeyLinePowerPressPowerBtnAction,
		settingKeyBatteryPressPowerBtnAction:
		firstPart = Tr("When pressing the power button, ")
	}
	secondPart = getPowerActionString(action)
	notifyString = firstPart + secondPart
	return notifyString
}

func getPowerActionString(action int32) string {
	switch action {
	case powerActionShutdown:
		return Tr("your computer will shut down")
	case powerActionSuspend:
		return Tr("your computer will suspend")
	case powerActionHibernate:
		return Tr("your computer will hibernate")
	case powerActionTurnOffScreen:
		return Tr("your monitor will turn off")
	case powerActionShowShutdownInterface:
		return Tr("your monitor will show the shutdown interface")
	case powerActionDoNothing:
		return Tr("it will do nothing to your computer")
	}
	return ""
}

func isFloatEqual(f1, f2 float64) bool {
	return math.Abs(f1-f2) < 1e-6
}

func (m *Manager) getDpmsList() ([]dbus.Variant, error) {
	sessionBus := m.sessionSigLoop.Conn()
	sessionObj := sessionBus.Object("com.deepin.daemon.KWayland", "/com/deepin/daemon/KWayland/DpmsManager")
	var ret []dbus.Variant
	err := sessionObj.Call("com.deepin.daemon.KWayland.DpmsManager.dpmsList", 0).Store(&ret)
	if err != nil {
		logger.Warning(err)
		return nil, err
	}

	return ret, nil
}

func (m *Manager) getDpmsModeByKwin() (int32, error) {
	list, err := m.getDpmsList()
	if err != nil {
		logger.Warning(err)
		return dpmsStateOn, err
	}

	var dpmsMode int32
	for i := 0; i < len(list); i++ {
		v := list[i].Value().(string)
		sessionObj := m.sessionSigLoop.Conn().Object("com.deepin.daemon.KWayland", dbus.ObjectPath(v))
		err = sessionObj.Call("com.deepin.daemon.KWayland.Dpms.getDpmsMode", 0).Store(&dpmsMode)
		if err != nil {
			logger.Warning(err)
			return dpmsStateOn, err
		}
	}

	return dpmsMode, nil
}

func (m *Manager) setDpmsModeByKwin(mode int32) error {
	list, err := m.getDpmsList()
	if err != nil {
		logger.Warning(err)
		return err
	}

	for i := 0; i < len(list); i++ {
		v := list[i].Value().(string)
		sessionObj := m.sessionSigLoop.Conn().Object("com.deepin.daemon.KWayland", dbus.ObjectPath(v))
		err = sessionObj.Call("com.deepin.daemon.KWayland.Dpms.setDpmsMode", 0, int32(mode)).Err
		if err != nil {
			logger.Warning(err)
			return err
		}
	}

	return nil
}
