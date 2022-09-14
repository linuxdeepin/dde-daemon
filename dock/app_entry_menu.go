// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"reflect"
	"sync"

	"github.com/godbus/dbus"
	. "github.com/linuxdeepin/go-lib/gettext"
	_ "github.com/linuxdeepin/go-x11-client"
)

func (entry *AppEntry) updateMenu() {
	logger.Debug("Update menu")
	menu := NewMenu()
	menu.AppendItem(entry.getMenuItemLaunch())

	desktopActionMenuItems := entry.getMenuItemDesktopActions()
	menu.AppendItem(desktopActionMenuItems...)
	hasWin := entry.hasWindow()
	if hasWin {
		menu.AppendItem(entry.getMenuItemAllWindows())
	}

	// menu item dock or undock
	logger.Debug(entry.Id, "Item docked?", entry.IsDocked)
	if entry.IsDocked {
		menu.AppendItem(entry.getMenuItemUndock())
	} else {
		menu.AppendItem(entry.getMenuItemDock())
	}

	if hasWin {
		if entry.manager.forceQuitAppStatus != forceQuitAppDisabled {
			if entry.appInfo != nil && entry.appInfo.identifyMethod == "Android" {
				menu.AppendItem(entry.getMenuItemForceQuitAndroid())
			} else {
				menu.AppendItem(entry.getMenuItemForceQuit())
			}
		}

		if entry.hasAllowedCloseWindow() {
			menu.AppendItem(entry.getMenuItemCloseAll())
		}
	}
	entry.Menu.setMenu(menu)
}

func (entry *AppEntry) getMenuItemDesktopActions() []*MenuItem {
	ai := entry.appInfo
	if ai == nil {
		return nil
	}

	var items []*MenuItem
	launchAction := func(action desktopAction) func(timestamp uint32) {
		return func(timestamp uint32) {
			logger.Debugf("launch action %+v", action)
			err := entry.manager.startManager.LaunchAppAction(dbus.FlagNoAutoStart,
				ai.GetFileName(), action.Section, timestamp)
			if err != nil {
				logger.Warning("launchAppAction failed:", err)
			}
		}
	}

	for _, action := range ai.GetActions() {
		item := NewMenuItem(action.Name, launchAction(action), true)
		items = append(items, item)
	}
	return items
}

func (entry *AppEntry) launchApp(timestamp uint32) {
	logger.Debug("launchApp timestamp:", timestamp)
	if entry.appInfo != nil {
		logger.Debug("Has AppInfo")
		entry.manager.launch(entry.appInfo.GetFileName(), timestamp, nil)
	} else {
		// TODO
		logger.Debug("not supported")
	}
}

func (entry *AppEntry) getMenuItemLaunch() *MenuItem {
	var itemName string
	if entry.hasWindow() {
		itemName = entry.getName()
	} else {
		itemName = Tr("Open")
	}
	logger.Debugf("getMenuItemLaunch, itemName: %q", itemName)
	return NewMenuItem(itemName, entry.launchApp, true)
}

func (entry *AppEntry) getMenuItemCloseAll() *MenuItem {
	return NewMenuItem(Tr("Close All"), func(timestamp uint32) {
		logger.Debug("Close All")
		entry.PropsMu.RLock()
		winInfos := entry.getAllowedCloseWindows()
		entry.PropsMu.RUnlock()

		for i := 0; i < len(winInfos)-1; i++ {
			for j := i + 1; j < len(winInfos); j++ {
				if winInfos[i].getCreatedTime() < winInfos[j].getCreatedTime() {
					winInfos[i], winInfos[j] = winInfos[j], winInfos[i]
				}
			}
		}

		for _, winInfo := range winInfos {
			logger.Debug("to close win, xid:", winInfo.getXid(), ", created time:", winInfo.getCreatedTime())
			err := winInfo.close(timestamp)
			if err != nil {
				logger.Warningf("failed to close window %d: %v", winInfo.getXid(), err)
			}
		}
	}, true)
}

func (entry *AppEntry) getMenuItemForceQuit() *MenuItem {
	active := entry.manager.forceQuitAppStatus != forceQuitAppDeactivated

	return NewMenuItem(Tr("Force Quit"), func(timestamp uint32) {
		logger.Debug("Force Quit")
		err := entry.ForceQuit()
		if err != nil {
			logger.Warning("ForceQuit error:", err)
		}
	}, active)
}

//dock栏上Android程序的Force Quit功能
func (entry *AppEntry) getMenuItemForceQuitAndroid() *MenuItem {
	active := entry.manager.forceQuitAppStatus != forceQuitAppDeactivated

	if entry.hasAllowedCloseWindow() {
		return NewMenuItem(Tr("Force Quit"), func(timestamp uint32) {
			logger.Debug("Force Quit")
			entry.PropsMu.RLock()
			winInfos := entry.getAllowedCloseWindows()
			entry.PropsMu.RUnlock()

			for _, winInfo := range winInfos {
				err := winInfo.close(timestamp)
				if err != nil {
					logger.Warningf("failed to close window %d: %v", winInfo.getXid(), err)
				}
			}
		}, active)
	}

	return NewMenuItem(Tr("Force Quit"), func(timestamp uint32) {}, true)
}

func (entry *AppEntry) getMenuItemDock() *MenuItem {
	return NewMenuItem(Tr("Dock"), func(uint32) {
		logger.Debug("menu action dock entry")
		err := entry.RequestDock()
		if err != nil {
			logger.Warning("RequestDock error:", err)
		}
	}, true)
}

func (entry *AppEntry) getMenuItemUndock() *MenuItem {
	return NewMenuItem(Tr("Undock"), func(uint32) {
		logger.Debug("menu action undock entry")
		err := entry.RequestUndock()
		if err != nil {
			logger.Warning("RequestUndock error:", err)
		}
	}, true)
}

func (entry *AppEntry) getMenuItemAllWindows() *MenuItem {
	menuItem := NewMenuItem(Tr("All Windows"), func(uint32) {
		logger.Debug("menu action all windows")
		err := entry.PresentWindows()
		if err != nil {
			logger.Warning("PresentWindows error:", err)
		}
	}, true)
	menuItem.hint = menuItemHintShowAllWindows
	return menuItem
}

type AppEntryMenu struct {
	manager *Manager
	cache   string
	is3DWM  bool
	dirty   bool
	menu    *Menu
	mu      sync.Mutex
}

func (m *AppEntryMenu) setMenu(menu *Menu) {
	m.mu.Lock()
	m.menu = menu
	m.dirty = true
	m.mu.Unlock()
}

func (m *AppEntryMenu) getMenu() *Menu {
	m.mu.Lock()
	ret := m.menu
	m.mu.Unlock()
	return ret
}

func (*AppEntryMenu) SetValue(val interface{}) (changed bool, err *dbus.Error) {
	// read only
	return
}

func (m *AppEntryMenu) GetValue() (val interface{}, err *dbus.Error) {
	is3DWM := m.manager.is3DWM()
	m.mu.Lock()
	if m.dirty || m.cache == "" || m.is3DWM != is3DWM {
		items := make([]*MenuItem, 0, len(m.menu.Items))
		for _, item := range m.menu.Items {
			if is3DWM || item.hint != menuItemHintShowAllWindows {
				items = append(items, item)
			}
		}
		menu := NewMenu()
		menu.Items = items
		m.cache = menu.GenerateJSON()
		m.dirty = false
		m.is3DWM = is3DWM
	}
	val = m.cache
	m.mu.Unlock()
	return
}

func (*AppEntryMenu) SetNotifyChangedFunc(func(val interface{})) {
}

func (*AppEntryMenu) GetType() reflect.Type {
	return reflect.TypeOf("")
}
