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
	"fmt"
	"sort"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/go-x11-client/util/wm/ewmh"
	"github.com/linuxdeepin/dde-daemon/session/common"
)

func (m *Manager) allocEntryId() string {
	m.PropsMu.Lock()

	num := m.entryCount
	m.entryCount++

	m.PropsMu.Unlock()

	return fmt.Sprintf("e%dT%x", num, getCurrentTimestamp())
}

func (m *Manager) markAppLaunched(appInfo *AppInfo) {
	if !m.clientListInitEnd || appInfo == nil {
		return
	}
	file := appInfo.GetFileName()
	logger.Debug("markAppLaunched", file)

	go func() {
		err := common.ActivateSysDaemonService(m.appsObj.ServiceName_())
		if err != nil {
			logger.Warning(err)
		}

		err = m.appsObj.LaunchedRecorder().MarkLaunched(0, file)
		if err != nil {
			logger.Debug(err)
		}
	}()
}

func (m *Manager) shouldShowOnDock(winInfo WindowInfoImp) bool {
	switch winInfo.(type) {
	case *WindowInfo:
		win := winInfo.getXid()
		isReg := m.isWindowRegistered(win)
		clientListContains := m.clientList.Contains(win)
		shouldSkip := winInfo.shouldSkip()
		isGood := isGoodWindow(win)
		logger.Debugf("isReg: %v, client list contains: %v, shouldSkip: %v, isGood: %v",
			isReg, clientListContains, shouldSkip, isGood)

		showOnDock := isReg && clientListContains && isGood && !shouldSkip
		return showOnDock

	case *KWindowInfo:
		return !winInfo.shouldSkip()
	default:
		return false
	}
}

func (m *Manager) attachOrDetachWindow(winInfo WindowInfoImp) {
	win := winInfo.getXid()
	showOnDock := m.shouldShowOnDock(winInfo)
	logger.Debugf("win %v showOnDock? %v", win, showOnDock)

	entry := winInfo.getEntry()
	if entry != nil {
		if !showOnDock {
			m.detachWindow(winInfo)
		} else {
			logger.Debugf("win %v nothing to do", win)
		}
	} else {
		if winInfo.getEntryInnerId() == "" {
			logger.Debugf("winInfo.entryInnerId is empty, call identifyWindow, win: %d", winInfo.getXid())
			entryInnerId, appInfo := m.identifyWindow(winInfo)
			winInfo.setEntryInnerId(entryInnerId)
			winInfo.setAppInfo(appInfo)
			m.markAppLaunched(appInfo)
		} else {
			logger.Debugf("win %v identified", win)
		}

		if showOnDock {
			m.attachWindow(winInfo)
		}
	}
}

func (m *Manager) initClientList() {
	clientList, err := ewmh.GetClientList(globalXConn).Reply(globalXConn)
	if err != nil {
		logger.Warning("Get client list failed:", err)
		return
	}
	winSlice := windowSlice(clientList)
	sort.Sort(winSlice)
	m.clientList = winSlice
	for _, win := range winSlice {
		winInfo := m.registerWindow(win)
		m.attachOrDetachWindow(winInfo)
	}
}

func (m *Manager) initDockedApps() {
	dockedApps := uniqStrSlice(m.DockedApps.Get())
	for _, app := range dockedApps {
		m.appendDockedApp(app)
	}
	m.saveDockedApps()
}

func (m *Manager) exportAppEntry(e *AppEntry) error {
	err := m.service.Export(dbus.ObjectPath(entryDBusObjPathPrefix+e.Id), e)
	if err != nil {
		logger.Warning("failed to export AppEntry:", err)
		return err
	}
	return nil
}

func (m *Manager) appendDockedApp(app string) {
	logger.Debugf("appendDockedApp %q", app)
	appInfo := NewDockedAppInfo(app)
	if appInfo == nil {
		logger.Warning("appendDockedApp failed: appInfo is nil")
		return
	}

	entry := newAppEntry(m, appInfo.innerId, appInfo)
	entry.setPropIsDocked(true)
	entry.updateMenu()
	err := m.exportAppEntry(entry)
	if err == nil {
		m.Entries.Append(entry)
	}
}

func (m *Manager) removeAppEntry(e *AppEntry) {
	if e == nil {
		return
	}
	logger.Info("removeAppEntry id:", e.Id)
	m.Entries.Remove(e)
}

func (m *Manager) attachWindow(winInfo WindowInfoImp) {
	entry := m.Entries.GetByInnerId(winInfo.getEntryInnerId())

	if entry != nil {
		// existed
		entry.attachWindow(winInfo)
	} else {
		entry = newAppEntry(m, winInfo.getEntryInnerId(), winInfo.getAppInfo())
		ok := entry.attachWindow(winInfo)
		if ok {
			err := m.exportAppEntry(entry)
			if err == nil {
				m.Entries.Append(entry)
			}
		}
	}
}

func (m *Manager) detachWindow(winInfo WindowInfoImp) {
	entry := m.Entries.getByWindowId(winInfo.getXid())
	if entry == nil {
		logger.Warningf("entry of window %d is nil", winInfo.getXid())
		return
	}
	needRemove := entry.detachWindow(winInfo)
	if needRemove {
		m.removeAppEntry(entry)
	}
}
