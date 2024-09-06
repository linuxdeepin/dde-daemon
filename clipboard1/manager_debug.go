// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package clipboard

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	x "github.com/linuxdeepin/go-x11-client"
)

func (m *Manager) saveClipboard() error {
	owner, err := m.xc.GetSelectionOwner(atomClipboard)
	if err != nil {
		return err
	}

	logger.Debug("clipboard selection owner:", owner)

	ts, err := m.getTimestamp()
	if err != nil {
		return err
	}

	targets, err := m.getClipboardTargets(ts)
	if err != nil {
		return err
	}
	logger.Debug("targets:", targets)

	targetDataMap := m.saveTargets(targets, ts)
	m.setContent(targetDataMap)
	m.contentMu.Lock()
	for _, targetData := range m.content {
		logger.Debugf("target %d type: %v", targetData.Target, targetData.Type)
	}
	m.contentMu.Unlock()

	return nil
}

func (m *Manager) SaveClipboard() *dbus.Error {
	err := m.saveClipboard()
	return dbusutil.ToError(err)
}

func (m *Manager) writeContent() error {
	dir := "/tmp/dde-session-daemon-clipboard"

	err := os.Mkdir(dir, 0700)
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	err = emptyDir(dir)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	m.contentMu.Lock()
	for _, targetData := range m.content {
		target := targetData.Target
		targetName, _ := m.xc.GetAtomName(target)
		_, err = fmt.Fprintf(&buf, "%d,%s\n", target, targetName)
		if err != nil {
			m.contentMu.Unlock()
			return err
		}

		err = os.WriteFile(filepath.Join(dir, strconv.Itoa(int(target))), targetData.Data, 0644)
		if err != nil {
			m.contentMu.Unlock()
			return err
		}
	}
	m.contentMu.Unlock()
	err = os.WriteFile(filepath.Join(dir, "index.txt"), buf.Bytes(), 0600)
	if err != nil {
		return err
	}

	return nil
}

func (m *Manager) WriteContent() *dbus.Error {
	err := m.writeContent()
	return dbusutil.ToError(err)
}

func (m *Manager) BecomeClipboardOwner() *dbus.Error {
	ts, err := m.getTimestamp()
	if err != nil {
		return dbusutil.ToError(err)
	}
	err = m.becomeClipboardOwner(ts)
	return dbusutil.ToError(err)
}

func (m *Manager) removeTarget(target x.Atom) {
	m.contentMu.Lock()
	newContent := make([]*TargetData, 0, len(m.content))
	for _, td := range m.content {
		if td.Target != target {
			newContent = append(newContent, td)
		}
	}
	m.content = newContent
	m.contentMu.Unlock()
}

func (m *Manager) RemoveTarget(target uint32) *dbus.Error {
	m.removeTarget(x.Atom(target))
	return nil
}
