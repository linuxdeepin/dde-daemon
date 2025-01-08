// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package gesture1

import (
	"fmt"
	"github.com/linuxdeepin/go-lib/gettext"
	"os/exec"
)

type actionInfo struct {
	Name        string
	Description string
	fn          func() error
}

type actionInfos []*actionInfo

var actions []*actionInfo

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

func (m *Manager) initActions() {
	actions = []*actionInfo{
		{"MaximizeWindow", gettext.Tr("Maximize Window"), m.doToggleMaximize},
		{"RestoreWindow", gettext.Tr("Restore Window"), m.doToggleMaximize},
		{"SplitWindowLeft", gettext.Tr("Current Window Left Split"), m.doTileActiveWindowLeft},
		{"SplitWindowRight", gettext.Tr("Current Window Right Split"), m.doTileActiveWindowRight},
		{"ShowMultiTask", gettext.Tr("Show multitasking view"), m.doShowMultiTasking},
		{"HideMultitask", gettext.Tr("Hide multitasking view"), m.doHideMultiTasking},
		{"SwitchToPreDesktop", gettext.Tr("Switch to previous desktop"), m.doPreviousWorkspace},
		{"SwitchToNextDesktop", gettext.Tr("Switch to the next desktop"), m.doNextWorkspace},
		{"ShowDesktop", gettext.Tr("Show desktop"), m.toggleShowDesktop},
		{"HideDesktop", gettext.Tr("Hide desktop"), m.toggleShowDesktop},
		{"ToggleLaunchPad", gettext.Tr("Show/hide launcher"), m.doToggleLaunchpad},
		{"MouseRightButtonDown", gettext.Tr("Mouse right button pressed"), m.doXdotoolsMouseDown},
		{"MouseRightButtonUp", gettext.Tr("Mouse right button released"), m.doXdotoolsMouseUp},
		{"ToggleClipboard", gettext.Tr("Show/hide clipboard"), m.doToggleClipboard},
		{"ToggleGrandSearch", gettext.Tr("Show/hide grand search"), m.doToggleGrandSearch},
		{"ToggleNotifications", gettext.Tr("Show/hide notificaion center"), m.doToggleNotifications},
		{"Disable", gettext.Tr("Disable"), nil},
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

func (m *Manager) doNextWorkspace() error {
	return m.wm.NextWorkspace(0)
}

func (m *Manager) doPreviousWorkspace() error {
	return m.wm.PreviousWorkspace(0)
}

func (m *Manager) doToggleLaunchpad() error {
	return m.launchpad.Toggle(0)
}

func (m *Manager) doXdotoolsMouseDown() error {
	cmd := "xdotool mousedown 3"
	out, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", string(out))
	}
	return nil
}

func (m *Manager) doXdotoolsMouseUp() error {
	cmd := "xdotool mousedown 3"
	out, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", string(out))
	}
	return nil
}

func (m *Manager) doShowMultiTasking() error {
	isShowMultiTask, err := m.wm.GetMultiTaskingStatus(0)
	if err != nil {
		return err
	}
	if !isShowMultiTask {
		return m.toggleShowMultiTasking()
	}
	return nil
}

func (m *Manager) doHideMultiTasking() error {
	isShowMultiTask, err := m.wm.GetMultiTaskingStatus(0)
	if err != nil {
		return err
	}
	if isShowMultiTask {
		return m.toggleShowMultiTasking()
	}
	return nil
}

func (m *Manager) doToggleClipboard() error {
	return m.clipboard.Toggle(0)
}

func (m *Manager) doToggleGrandSearch() error {
	cmd := "/usr/libexec/dde-daemon/keybinding/shortcut-dde-grand-search.sh"
	return exec.Command("/bin/sh", "-c", cmd).Run()
}

func (m *Manager) doToggleNotifications() error {
	cmd := "dbus-send --print-reply --dest=org.deepin.dde.Osd1 /org/deepin/dde/shell/notification/center org.deepin.dde.shell.notification.center.Toggle"
	return exec.Command("/bin/bash", "-c", cmd).Run()
}
