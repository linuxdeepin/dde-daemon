// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package gesture

import (
	"os/exec"
)

const (
	wmActionShowWorkspace int32 = iota + 1
	wmActionToggleMaximize
	wmActionMinimize
	wmActionShowWindow    = 6
	wmActionShowAllWindow = 7
)

const (
	wmTileDirectionLeft uint32 = iota + 1
	wmTileDirectionRight
)

func (m *Manager) initBuiltinSets() {
	m.builtinSets = map[string]func() error{
		"ShowWorkspace":              m.toggleShowMultiTasking,
		"Handle4Or5FingersSwipeUp":   m.doHandle4Or5FingersSwipeUp,
		"Handle4Or5FingersSwipeDown": m.doHandle4Or5FingersSwipeDown,
		"ToggleMaximize":             m.doToggleMaximize,
		"Minimize":                   m.doMinimize,
		"ShowWindow":                 m.doShowWindow,
		"ShowAllWindow":              m.doShowAllWindow,
		"SwitchApplication":          m.doSwitchApplication,
		"ReverseSwitchApplication":   m.doReverseSwitchApplication,
		"SwitchWorkspace":            m.doSwitchWorkspace,
		"ReverseSwitchWorkspace":     m.doReverseSwitchWorkspace,
		"SplitWindowLeft":            m.doTileActiveWindowLeft,
		"SplitWindowRight":           m.doTileActiveWindowRight,
		"MoveWindow":                 m.doMoveActiveWindow,
	}
}

func (m *Manager) toggleShowDesktop() error {
	return exec.Command("/usr/lib/deepin-daemon/desktop-toggle").Run()
}

func (m *Manager) toggleShowMultiTasking() error {
	return m.wm.PerformAction(0, wmActionShowWorkspace)
}

func (m *Manager) getWmStates() (bool, bool, error) {
	isShowDesktop, err := m.wm.GetIsShowDesktop(0)
	if err != nil {
		return false, false, err
	}
	isShowMultiTask, err := m.wm.GetMultiTaskingStatus(0)
	if err != nil {
		return false, false, err
	}

	return isShowDesktop, isShowMultiTask, nil
}

func (m *Manager) doHandle4Or5FingersSwipeUp() error {
	isShowDesktop, isShowMultiTask, err := m.getWmStates()
	if err != nil {
		return err
	}

	if !isShowMultiTask {
		if !isShowDesktop {
			return m.toggleShowMultiTasking()
		}
		return m.toggleShowDesktop()
	}

	return nil
}

func (m *Manager) doHandle4Or5FingersSwipeDown() error {
	isShowDesktop, isShowMultiTask, err := m.getWmStates()
	if err != nil {
		return err
	}

	if isShowMultiTask {
		return m.toggleShowMultiTasking()
	}
	if !isShowDesktop {
		return m.toggleShowDesktop()
	}
	return nil
}

func (m *Manager) doToggleMaximize() error {
	return m.wm.PerformAction(0, wmActionToggleMaximize)
}

func (m *Manager) doMinimize() error {
	return m.wm.PerformAction(0, wmActionMinimize)
}

func (m *Manager) doShowWindow() error {
	return m.wm.PerformAction(0, wmActionShowWindow)
}

func (m *Manager) doShowAllWindow() error {
	return m.wm.PerformAction(0, wmActionShowAllWindow)
}

func (m *Manager) doSwitchApplication() error {
	return m.wm.SwitchApplication(0, false)
}

func (m *Manager) doReverseSwitchApplication() error {
	return m.wm.SwitchApplication(0, true)
}

func (m *Manager) doSwitchWorkspace() error {
	return m.wm.SwitchToWorkspace(0, false)
}

func (m *Manager) doReverseSwitchWorkspace() error {
	return m.wm.SwitchToWorkspace(0, true)
}

func (m *Manager) doTileActiveWindowLeft() error {
	return m.wm.TileActiveWindow(0, wmTileDirectionLeft)
}

func (m *Manager) doTileActiveWindowRight() error {
	return m.wm.TileActiveWindow(0, wmTileDirectionRight)
}

func (m *Manager) doMoveActiveWindow() error {
	return m.wm.BeginToMoveActiveWindow(0)
}
