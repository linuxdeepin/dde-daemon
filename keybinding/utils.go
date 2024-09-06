// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package keybinding

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	dbus "github.com/godbus/dbus/v5"
	wm "github.com/linuxdeepin/go-dbus-factory/session/com.deepin.wm"
	"github.com/linuxdeepin/go-x11-client/ext/dpms"

	"github.com/linuxdeepin/dde-daemon/keybinding/util"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/strv"
)

// nolint
const (
	suspendStateUnknown = iota + 1
	suspendStateLidOpen
	suspendStateFinish
	suspendStateWakeup
	suspendStatePrepare
	suspendStateLidClose
	suspendStateButtonClick
)

func resetGSettings(gs *gio.Settings) {
	for _, key := range gs.ListKeys() {
		userVal := gs.GetUserValue(key)
		if userVal != nil {
			// TODO unref userVal
			logger.Debug("reset gsettings key", key)
			gs.Reset(key)
		}
	}
}

func resetKWin(wmObj wm.Wm) error {
	accels, err := util.GetAllKWinAccels(wmObj)
	if err != nil {
		return err
	}
	for _, accel := range accels {
		// logger.Debug("resetKwin each accel:", accel.Id, accel.Keystrokes, accel.DefaultKeystrokes)
		if !strv.Strv(accel.Keystrokes).Equal(accel.DefaultKeystrokes) &&
			len(accel.DefaultKeystrokes) > 0 && accel.DefaultKeystrokes[0] != "" {
			accelJson, err := util.MarshalJSON(&util.KWinAccel{
				Id:         accel.Id,
				Keystrokes: accel.DefaultKeystrokes,
			})
			if err != nil {
				logger.Warning(err)
				continue
			}
			logger.Debug("resetKwin SetAccel", accelJson)
			ok, err := wmObj.SetAccel(0, accelJson)
			// 目前 wm 的实现，调用 SetAccel 如果遇到冲突情况，会导致目标快捷键被清空。
			if !ok {
				logger.Warning("wm.SetAccel failed, id: ", accel.Id)
				continue
			}
			if err != nil {
				logger.Warning("failed to set accel:", err, accel.Id)
				continue
			}
		}
	}
	return nil
}

func showOSD(signal string) {
	logger.Debug("show OSD", signal)
	sessionDBus, _ := dbus.SessionBus()
	go sessionDBus.Object("org.deepin.dde.Osd1", "/").Call("org.deepin.dde.Osd1.ShowOSD", 0, signal)
}

const sessionManagerDest = "org.deepin.dde.SessionManager1"
const sessionManagerObjPath = "/org/deepin/dde/SessionManager1"

func systemLock() {
	sessionDBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}
	go sessionDBus.Object(sessionManagerDest, sessionManagerObjPath).Call(sessionManagerDest+".RequestLock", 0)
}

func (m *Manager) canSuspend() bool {
	can, err := m.sessionManager.CanSuspend(0) // 当前能否待机
	if err != nil {
		logger.Warning(err)
		return false
	}
	return can
}

func (m *Manager) systemSuspend() {
	if !m.canSuspend() {
		logger.Info("can not suspend")
		return
	}

	logger.Debug("suspend")
	err := m.sessionManager.RequestSuspend(0)
	if err != nil {
		logger.Warning("failed to suspend:", err)
	}
}

// 为了处理待机闪屏的问题，通过前端进行待机，前端会在待机前显示一个纯黑的界面
func (m *Manager) systemSuspendByFront() {
	if !m.canSuspend() {
		logger.Info("can not suspend")
		return
	}

	logger.Debug("suspend")
	err := m.shutdownFront.Suspend(0)
	if err != nil {
		logger.Warning("failed to suspend:", err)
	}
}

func (m *Manager) canHibernate() bool {
	can, err := m.sessionManager.CanHibernate(0) // 能否休眠
	if err != nil {
		logger.Warning(err)
		return false
	}
	return can
}

func systemSuspend() {
	sessionDBus, _ := dbus.SessionBus()
	go sessionDBus.Object(sessionManagerDest, sessionManagerObjPath).Call(sessionManagerDest+".RequestSuspend", 0)
}

func (m *Manager) systemHibernate() {
	if !m.canHibernate() {
		logger.Info("can not Hibernate")
		return
	}

	logger.Debug("Hibernate")
	err := m.sessionManager.RequestHibernate(0)
	if err != nil {
		logger.Warning("failed to Hibernate:", err)
	}
}

func (m *Manager) systemHibernateByFront() {
	if !m.canHibernate() {
		logger.Info("can not Hibernate")
		return
	}

	logger.Debug("Hibernate")
	err := m.shutdownFront.Hibernate(0)
	if err != nil {
		logger.Warning("failed to Hibernate:", err)
	}
}

func (m *Manager) canShutdown() bool {
	can, err := m.sessionManager.CanShutdown(0) // 当前能否关机
	if err != nil {
		logger.Warning(err)
		return false
	}
	return can
}

func (m *Manager) systemShutdown() {
	if !m.canShutdown() {
		logger.Info("can not Shutdown")
		return
	}

	locked, err := m.sessionManager.Locked().Get(0)
	if err != nil {
		logger.Warning("sessionManager get locked error:", err)
		return
	}
	if locked {
		logger.Info("session is locked")
		return
	}

	logger.Debug("Shutdown")
	err = m.sessionManager.RequestShutdown(0)
	if err != nil {
		logger.Warning("failed to Shutdown:", err)
	}
}

func (m *Manager) systemTurnOffScreen() {
	const settingKeyScreenBlackLock = "screen-black-lock"
	logger.Info("DPMS Off")
	var err error
	var useWayland bool
	if len(os.Getenv("WAYLAND_DISPLAY")) != 0 {
		useWayland = true
	} else {
		useWayland = false
	}
	if m.gsPower.GetBoolean(settingKeyScreenBlackLock) {
		m.doLock(true)
	}

	doPrepareSuspend()
	if useWayland {
		err = exec.Command("dde_wldpms", "-s", "Off").Run()
	} else {
		err = dpms.ForceLevelChecked(m.conn, dpms.DPMSModeOff).Check(m.conn)
	}
	if err != nil {
		logger.Warning("Set DPMS off error:", err)
	}
	undoPrepareSuspend()
}

func (m *Manager) systemLogout() {
	can, err := m.sessionManager.CanLogout(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	if !can {
		logger.Info("can not logout")
		return
	}

	logger.Debug("logout")
	err = m.sessionManager.RequestLogout(0)
	if err != nil {
		logger.Warning("failed to logout:", err)
	}
}

func (m *Manager) systemAway() {
	err := m.sessionManager.RequestLock(0)
	if err != nil {
		logger.Warning(err)
	}
}

func queryAppDesktopByMime(mime string) (string, error) {
	desktop, err := exec.Command("xdg-mime", "query", "default", mime).Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(desktop)), nil
}

func getRfkillWlanState() (int, error) {
	dir := "/sys/class/rfkill"
	fileInfoList, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	for _, fileInfo := range fileInfoList {
		typeFile := filepath.Join(dir, fileInfo.Name(), "type")
		typeBytes, err := readTinyFile(typeFile)
		if err != nil {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(typeBytes), []byte("wlan")) {
			stateFile := filepath.Join(dir, fileInfo.Name(), "state")
			stateBytes, err := readTinyFile(stateFile)
			if err != nil {
				return 0, err
			}
			stateBytes = bytes.TrimSpace(stateBytes)
			state, err := strconv.Atoi(string(stateBytes))
			if err != nil {
				return 0, err
			}
			return state, nil

		}
	}
	return 0, errors.New("not found rfkill with type wlan")
}

func readTinyFile(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, 8)
	n, err := f.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func shouldUseDDEKwin() bool {
	_, err := exec.LookPath("deepin-kwin_x11")
	return err == nil
}

func (m *Manager) doLock(autoStartAuth bool) {
	logger.Info("Lock Screen")
	err := m.lockFront.ShowAuth(0, autoStartAuth)
	if err != nil {
		logger.Warning("failed to call lockFront ShowAuth:", err)
	}
}

func doPrepareSuspend() {
	sessionDBus, _ := dbus.SessionBus()
	obj := sessionDBus.Object("org.deepin.dde.Power1", "/org/deepin/dde/Power1")
	err := obj.Call("org.deepin.dde.Power1.SetPrepareSuspend", 0, suspendStateButtonClick).Err
	if err != nil {
		logger.Warning(err)
	}
}

func undoPrepareSuspend() {
	sessionDBus, _ := dbus.SessionBus()
	obj := sessionDBus.Object("org.deepin.dde.Power1", "/org/deepin/dde/Power1")
	err := obj.Call("org.deepin.dde.Power1.SetPrepareSuspend", 0, suspendStateFinish).Err
	if err != nil {
		logger.Warning(err)
	}
}
