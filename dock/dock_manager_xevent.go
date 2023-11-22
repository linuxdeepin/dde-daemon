// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"fmt"
	"sort"
	"strings"
	"time"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/wm/ewmh"
)

func (m *Manager) registerWindow(win x.Window) WindowInfoImp {
	logger.Debug("register window", win)

	m.windowInfoMapMutex.RLock()
	winInfo, ok := m.windowInfoMap[win]
	m.windowInfoMapMutex.RUnlock()
	if ok {
		logger.Debugf("register window %v failed, window existed", win)
		return winInfo
	}

	xwinInfo := NewWindowInfo(win)
	m.listenWindowXEvent(xwinInfo)

	m.windowInfoMapMutex.Lock()
	m.windowInfoMap[win] = xwinInfo
	m.windowInfoMapMutex.Unlock()
	return xwinInfo
}

func (m *Manager) isWindowRegistered(win x.Window) bool {
	m.windowInfoMapMutex.RLock()
	_, ok := m.windowInfoMap[win]
	m.windowInfoMapMutex.RUnlock()
	return ok
}

func (m *Manager) unregisterWindow(win x.Window) {
	logger.Debugf("unregister window %v", win)
	m.windowInfoMapMutex.Lock()
	delete(m.windowInfoMap, win)
	m.windowInfoMapMutex.Unlock()
}

func (m *Manager) handleClientListChanged() {
	clientList, err := ewmh.GetClientList(globalXConn).Reply(globalXConn)
	if err != nil {
		logger.Warning("Get client list failed:", err)
		return
	}
	newClientList := windowSlice(clientList)
	sort.Sort(newClientList)
	add, remove := diffSortedWindowSlice(m.clientList, newClientList)
	m.clientList = newClientList

	if len(add) > 0 {
		for _, win := range add {
			window0 := win
			addFunc := func() {
				logger.Debugf("client list add: %d", window0)
				winInfo := m.registerWindow(window0)
				repeatCount := 0
				for {
					if repeatCount > 10 {
						logger.Debugf("give up identify window %d", window0)
						return
					}
					good := isGoodWindow(window0)
					if !good {
						return
					}
					pid := getWmPid(window0)
					wmClass, _ := getWmClass(window0)
					wmName := getWmName(window0)
					wmCmd, _ := getWmCommand(window0)
					if pid != 0 || wmClass != nil || wmName != "" || strings.Join(wmCmd, "") != "" {
						m.attachOrDetachWindow(winInfo)
						return
					}
					repeatCount++
					time.Sleep(100 * time.Millisecond)
				}
			}
			m.entryDealChan <- addFunc
		}
	}

	if len(remove) > 0 {
		for _, win := range remove {
			window0 := win
			removeFunc := func() {
				logger.Debugf("client list remove: %d", window0)
				m.windowInfoMapMutex.RLock()
				winInfo := m.windowInfoMap[window0]
				m.windowInfoMapMutex.RUnlock()
				if winInfo != nil {
					m.detachWindow(winInfo)
					winInfo.setEntryInnerId("")
				} else {
					logger.Warningf("window info of %d is nil", window0)
					entry := m.Entries.getByWindowId(window0)
					if entry != nil {
						entry.PropsMu.RLock()
						if !entry.IsDocked {
							m.removeAppEntry(entry)
						}
						entry.PropsMu.RUnlock()
					}
				}
			}
			m.entryDealChan <- removeFunc
		}
	}
}

func (m *Manager) handleActiveWindowChangedX() {
	activeWindow, err := ewmh.GetActiveWindow(globalXConn).Reply(globalXConn)
	if err != nil {
		logger.Warning(err)
		return
	}
	winInfo := m.findWindowByXid(activeWindow)

	logger.Debug("Active window changed X", activeWindow)
	m.handleActiveWindowChanged(winInfo)
}

func (m *Manager) handleActiveWindowChanged(activeWindow WindowInfoImp) {
	m.activeWindowMu.Lock()
	if activeWindow == nil {
		m.activeWindowOld = m.activeWindow
		m.activeWindow = nil
		m.activeWindowMu.Unlock()
		return
	}

	m.activeWindow = activeWindow
	m.activeWindowMu.Unlock()

	activeWinXid := activeWindow.getXid()

	m.Entries.mu.RLock()
	for _, entry := range m.Entries.items {
		entry.PropsMu.Lock()

		winInfo, ok := entry.windows[activeWinXid]
		if ok {
			entry.setPropIsActive(true)
			entry.setCurrentWindowInfo(winInfo)
			entry.updateName()
			entry.updateIcon()
		} else {
			entry.setPropIsActive(false)
		}

		entry.PropsMu.Unlock()
	}
	m.Entries.mu.RUnlock()

	isShowDesktop := false
	var err error

	switch activeWindow.(type) {
	case *WindowInfo:
		isShowDesktop, err = ewmh.GetShowingDesktop(globalXConn).Reply(globalXConn)
	case *KWindowInfo:
		isShowDesktop, err = m.wm.GetIsShowDesktop(0)
	default:
		logger.Warning("invalid type WindowInfo")
	}

	if err != nil {
		logger.Warning(err)
	}

	m.updateHideState(!isShowDesktop)
}

func (m *Manager) listenRootWindowXEvent() {
	const eventMask = x.EventMaskPropertyChange | x.EventMaskSubstructureNotify
	err := x.ChangeWindowAttributesChecked(globalXConn, m.rootWindow, x.CWEventMask,
		[]uint32{eventMask}).Check(globalXConn)
	if err != nil {
		logger.Warning(err)
	}
	m.handleActiveWindowChangedX()
	m.handleClientListChanged()
}

func (m *Manager) listenWindowXEvent(winInfo *WindowInfo) {
	const eventMask = x.EventMaskPropertyChange | x.EventMaskStructureNotify | x.EventMaskVisibilityChange
	err := x.ChangeWindowAttributesChecked(globalXConn, winInfo.xid, x.CWEventMask,
		[]uint32{eventMask}).Check(globalXConn)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) handleDestroyNotifyEvent(ev *x.DestroyNotifyEvent) {
	logger.Debug("DestroyNotifyEvent window:", ev.Window)
	winInfo, err := m.getWindowInfo(ev.Window)
	if err == nil {
		m.detachWindow(winInfo)
	}

	m.unregisterWindow(ev.Window)
}

func (m *Manager) handleMapNotifyEvent(ev *x.MapNotifyEvent) {
	logger.Debug("MapNotifyEvent window:", ev.Window)
	winInfo := m.registerWindow(ev.Window)
	time.AfterFunc(2*time.Second, func() {
		logger.Warningf("mapNotifyEvent after 2s, call identifyWindow, win: %d", winInfo.getXid())
		_, appInfo := m.identifyWindow(winInfo)
		m.markAppLaunched(appInfo)
	})
}

func (m *Manager) getWindowInfo(win x.Window) (WindowInfoImp, error) {
	m.windowInfoMapMutex.RLock()
	v, ok := m.windowInfoMap[win]
	if !ok {
		err := fmt.Errorf("can not get %d window info", win)
		m.windowInfoMapMutex.RUnlock()
		return nil, err
	}

	m.windowInfoMapMutex.RUnlock()
	return v, nil
}

func (m *Manager) handleConfigureNotifyEvent(ev *x.ConfigureNotifyEvent) {
	winInfo := m.findXWindowInfo(ev.Window)
	if winInfo == nil {
		return
	}

	if HideModeType(m.HideMode.Get()) != HideModeSmartHide {
		return
	}
	if winInfo.wmClass != nil && winInfo.wmClass.Class == frontendWindowWmClass {
		// ignore frontend window ConfigureNotify event
		return
	}

	winInfo.mu.Lock()
	winInfo.lastConfigureNotifyEvent = ev
	winInfo.mu.Unlock()

	const configureNotifyDelay = 100 * time.Millisecond
	if winInfo.updateConfigureTimer != nil {
		winInfo.updateConfigureTimer.Reset(configureNotifyDelay)
	} else {
		winInfo.updateConfigureTimer = time.AfterFunc(configureNotifyDelay, func() {
			logger.Debug("ConfigureNotify: updateConfigureTimer expired")

			winInfo.mu.Lock()
			ev := winInfo.lastConfigureNotifyEvent
			winInfo.mu.Unlock()

			logger.Debugf("in closure: configure notify ev: %#v", ev)
			isXYWHChange := false
			if winInfo.x != ev.X {
				winInfo.x = ev.X
				isXYWHChange = true
			}

			if winInfo.y != ev.Y {
				winInfo.y = ev.Y
				isXYWHChange = true
			}

			if winInfo.width != ev.Width {
				winInfo.width = ev.Width
				isXYWHChange = true
			}

			if winInfo.height != ev.Height {
				winInfo.height = ev.Height
				isXYWHChange = true
			}
			logger.Debug("isXYWHChange", isXYWHChange)
			// if xywh changed ,update hide state without delay
			m.updateHideState(!isXYWHChange)
		})
	}

}

func (m *Manager) handleRootWindowPropertyNotifyEvent(ev *x.PropertyNotifyEvent) {
	switch ev.Atom {
	case atomNetClientList:
		m.handleClientListChanged()
	case atomNetActiveWindow:
		m.handleActiveWindowChangedX()
	case atomNetShowingDesktop:
		m.updateHideState(false)
	}
}

func (m *Manager) handlePropertyNotifyEvent(ev *x.PropertyNotifyEvent) {
	if ev.Window == m.rootWindow {
		m.handleRootWindowPropertyNotifyEvent(ev)
		return
	}

	winInfo := m.findXWindowInfo(ev.Window)
	if winInfo == nil {
		return
	}

	var newInnerId string
	var needAttachOrDetach bool

	switch ev.Atom {
	case atomNetWMState:
		winInfo.updateWmState()
		needAttachOrDetach = true

	case atomGtkApplicationId:
		winInfo.gtkAppId = getWindowGtkApplicationId(winInfo.xid)
		newInnerId = genInnerId(winInfo)

	case atomNetWmPid:
		winInfo.updateProcessInfo()
		newInnerId = genInnerId(winInfo)

	case atomNetWMName:
		winInfo.updateWmName()
		newInnerId = genInnerId(winInfo)

	case atomNetWMIcon:
		winInfo.updateIcon()

	case atomNetWmAllowedActions:
		winInfo.updateWmAllowedActions()

	case atomMotifWmHints:
		winInfo.updateMotifWmHints()

	case x.AtomWMClass:
		winInfo.updateWmClass()
		newInnerId = genInnerId(winInfo)
		needAttachOrDetach = true

	case atomXEmbedInfo:
		winInfo.updateHasXEmbedInfo()
		needAttachOrDetach = true

	case atomNetWMWindowType:
		winInfo.updateWmWindowType()
		needAttachOrDetach = true

	case x.AtomWMTransientFor:
		winInfo.updateHasWmTransientFor()
		needAttachOrDetach = true
	}

	if winInfo.updateCalled && newInnerId != "" && winInfo.innerId != newInnerId {
		// winInfo.innerId changed
		logger.Debugf("window %v innerId changed to %s", winInfo.xid, newInnerId)
		m.detachWindow(winInfo)
		winInfo.innerId = newInnerId
		winInfo.entryInnerId = ""
		needAttachOrDetach = true
	}

	if needAttachOrDetach {
		m.attachOrDetachWindow(winInfo)
	}

	entry := m.Entries.getByWindowId(ev.Window)
	if entry == nil {
		return
	}

	entry.PropsMu.Lock()
	defer entry.PropsMu.Unlock()

	switch ev.Atom {
	case atomNetWMState:
		entry.updateWindowInfos()

	case atomNetWMIcon:
		if entry.current == winInfo {
			entry.updateIcon()
		}

	case atomNetWMName:
		if entry.current == winInfo {
			entry.updateName()
		}
		entry.updateWindowInfos()

	case atomNetWmAllowedActions, atomMotifWmHints:
		entry.updateMenu()
	}
}

func (m *Manager) eventHandleLoop() {
	eventChan := make(chan x.GenericEvent, 500)
	globalXConn.AddEventChan(eventChan)

	for ev := range eventChan {
		switch ev.GetEventCode() {
		case x.MapNotifyEventCode:
			event, _ := x.NewMapNotifyEvent(ev)
			m.handleMapNotifyEvent(event)

		case x.DestroyNotifyEventCode:
			event, _ := x.NewDestroyNotifyEvent(ev)
			m.handleDestroyNotifyEvent(event)

		case x.ConfigureNotifyEventCode:
			event, _ := x.NewConfigureNotifyEvent(ev)
			m.handleConfigureNotifyEvent(event)

		case x.PropertyNotifyEventCode:
			event, _ := x.NewPropertyNotifyEvent(ev)
			m.handlePropertyNotifyEvent(event)
		}
	}
}

func (m *Manager) isActiveWindow(winInfo WindowInfoImp) bool {
	if winInfo == nil {
		return false
	}
	return winInfo == m.getActiveWindow()
}
