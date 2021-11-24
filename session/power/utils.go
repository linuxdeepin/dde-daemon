/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package power

import (
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	dbus "github.com/godbus/dbus"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/dpms"
	"github.com/linuxdeepin/go-x11-client/util/keybind"
	"github.com/linuxdeepin/go-x11-client/util/wm/ewmh"
	"github.com/linuxdeepin/go-x11-client/util/wm/icccm"
	"pkg.deepin.io/dde/api/soundutils"
	. "pkg.deepin.io/lib/gettext"
	"pkg.deepin.io/lib/gsettings"
	"pkg.deepin.io/lib/pulse"
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

func (m *Manager) setDPMSModeOn() {
	logger.Info("DPMS On")
	var err error

	if m.UseWayland {
		_, err = exec.Command("dde_wldpms", "-s", "On").Output()
	} else {
		c := m.helper.xConn
		err = dpms.ForceLevelChecked(c, dpms.DPMSModeOn).Check(c)
	}
	if err != nil {
		logger.Warning("Set DPMS on error:", err)
	}
}

func (m *Manager) setDPMSModeOff() {
	logger.Info("DPMS Off")
	var err error
	if m.UseWayland {
		_, err = exec.Command("dde_wldpms", "-s", "Off").Output()
	} else {
		c := m.helper.xConn
		err = dpms.ForceLevelChecked(c, dpms.DPMSModeOff).Check(c)
	}
	if err != nil {
		logger.Warning("Set DPMS off error:", err)
	}
}

const (
	lockFrontServiceName = "com.deepin.dde.lockFront"
	lockFrontIfc         = lockFrontServiceName
	lockFrontObjPath     = "/com/deepin/dde/lockFront"
)

func (m *Manager) doLock(autoStartAuth bool) {
	// 在锁屏前, 判断是否有应用(窗口)一直占用着键盘(鼠标), 如果有, 通过最小化该窗口强制释放键盘(鼠标)
	// 最小化窗口成功后再去进行锁屏, 否则不锁屏
	locked, err := m.helper.SessionManager.Locked().Get(0)
	if err != nil {
		logger.Warning(err)
	}

	if !locked {
		isMinimizeWindowSuccess := m.canGrabWindowKeyboard(m.helper.xConn)
		if isMinimizeWindowSuccess {
			m.IsBlockLockScreenHasNotified = false
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
		} else {
			m.IsBlockLockScreenHasNotified = true
		}
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
		m.doLock(true)
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
	if icon == iconBatteryLow {
		if !m.LowPowerNotifyEnable.Get() {
			logger.Info("notify switch is off ")
			return
		}
	}

	n := m.helper.Notifications
	_, err := n.Notify(0, "dde-control-center", 0, icon, summary, body, nil, nil, -1)
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
	// 监听 session power 的属性的改变,并发送通知
	gsettings.ConnectChanged(gsSchemaPower, "*", func(key string) {
		logger.Debug("Power Settings Changed :", key)
		switch key {
		case settingKeyLinePowerLidClosedAction:
			value := m.LinePowerLidClosedAction.Get()
			notifyString := getNotifyString(settingKeyLinePowerLidClosedAction, value)
			m.sendNotify(powerSettingsIcon, Tr("Power settings changed"), notifyString)
		case settingKeyLinePowerPressPowerBtnAction:
			value := m.LinePowerPressPowerBtnAction.Get()
			notifyString := getNotifyString(settingKeyLinePowerPressPowerBtnAction, value)
			m.sendNotify(powerSettingsIcon, Tr("Power settings changed"), notifyString)
		case settingKeyBatteryLidClosedAction:
			value := m.BatteryLidClosedAction.Get()
			notifyString := getNotifyString(settingKeyBatteryLidClosedAction, value)
			m.sendNotify(powerSettingsIcon, Tr("Power settings changed"), notifyString)
		case settingKeyBatteryPressPowerBtnAction:
			value := m.BatteryPressPowerBtnAction.Get()
			notifyString := getNotifyString(settingKeyBatteryPressPowerBtnAction, value)
			m.sendNotify(powerSettingsIcon, Tr("Power settings changed"), notifyString)
		}
	})
}

func (m *Manager) canGrabWindowKeyboard(conn *x.Conn) bool {
	var grabWin x.Window
	var grabKbdErr error

	rootWin := conn.GetDefaultScreen().Root
	activeWin, _ := ewmh.GetActiveWindow(conn).Reply(conn)
	name, _ := ewmh.GetWMName(conn, activeWin).Reply(conn)
	wmClass, _ := icccm.GetWMClass(conn, activeWin).Reply(conn)
	class := strings.ToLower(wmClass.Class)

	format := Tr("%q is preventing the computer from locking the screen")
	body := fmt.Sprintf(format, name)

	if activeWin == 0 {
		grabWin = rootWin
	} else {
		// check viewable
		attrs, err := x.GetWindowAttributes(conn, activeWin).Reply(conn)
		if err != nil {
			grabWin = rootWin
		} else if attrs.MapState != x.MapStateViewable {
			// err is nil and activeWin is not viewable
			grabWin = rootWin
		} else {
			// err is nil, activeWin is viewable
			grabWin = activeWin
		}
	}

	// 尝试抓取该激活窗口键盘三次， 确保正确获取抓取窗口的键盘状态
	for i := 0; i < 3; i++ {
		grabKbdErr = keybind.GrabKeyboard(conn, grabWin)
		if grabKbdErr == nil {
			// grab keyboard successful
			err := keybind.UngrabKeyboard(conn)
			if err != nil {
				logger.Warning("ungrabKeyboard Failed:", err)
			}

			return true
		}

		time.Sleep(100 * time.Millisecond)
	}

	gkErr, ok := grabKbdErr.(keybind.GrabKeyboardError)
	logger.Debugf("grabKeyboard win %d failed: %v", grabWin, grabKbdErr)

	// 如果键盘已经被某个应用抓取, 将该应用的窗口最小化,使其释放抓取的键盘, 防止dde-lock在锁屏时因抓取键盘失败导致锁屏一直不成功
	// 如果是桌面右键菜单（不能被最小化释放键盘)或者设置了唤醒显示器需要密码开关(最小化窗口的操作会打断系统空闲处理)，弹提示提醒用户，不去锁屏
	if ok && gkErr.Status == x.GrabStatusAlreadyGrabbed && class != "dde-desktop" && !m.ScreenBlackLock.Get() {
		// 保存最小化所有窗口的状态
		m.isMinimizeAllWindows = true

		err := exec.Command("/usr/lib/deepin-daemon/desktop-toggle").Run()
		if err != nil {
			logger.Warning("failed to call desktop-toggle:", err)

			// 最小化所有窗口失败弹提示提醒用户
			if !m.IsBlockLockScreenHasNotified {
				m.sendNotify("preferences-system", "", body)
				return false
			}
		}

		return true
	}

	// 抓取键盘失败弹(“XX（程序）”阻止了屏幕锁定")提醒用户
	if !m.IsBlockLockScreenHasNotified {
		m.sendNotify("preferences-system", "", body)
	}

	return false
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
	case powerActionDoNothing:
		return Tr("it will do nothing to your computer")
	}
	return ""
}

func isFloatEqual(f1, f2 float64) bool {
	return math.Abs(f1-f2) < 1e-6
}
