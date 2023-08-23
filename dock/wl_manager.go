// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"os"
	"strconv"
	"sync"

	dbus "github.com/godbus/dbus"
	kwayland "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.kwayland"
	x "github.com/linuxdeepin/go-x11-client"
)

type WaylandManager struct {
	mu      sync.Mutex
	windows map[dbus.ObjectPath]*KWindowInfo
}

func (wm *WaylandManager) handleActiveWindowChangedK(activeWin uint32) WindowInfoImp {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	logger.Debug("WaylandManager.handleActiveWindowChangedK", activeWin)

	for _, winInfo := range wm.windows {
		if winInfo.internalId == activeWin {
			return winInfo
		}
	}
	return nil
}

func newWaylandManager() *WaylandManager {
	m := &WaylandManager{
		windows: make(map[dbus.ObjectPath]*KWindowInfo),
	}
	return m
}

func (m *Manager) listenWaylandWMSignals() {
	m.waylandWM.InitSignalExt(m.sessionSigLoop, true)
	_, err := m.waylandWM.ConnectActiveWindowChanged(func() {
		activeWinInternalId, err := m.waylandWM.ActiveWindow(0)
		if err != nil {
			logger.Warning(err)
			return
		}
		activeWinInfo := m.waylandManager.handleActiveWindowChangedK(activeWinInternalId)
		if activeWinInfo != nil {
			m.handleActiveWindowChanged(activeWinInfo)
		} else {
			m.updateHideState(false)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	_, err = m.waylandWM.ConnectWindowCreated(func(objPathStr string) {
		objPath := dbus.ObjectPath(objPathStr)
		logger.Debug("window created", objPath)
		m.registerWindowWayland(objPath)
	})
	if err != nil {
		logger.Warning(err)
	}

	_, err = m.waylandWM.ConnectWindowRemove(func(objPathStr string) {
		objPath := dbus.ObjectPath(objPathStr)
		logger.Debug("window removed", objPath)
		m.unregisterWindowWayland(objPath)
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) listenKWindowSignals(winInfo *KWindowInfo) {
	winInfo.winObj.InitSignalExt(m.sessionSigLoop, true)
	var err error

	// Title changed
	_, err = winInfo.winObj.ConnectTitleChanged(func() {
		winInfo.updateTitle()
		entry := m.Entries.getByWindowId(winInfo.xid)
		if entry == nil {
			return
		}
		if entry.current == winInfo {
			entry.updateName()
		}
		entry.updateWindowInfos()
	})
	if err != nil {
		logger.Warning(err)
	}

	// Icon changed
	_, err = winInfo.winObj.ConnectIconChanged(func() {
		winInfo.updateIcon()
		entry := m.Entries.getByWindowId(winInfo.xid)
		if entry == nil {
			return
		}
		entry.updateIcon()
	})

	// DemandingAttention changed
	_, err = winInfo.winObj.ConnectDemandsAttentionChanged(func() {
		winInfo.updateDemandingAttention()
		entry := m.Entries.getByWindowId(winInfo.xid)
		if entry == nil {
			return
		}
		entry.updateWindowInfos()
	})

	// Geometry changed
	_, err = winInfo.winObj.ConnectGeometryChanged(func() {
		changed := winInfo.updateGeometry()
		if !changed {
			return
		}
		m.handleWindowGeometryChanged(winInfo)
	})
}

func (m *Manager) handleWindowGeometryChanged(winInfo WindowInfoImp) {
	if HideModeType(m.HideMode.Get()) != HideModeSmartHide {
		return
	}

	m.updateHideState(false)
}

func (m *Manager) unregisterWindowWayland(objPath dbus.ObjectPath) {
	logger.Debug("unregister window", objPath)

	m.waylandManager.mu.Lock()
	winInfo, ok := m.waylandManager.windows[objPath]
	m.waylandManager.mu.Unlock()
	if !ok {
		return
	}

	winInfo.winObj.RemoveAllHandlers()
	m.detachWindow(winInfo)

	err := globalXConn.FreeID(uint32(winInfo.xid))
	if err != nil {
		logger.Warning(err)
	}

	m.waylandManager.mu.Lock()
	delete(m.waylandManager.windows, objPath)
	m.waylandManager.mu.Unlock()

	// TODO 发现windowInfoMap中的内容没有被清除过
	// m.windowInfoMapMutex.Lock()
	// delete(m.windowInfoMap, winInfo.getXid())
	// m.windowInfoMapMutex.Unlock()
}

var globalRestrictWaylandWindow = true

func init() {
	if os.Getenv("DEEPIN_DOCK_RESTRICT_WAYLAND_WINDOW") == "0" {
		globalRestrictWaylandWindow = false
	}
}

// TODO: remove it
func (m *Manager) DebugRegisterWW(id uint32) *dbus.Error {
	objPath := dbus.ObjectPath("/com/deepin/daemon/KWayland/PlasmaWindow_" + strconv.Itoa(int(id)))
	m.registerWindowWayland(objPath)
	return nil
}

// TODO: remove it
func (m *Manager) DebugSetActiveWindow(id uint32) *dbus.Error {
	activeWinInfo := m.waylandManager.handleActiveWindowChangedK(id)
	if activeWinInfo != nil {
		m.handleActiveWindowChanged(activeWinInfo)
	}
	return nil
}

func (m *Manager) registerWindowWayland(objPath dbus.ObjectPath) {
	logger.Debug("register window", objPath)

	m.waylandManager.mu.Lock()
	_, ok := m.waylandManager.windows[objPath]
	m.waylandManager.mu.Unlock()
	if ok {
		return
	}

	sessionBus := m.sessionSigLoop.Conn()
	winObj, err := kwayland.NewWindow(sessionBus, objPath)
	if err != nil {
		logger.Warning(err)
		return
	}
	appId, err := winObj.AppId(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	if appId == "dde-dock" || appId == "dde-launcher" || appId == "dde-clipboard" ||
		appId == "dde-osd" || appId == "dde-polkit-agent" || appId == "dde-simple-egl" || appId == "dmcs" {
		return
	}

	xid, err := globalXConn.AllocID()
	if err != nil {
		logger.Warning(err)
		return
	}

	realWid, err := winObj.WindowId(0)
	if err != nil {
		logger.Warning(err)
		realWid = 0
	}
	if realWid != 0 {
		xid = realWid
	}

	winInfo := newKWindowInfo(winObj, xid)
	m.listenKWindowSignals(winInfo)

	m.waylandManager.mu.Lock()
	m.waylandManager.windows[objPath] = winInfo
	m.waylandManager.mu.Unlock()

	m.attachOrDetachWindow(winInfo)

	if realWid != 0 {
		m.windowInfoMapMutex.Lock()
		m.windowInfoMap[x.Window(realWid)] = winInfo
		m.windowInfoMapMutex.Unlock()
	}
}

func (m *Manager) registerKWaylandInfo(winInfo WindowInfoImp) {
	switch winInfo.(type) {
	case *KWindowInfo:
		break
	default:
		logger.Warningf("registerKWaylandInfo, not wayland, wid=%d", winInfo.getXid())
		return
	}

	m.windowInfoMapMutex.RLock()
	defer m.windowInfoMapMutex.RUnlock()

	winInfo, ok := m.windowInfoMap[winInfo.getXid()]
	if ok {
		logger.Warningf("registerKWaylandInfo, already registered, wid=%d", winInfo.getXid())
		return
	}
	m.windowInfoMap[winInfo.getXid()] = winInfo
}

func (m *Manager) initWaylandWindows() {
	windowPaths, err := m.waylandWM.Windows(0)
	if err != nil {
		logger.Warning(err)
	}
	for _, objPath := range windowPaths {
		objPathStr, ok := objPath.Value().(string)
		if !ok {
			continue
		}
		m.registerWindowWayland(dbus.ObjectPath(objPathStr))
	}
}
