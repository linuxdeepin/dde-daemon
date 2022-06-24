/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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

package power1

import (
	"os/exec"
	"strings"
	"time"
)

func init() {
	submoduleList = append(submoduleList, newLidSwitchHandler)
}

// nolint
const (
	lidSwitchStateUnknown = iota
	lidSwitchStateOpen
	lidSwitchStateClose
)

type LidSwitchHandler struct {
	manager       *Manager
	cmd           *exec.Cmd
	cookie        chan struct{}
	isLidOpenLast bool //上一次有效操作是否是开盖
}

func newLidSwitchHandler(m *Manager) (string, submodule, error) {
	h := &LidSwitchHandler{
		manager:       m,
		isLidOpenLast: true,
	}
	return "LidSwitchHandler", h, nil
}

func (h *LidSwitchHandler) Start() error {
	power := h.manager.helper.Power
	_, err := power.ConnectLidClosed(h.onLidClosed)
	if err != nil {
		return err
	}
	_, err = power.ConnectLidOpened(h.onLidOpened)
	if err != nil {
		return err
	}
	return nil
}

func (h *LidSwitchHandler) onLidClosed() {
	h.onLidDelayOperate(false)
}

func (h *LidSwitchHandler) onLidOpened() {
	h.onLidDelayOperate(true)
}

func (h *LidSwitchHandler) onLidDelayOperate(state bool) {
	if h.cookie != nil {
		h.cookie <- struct{}{}
		close(h.cookie)
	}

	h.cookie = make(chan struct{}, 1)

	go func() {
		select {
		case <-h.cookie:
			break
		// 1.5s是为了保证在关闭显示器时，其强制等待的1秒操作被完全执行，否则会带来闪屏问题
		case <-time.After(time.Millisecond * 1500):
			h.doLidStateChanged(state)
			break
		}
	}()
}

func (h *LidSwitchHandler) doLidStateChanged(state bool) {
	logger.Info("Lid open:", state)
	if h.isLidOpenLast == state {
		logger.Info("ignore operate")
		return
	}
	h.isLidOpenLast = state

	m := h.manager
	m.setPrepareSuspend(suspendStateLidClose)
	m.PropsMu.Lock()
	m.lidSwitchState = lidSwitchStateClose
	m.PropsMu.Unlock()
	m.claimOrReleaseAmbientLight()

	// 合盖
	if !state {
		var onBattery bool
		onBattery = h.manager.OnBattery
		var lidCloseAction int32
		if onBattery {
			lidCloseAction = m.BatteryLidClosedAction.Get() // 获取合盖操作
		} else {
			lidCloseAction = m.LinePowerLidClosedAction.Get() // 获取合盖操作
		}
		switch lidCloseAction {
		case powerActionShutdown:
			m.doShutdown()
		case powerActionSuspend:
			m.doSuspendByFront()
		case powerActionHibernate:
			m.doHibernateByFront()
		case powerActionTurnOffScreen:
			m.doTurnOffScreen()
		case powerActionDoNothing:
			return
		}

		if !m.isWmBlackScreenActive() {
			m.setWmBlackScreenActive(true)
		}
	} else { // 开盖
		err := h.stopAskUser()
		if err != nil {
			logger.Warning("stopAskUser error:", err)
		}

		err = m.helper.ScreenSaver.SimulateUserActivity(0)
		if err != nil {
			logger.Warning(err)
		}

		if m.isWmBlackScreenActive() {
			m.setWmBlackScreenActive(false)
		}

		m.setDPMSModeOn()
	}
}

func (h *LidSwitchHandler) stopAskUser() error {
	if h.cmd == nil {
		return nil
	}

	if h.cmd.ProcessState == nil {
		// h.cmd is running
		logger.Debug("stopAskUser: kill process")
		err := h.cmd.Process.Kill()
		if err != nil {
			return err
		}
	} else {
		logger.Debug("stopAskUser: cmd exited")
	}
	h.cmd = nil
	return nil
}

// copy from display module of project startdde
func isBuiltinOutput(name string) bool {
	name = strings.ToLower(name)
	switch {
	case strings.Contains(name, "lvds"):
		// Most drivers use an "LVDS" prefix
		fallthrough
	case strings.Contains(name, "lcd"):
		// fglrx uses "LCD" in some versions
		fallthrough
	case strings.Contains(name, "edp"):
		// eDP is for internal built-in panel connections
		fallthrough
	case strings.Contains(name, "dsi"):
		return true
	case name == "default":
		// now sunway notebook has only one output named default
		return true
	}
	return false
}

func (h *LidSwitchHandler) Destroy() {
}
