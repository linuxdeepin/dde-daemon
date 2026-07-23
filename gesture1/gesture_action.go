// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package gesture1

import (
	"fmt"
	"os/exec"
)

const wmActionShowWorkspace int32 = 1

func (m *Manager) toggleShowMultiTasking() error {
	return m.wm.PerformAction(0, wmActionShowWorkspace)
}

func (m *Manager) doXdotoolsMouseDown() error {
	out, err := exec.Command("xdotool", "mousedown", "3").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", string(out))
	}
	return nil
}

func (m *Manager) doXdotoolsMouseUp() error {
	out, err := exec.Command("xdotool", "mouseup", "3").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", string(out))
	}
	return nil
}

func (m *Manager) showMultiTaskingWithLaunchType(launchType string) error {
	isShowMultiTask, err := m.wm.GetMultiTaskingStatus(0)
	if err != nil {
		return err
	}
	if !isShowMultiTask {
		if err := m.toggleShowMultiTasking(); err != nil {
			return err
		}
		logMultiTaskViewEvent(launchType)
	}
	return nil
}
