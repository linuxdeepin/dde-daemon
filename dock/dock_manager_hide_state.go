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

package dock

import (
	"errors"
	"time"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/wm/ewmh"
)

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func hasIntersection(rectA, rectB *Rect) bool {
	if rectA == nil || rectB == nil {
		logger.Warning("hasIntersection rectA or rectB is nil")
		return false
	}
	X, y, w, h := rectA.Pieces()
	x1, y1, w1, h1 := rectB.Pieces()
	ax := max(X, x1)
	ay := max(y, y1)
	bx := min(X+w, x1+w1)
	by := min(y+h, y1+h1)
	return ax < bx && ay < by
}

func (m *Manager) getActiveWinGroup(activeWin x.Window) (ret []x.Window) {

	ret = []x.Window{activeWin}

	list, err := ewmh.GetClientListStacking(globalXConn).Reply(globalXConn)
	if err != nil {
		logger.Warning(err)
		return
	}

	idx := -1
	for i, win := range list {
		if win == activeWin {
			idx = i
			break
		}
	}
	if idx == -1 {
		logger.Warning("getActiveWinGroup: not found active window in clientListStacking")
		return
	} else if idx == 0 {
		return
	}

	aPid := getWmPid(activeWin)
	aLeaderWin, _ := getWmClientLeader(activeWin)

	for i := idx - 1; i >= 0; i-- {
		win := list[i]
		pid := getWmPid(win)
		if aPid != 0 && pid == aPid {
			// ok
			ret = append(ret, win)
			continue
		}

		wmClass, _ := getWmClass(win)
		if wmClass != nil && wmClass.Class == frontendWindowWmClass {
			// skip over frontend window
			continue
		}

		leaderWin, _ := getWmClientLeader(win)
		if aLeaderWin != 0 && leaderWin == aLeaderWin {
			// ok
			ret = append(ret, win)
			continue
		}

		aboveWin := list[i+1]
		aboveWinTransientFor, _ := getWmTransientFor(aboveWin)
		if aboveWinTransientFor != 0 && aboveWinTransientFor == win {
			// ok
			ret = append(ret, win)
			continue
		}

		break
	}
	return
}

func (m *Manager) isWindowDockOverlap(win x.Window) (bool, error) {
	// overlap condition:
	// window type is not desktop
	// window opacity is not zero
	// window showing and  on current workspace,
	// window dock rect has intersection

	windowType, err := ewmh.GetWMWindowType(globalXConn, win).Reply(globalXConn)

	if err == nil && atomsContains(windowType, atomNetWmWindowTypeDesktop) {
		return false, nil
	}

	opacity, err := getWmWindowOpacity(win)
	if err == nil && opacity == 0 {
		return false, nil
	}

	if isHiddenPre(win) || (!onCurrentWorkspacePre(win)) {
		logger.Debugf("window %v is hidden or not on current workspace", win)
		return false, nil
	}

	winRect, err := getWindowGeometry(globalXConn, win)
	if err != nil {
		logger.Warning("Get target window geometry failed", err)
		return false, err
	}

	logger.Debug("window rect:", winRect)
	logger.Debug("dock rect:", m.FrontendWindowRect)
	return hasIntersection(winRect, m.FrontendWindowRect), nil
}

const (
	ddeLauncherWMClass = "dde-launcher"
)

func isDDELauncher(win x.Window) (bool, error) {
	winClass, err := getWmClass(win)
	if err != nil {
		return false, err
	}
	return winClass.Instance == ddeLauncherWMClass, nil
}

func (m *Manager) getActiveWindow() (activeWin WindowInfoImp) {
	m.activeWindowMu.Lock()
	if m.activeWindow == nil {
		activeWin = m.activeWindowOld
	} else {
		activeWin = m.activeWindow
	}
	m.activeWindowMu.Unlock()
	return
}

func (m *Manager) shouldHideOnSmartHideModeX(activeWin x.Window) (bool, error) {
	isLauncher, err := isDDELauncher(activeWin)
	if err != nil {
		logger.Warning(err)
	}
	if isLauncher {
		// dde launcher is invisible, but it is still active window
		logger.Debug("shouldHideOnSmartHideMode: active window is dde launcher")
		return false, nil
	}

	list := m.getActiveWinGroup(activeWin)
	logger.Debug("shouldHideOnSmartHideMode: activeWinGroup is", list)
	for _, win := range list {
		over, err := m.isWindowDockOverlap(win)
		if err != nil {
			logger.Warning(err)
		}
		logger.Debugf("shouldHideOnSmartHideMode: win %d dock overlap %v", win, over)
		if over {
			return true, nil
		}
	}
	return false, nil
}

func (m *Manager) shouldHideOnSmartHideModeK(activeWin *KWindowInfo) (bool, error) {
	return m.isWindowDockOverlapK(activeWin)
}

func (m *Manager) shouldHideOnSmartHideMode() (bool, error) {
	activeWinInfo := m.getActiveWindow()
	if activeWinInfo == nil {
		logger.Debug("shouldHideOnSmartHideMode: activeWinInfo is nil")
		return false, errors.New("activeWinInfo is nil")
	}
	if m.isDDELauncherVisible() {
		logger.Debug("shouldHideOnSmartHideMode: dde launcher is visible")
		return false, nil
	}

	switch winInfo := activeWinInfo.(type) {
	case *WindowInfo:
		activeWin := winInfo.getXid()
		return m.shouldHideOnSmartHideModeX(activeWin)

	case *KWindowInfo:
		return m.shouldHideOnSmartHideModeK(winInfo)
	default:
		return false, errors.New("invalid type WindowInfo")
	}
}

func (m *Manager) smartHideModeTimerExpired() {
	logger.Debug("smartHideModeTimer expired!")
	shouldHide, err := m.shouldHideOnSmartHideMode()
	if err != nil {
		logger.Warning(err)
		m.setPropHideState(HideStateUnknown)
		return
	}

	if shouldHide {
		m.setPropHideState(HideStateHide)
	} else {
		m.setPropHideState(HideStateShow)
	}
}

func (m *Manager) resetSmartHideModeTimer(delay time.Duration) {
	m.smartHideModeMutex.Lock()
	defer m.smartHideModeMutex.Unlock()

	m.smartHideModeTimer.Reset(delay)
	logger.Debug("reset smart hide mode timer ", delay)
}

func (m *Manager) updateHideState(delay bool) {
	if m.isDDELauncherVisible() {
		logger.Debug("updateHideState: dde launcher is visible, show dock")
		m.setPropHideState(HideStateShow)
		return
	}

	hideMode := HideModeType(m.HideMode.Get())
	logger.Debug("updateHideState: mode is", hideMode)
	switch hideMode {
	case HideModeKeepShowing:
		m.setPropHideState(HideStateShow)

	case HideModeKeepHidden:
		m.setPropHideState(HideStateHide)

	case HideModeSmartHide:
		if delay {
			m.resetSmartHideModeTimer(time.Millisecond * 400)
		} else {
			m.resetSmartHideModeTimer(0)
		}
	}
}

func (m *Manager) setPropHideState(hideState HideStateType) {
	logger.Debug("setPropHideState", hideState)
	if hideState == HideStateUnknown {
		logger.Warning("try setPropHideState to Unknown")
		return
	}

	m.PropsMu.Lock()
	if m.HideState != hideState {
		logger.Debugf("HideState %v => %v", m.HideState, hideState)
		m.HideState = hideState
		_ = m.service.EmitPropertyChanged(m, "HideState", m.HideState)
	}
	m.PropsMu.Unlock()
}

func (m *Manager) isWindowDockOverlapK(winInfo *KWindowInfo) (bool, error) {
	geo := winInfo.geometry
	winRect := &Rect{
		X:      geo.X,
		Y:      geo.Y,
		Width:  uint32(geo.Width),
		Height: uint32(geo.Height),
	}
	logger.Debugf("window [%s] rect: %v", winInfo.appId, winRect)
	logger.Debug("dock rect:", m.FrontendWindowRect)

	isActiveWin, err := winInfo.winObj.IsActive(0)
	if err != nil {
		logger.Warning(err)
		return false, nil
	}

	if !isActiveWin {
		logger.Debugf("check window [%s] InActive && return isWindowDockOverlapK false", winInfo.appId)
		return false, nil
	}

	if winInfo.appId == "dde-desktop" ||
		winInfo.appId == "dde-lock" ||
		winInfo.appId == "dde-shutdown" {
		logger.Debug("Active Window is dde-desktop/dde-lock/dde-shutdowm && return isWindowDockOverlapK false")
		return false, nil
	}

	return hasIntersection(winRect, m.FrontendWindowRect), nil
}
