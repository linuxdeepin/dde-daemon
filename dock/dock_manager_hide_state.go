// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
	x, y, w, h := rectA.Pieces()
	x1, y1, w1, h1 := rectB.Pieces()
	ax := max(x, x1)
	ay := max(y, y1)
	bx := min(x+w, x1+w1)
	by := min(y+h, y1+h1)
	return ax < bx && ay < by
}

func (m *Manager) hasIntersectionK(rectA, rectB *Rect) bool {
	if rectA == nil || rectB == nil {
		logger.Warning("hasIntersectionK rectA or rectB is nil")
		return false
	}
	x, y, w, h := rectA.Pieces()
	x1, y1, w1, h1 := rectB.Pieces()
	ax := max(x, x1)
	ay := max(y, y1)
	bx := min(x+w, x1+w1)
	by := min(y+h, y1+h1)
	positionVal := m.Position.Get()
	logger.Debug("positionVal=", positionVal)
	if positionVal == int32(positionRight) || positionVal == int32(positionLeft) {
		return ax <= bx && ay < by
	} else if positionVal == int32(positionTop) || positionVal == int32(positionBottom) {
		return ax < bx && ay <= by
	} else {
		return ax < bx && ay < by
	}
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

	// 与dock区域完全重合的情况认为就是dock本身
	if winRect.X == m.FrontendWindowRect.X &&
		winRect.Y == m.FrontendWindowRect.Y &&
		winRect.Height == m.FrontendWindowRect.Height &&
		winRect.Width == m.FrontendWindowRect.Width {
		logger.Warning("FrontendWindowRect' geometry is the same as winRect' geometry")
		return false, nil
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

	isShowDesktop, err := ewmh.GetShowingDesktop(globalXConn).Reply(globalXConn)
	if err != nil {
		logger.Warning(err)
	}

	// 当显示桌面时，不去隐藏任务栏
	if isShowDesktop {
		return false, nil
	}

	list, err := ewmh.GetClientListStacking(globalXConn).Reply(globalXConn)
	if err != nil {
		logger.Warning(err)
	}

	logger.Debug("shouldHideOnSmartHideMode: current window stack list", list)

	// 当激活的窗口有变化时，去遍历当前的窗口栈中的窗口，而不只是激活的窗口，看是否有窗口和任务栏显示有重叠
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
	// 向窗管小伙伴讨教了一下，这里的currentDesktop - 1 后才和wayland下通过kwayland拿到的virtualDesktop相等
	currentDesktop, _ := m.kwin.CurrentDesktop(0)

	isShowDesktop, err := m.wm.GetIsShowDesktop(0)
	if err != nil {
		logger.Warning(err)
	}
	if isShowDesktop {
		return false, nil
	}

	for _, winInfo := range m.waylandManager.windows {
		// 最小化的窗口不用关心
		minimized, err := winInfo.winObj.IsMinimized(0)

		// 非当前工作区的窗口不用关心
		virtualDesktop, _ := winInfo.winObj.VirtualDesktop(0)

		// 一些置顶的窗口过滤掉，例如输入法的小窗口
		isKeepAbove, _ := winInfo.winObj.IsKeepAbove(0)
		if err != nil {
			logger.Warning(err)
			continue
		}

		if minimized || virtualDesktop != uint32(currentDesktop-1) || isKeepAbove {
			logger.Debugf("window %v is hidden or keepAbove or  not on current workspace", winInfo.appId)
			continue
		}

		hide, _ := m.isWindowDockOverlapK(winInfo)
		if hide {
			return true, nil
		}
	}

	return false, nil
}

func (m *Manager) shouldHideOnSmartHideMode() (bool, error) {
	if m.isMultiTaskViewShow {
		logger.Debug("shouldHideOnSmartHideMode: multitaskview is visible")
		return true, nil
	}

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

	if winInfo.appId == "dde-desktop" ||
		winInfo.appId == "dde-lock" ||
		winInfo.appId == "dde-shutdown" ||
		winInfo.appId == "reset-password-dialog" ||
		winInfo.appId == "deepin-screen-recorder" {
		logger.Debug("Active Window is dde-desktop/dde-lock/dde-shutdowm/deepin-screen-recorder && return isWindowDockOverlapK false")
		return false, nil
	}

	return m.hasIntersectionK(winRect, m.FrontendWindowRect), nil
}
