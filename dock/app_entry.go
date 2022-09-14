// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"sort"
	"sync"
	"unicode/utf8"

	"github.com/linuxdeepin/go-lib/dbusutil"
	x "github.com/linuxdeepin/go-x11-client"
)

const (
	entryDBusObjPathPrefix = dbusPath + "/entries/"
	entryDBusInterface     = dbusInterface + ".Entry"
)

//go:generate dbusutil-gen -type AppEntry -import=github.com/linuxdeepin/go-x11-client=x app_entry.go
//go:generate dbusutil-gen em -type AppEntry,Manager

type AppEntry struct {
	PropsMu  sync.RWMutex
	Id       string
	IsActive bool
	Name     string
	Icon     string
	// dbusutil-gen: ignore
	Menu          AppEntryMenu
	DesktopFile   string
	CurrentWindow x.Window
	IsDocked      bool
	// dbusutil-gen: equal=method:Equal
	WindowInfos windowInfosType

	service          *dbusutil.Service
	manager          *Manager
	innerId          string
	windows          map[x.Window]WindowInfoImp
	current          WindowInfoImp
	appInfo          *AppInfo
	winIconPreferred bool
}

func newAppEntry(dockManager *Manager, innerId string, appInfo *AppInfo) *AppEntry {
	entry := &AppEntry{
		manager: dockManager,
		service: dockManager.service,
		Id:      dockManager.allocEntryId(),
		innerId: innerId,
		windows: make(map[x.Window]WindowInfoImp),
	}
	entry.Menu.manager = dockManager
	entry.setAppInfo(appInfo)
	entry.Name = entry.getName()
	entry.Icon = entry.getIcon()
	return entry
}

func (entry *AppEntry) setAppInfo(newAppInfo *AppInfo) {
	if entry.appInfo == newAppInfo {
		logger.Debug("setAppInfo failed: old == new")
		return
	}
	entry.appInfo = newAppInfo

	if newAppInfo == nil {
		entry.winIconPreferred = true
		entry.setPropDesktopFile("")
	} else {
		entry.winIconPreferred = false
		entry.setPropDesktopFile(newAppInfo.GetFileName())
		id := newAppInfo.GetId()
		if strSliceContains(entry.manager.getWinIconPreferredApps(), id) {
			entry.winIconPreferred = true
			return
		}

		icon := newAppInfo.GetIcon()
		if icon == "" {
			entry.winIconPreferred = true
		}
	}
}

func (entry *AppEntry) hasWindow() bool {
	return len(entry.windows) != 0
}

func (entry *AppEntry) hasAllowedCloseWindow() bool {
	winInfos := entry.getAllowedCloseWindows()
	return len(winInfos) > 0
}

func (entry *AppEntry) getAllowedCloseWindows() []WindowInfoImp {
	ret := make([]WindowInfoImp, 0, len(entry.windows))
	for _, winInfo := range entry.windows {
		if winInfo.allowClose() {
			ret = append(ret, winInfo)
		}
	}
	return ret
}

func (entry *AppEntry) getWindowIds() []uint32 {
	list := make([]uint32, 0, len(entry.windows))
	for _, winInfo := range entry.windows {
		list = append(list, uint32(winInfo.getXid()))
	}
	return list
}

func (entry *AppEntry) getWindowInfoSlice() []WindowInfoImp {
	winInfoSlice := make([]WindowInfoImp, 0, len(entry.windows))
	for _, winInfo := range entry.windows {
		winInfoSlice = append(winInfoSlice, winInfo)
	}
	return winInfoSlice
}

func (entry *AppEntry) getExec(oneLine bool) string {
	if entry.current == nil {
		return ""
	}
	winProcess := entry.current.getProcess()
	if winProcess != nil {
		if oneLine {
			return winProcess.GetOneCommandLine()
		} else {
			return winProcess.GetShellScriptLines()
		}
	}
	return ""
}

func (entry *AppEntry) setCurrentWindowInfo(winInfo WindowInfoImp) {
	entry.current = winInfo
	if winInfo == nil {
		entry.setPropCurrentWindow(0)
	} else {
		entry.setPropCurrentWindow(winInfo.getXid())
	}
}

func (entry *AppEntry) findNextLeader() WindowInfoImp {
	winSlice := make(windowSlice, 0, len(entry.windows))
	for win := range entry.windows {
		winSlice = append(winSlice, win)
	}
	sort.Sort(winSlice)
	currentWin := entry.current.getXid()
	logger.Debug("sorted window slice:", winSlice)
	logger.Debug("current window:", currentWin)
	currentIndex := -1
	for i, win := range winSlice {
		if win == currentWin {
			currentIndex = i
		}
	}
	if currentIndex == -1 {
		logger.Warning("findNextLeader unexpect, return 0")
		return nil
	}
	// if current window is max, return min: winSlice[0]
	// else return winSlice[currentIndex+1]
	nextIndex := 0
	if currentIndex < len(winSlice)-1 {
		nextIndex = currentIndex + 1
	}
	logger.Debug("next window:", winSlice[nextIndex])
	for _, winInfo := range entry.windows {
		if winInfo.getXid() == winSlice[nextIndex] {
			return winInfo
		}
	}
	return nil
}

func (entry *AppEntry) attachWindow(winInfo WindowInfoImp) bool {
	win := winInfo.getXid()
	logger.Debugf("attach win %v to entry", win)

	winInfo.setEntry(entry)

	entry.PropsMu.Lock()
	defer entry.PropsMu.Unlock()

	if _, ok := entry.windows[win]; ok {
		logger.Infof("win %v is already attach to entry", win)
		return false
	}

	entry.windows[win] = winInfo
	entry.updateWindowInfos()
	entry.updateIsActive()

	if entry.current == nil {
		// from no window to has window
		entry.setCurrentWindowInfo(winInfo)
	}
	entry.updateIcon()
	entry.updateMenu()

	// print window info
	winInfo.print()

	return true
}

// return need remove?
func (entry *AppEntry) detachWindow(winInfo WindowInfoImp) bool {
	winInfo.setEntry(nil)
	win := winInfo.getXid()
	logger.Debug("detach window ", win)

	entry.PropsMu.Lock()
	defer entry.PropsMu.Unlock()

	delete(entry.windows, win)

	if len(entry.windows) == 0 {
		if !entry.IsDocked {
			// no window and not docked
			return true
		}
		entry.setCurrentWindowInfo(nil)
	} else {
		for _, winInfo := range entry.windows {
			// select first
			entry.setCurrentWindowInfo(winInfo)
			break
		}
	}

	entry.updateWindowInfos()
	entry.updateIcon()
	entry.updateIsActive()
	entry.updateMenu()
	return false
}

func (entry *AppEntry) getName() (name string) {
	if entry.appInfo != nil {
		name = entry.appInfo.name
		if !utf8.ValidString(name) {
			name = ""
		}
	}

	if name == "" && entry.current != nil {
		name = entry.current.getDisplayName()
	}

	return
}

func (entry *AppEntry) updateName() {
	name := entry.getName()
	entry.setPropName(name)
}

func (entry *AppEntry) updateIcon() {
	icon := entry.getIcon()
	entry.setPropIcon(icon)
}

func (entry *AppEntry) forceUpdateIcon() {
	icon := entry.getIcon()
	entry.Icon = icon
	err := entry.emitPropChangedIcon(icon)
	if err != nil {
		logger.Warning(err)
	}
}

func (entry *AppEntry) getIcon() string {
	var icon string
	appInfo := entry.appInfo
	current := entry.current

	if entry.hasWindow() {
		if current == nil {
			logger.Warning("AppEntry.getIcon entry.hasWindow but entry.current is nil")
			return ""
		}

		// has window && current not nil
		if entry.winIconPreferred {
			// try current window icon first
			icon = current.getIcon()
			if icon != "" {
				return icon
			}
		}
		if appInfo != nil {
			icon = appInfo.GetIcon()
			if icon != "" {
				return icon
			}
		}
		return current.getIcon()

	} else if appInfo != nil {
		// no window
		return appInfo.GetIcon()
	}
	return ""
}

func (e *AppEntry) updateWindowInfos() {
	windowInfos := newWindowInfos()
	for _, winInfo := range e.windows {
		windowInfos[winInfo.getXid()] = ExportWindowInfo{
			Title: winInfo.getTitle(),
			Flash: winInfo.isDemandingAttention(),
		}
	}
	e.setPropWindowInfos(windowInfos)
}

func (e *AppEntry) updateIsActive() {
	isActive := false
	activeWin := e.manager.getActiveWindow()
	if activeWin != nil {
		_, isActive = e.windows[activeWin.getXid()]
	}
	e.setPropIsActive(isActive)
}
