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
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/godbus/dbus"
	libApps "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.apps"
	launcher "github.com/linuxdeepin/go-dbus-factory/com.deepin.dde.daemon.launcher"
	libDDELauncher "github.com/linuxdeepin/go-dbus-factory/com.deepin.dde.launcher"
	sessionmanager "github.com/linuxdeepin/go-dbus-factory/com.deepin.sessionmanager"
	wm "github.com/linuxdeepin/go-dbus-factory/com.deepin.wm"
	wmswitcher "github.com/linuxdeepin/go-dbus-factory/com.deepin.wmswitcher"
	x "github.com/linuxdeepin/go-x11-client"
	"pkg.deepin.io/dde/daemon/common/dsync"
	"pkg.deepin.io/gir/gio-2.0"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/dbusutil/gsprop"
	"pkg.deepin.io/lib/dbusutil/proxy"
)

type Manager struct {
	PropsMu             sync.RWMutex
	Entries             AppEntries
	HideMode            gsprop.Enum `prop:"access:rw"`
	DisplayMode         gsprop.Enum `prop:"access:rw"`
	Position            gsprop.Enum `prop:"access:rw"`
	IconSize            gsprop.Uint `prop:"access:rw"`
	ShowTimeout         gsprop.Uint `prop:"access:rw"`
	HideTimeout         gsprop.Uint `prop:"access:rw"`
	WindowSizeEfficient gsprop.Uint `prop:"access:rw"`
	WindowSizeFashion   gsprop.Uint `prop:"access:rw"`
	DockedApps          gsprop.Strv
	Opacity             gsprop.Double
	HideState           HideStateType
	FrontendWindowRect  *Rect

	service            *dbusutil.Service
	sessionSigLoop     *dbusutil.SignalLoop
	syncConfig         *dsync.Config
	clientList         windowSlice
	clientListInitEnd  bool
	windowInfoMap      map[x.Window]*WindowInfo
	windowInfoMapMutex sync.RWMutex
	settings           *gio.Settings
	appearanceSettings *gio.Settings
	pluginSettings     *pluginSettingsStorage

	entryDealChan   chan func()
	rootWindow      x.Window
	activeWindow    x.Window
	activeWindowOld x.Window
	activeWindowMu  sync.Mutex

	ddeLauncherVisible   bool
	ddeLauncherVisibleMu sync.Mutex

	smartHideModeTimer *time.Timer
	smartHideModeMutex sync.Mutex

	entryCount         uint
	identifyWindowFuns []*IdentifyWindowFunc
	windowPatterns     WindowPatterns

	// dbus objects:
	launcher     launcher.Launcher
	ddeLauncher  libDDELauncher.Launcher
	wm           wm.Wm
	appsObj      libApps.Apps
	startManager sessionmanager.StartManager
	wmSwitcher   wmswitcher.WMSwitcher
	wmName       string
	//nolint
	signals *struct {
		ServiceRestarted struct{}
		EntryAdded       struct {
			path  dbus.ObjectPath
			index int32
		}

		EntryRemoved struct {
			entryId string
		}

		PluginSettingsSynced  struct{}
		DockAppSettingsSynced struct{}
	}
}

const (
	dockSchema                     = "com.deepin.dde.dock"
	appearanceSchema               = "com.deepin.dde.appearance"
	settingKeyHideMode             = "hide-mode"
	settingKeyDisplayMode          = "display-mode"
	settingKeyPosition             = "position"
	settingKeyIconSize             = "icon-size"
	settingKeyDockedApps           = "docked-apps"
	settingKeyShowTimeout          = "show-timeout"
	settingKeyHideTimeout          = "hide-timeout"
	settingKeyWindowSizeFashion    = "window-size-fashion"
	settingKeyWindowSizeEfficient  = "window-size-efficient"
	settingKeyWinIconPreferredApps = "win-icon-preferred-apps"
	settingKeyOpacity              = "opacity"
	settingKeyPluginSettings       = "plugin-settings"

	frontendWindowWmClass = "dde-dock"

	dbusServiceName = "com.deepin.dde.daemon.Dock"
	dbusPath        = "/com/deepin/dde/daemon/Dock"
	dbusInterface   = dbusServiceName
)

func newManager(service *dbusutil.Service) (*Manager, error) {
	m := new(Manager)
	m.service = service
	err := m.init()
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) destroy() {
	if m.smartHideModeTimer != nil {
		m.smartHideModeTimer.Stop()
		m.smartHideModeTimer = nil
	}

	if m.settings != nil {
		m.settings.Unref()
		m.settings = nil
	}

	m.launcher.RemoveHandler(proxy.RemoveAllHandlers)
	m.ddeLauncher.RemoveHandler(proxy.RemoveAllHandlers)
	m.sessionSigLoop.Stop()
	m.syncConfig.Destroy()

	err := m.service.StopExport(m)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) launch(desktopFile string, timestamp uint32, files []string) {
	err := m.startManager.LaunchApp(dbus.FlagNoAutoStart, desktopFile, timestamp, files)
	if err != nil {
		logger.Warningf("launch %q failed: %v", desktopFile, err)
	}
}

// ActivateWindow会激活给定id的窗口，被激活的窗口通常会成为焦点窗口。
func (m *Manager) ActivateWindow(win uint32) *dbus.Error {
	err := activateWindow(x.Window(win))
	if err != nil {
		logger.Warning("Activate window failed:", err)
		return dbusutil.ToError(err)
	}
	return nil
}

// CloseWindow会将传入id的窗口关闭。
func (m *Manager) CloseWindow(win uint32) *dbus.Error {
	err := closeWindow(x.Window(win), 0)
	if err != nil {
		logger.Warning("Close window failed:", err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (m *Manager) MaximizeWindow(win uint32) *dbus.Error {
	err := m.ActivateWindow(win)
	if err != nil {
		return err
	}
	err1 := maximizeWindow(x.Window(win))
	if err1 != nil {
		logger.Warning("maximize window failed:", err)
		return dbusutil.ToError(err1)
	}
	return nil
}

func (m *Manager) MinimizeWindow(win uint32) *dbus.Error {
	err := minimizeWindow(x.Window(win))
	if err != nil {
		logger.Warning("minimize window failed:", err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (m *Manager) MakeWindowAbove(win uint32) *dbus.Error {
	err := m.ActivateWindow(win)
	if err != nil {
		return err
	}

	err1 := makeWindowAbove(x.Window(win))
	if err1 != nil {
		logger.Warning("make window above failed:", err)
		return dbusutil.ToError(err1)
	}
	return nil
}

func (m *Manager) MoveWindow(win uint32) *dbus.Error {
	err := m.ActivateWindow(win)
	if err != nil {
		return err
	}

	err1 := moveWindow(x.Window(win))
	if err1 != nil {
		logger.Warning("move window failed:", err)
		return dbusutil.ToError(err1)
	}
	return nil
}

func (m *Manager) PreviewWindow(win uint32) *dbus.Error {
	err := m.wm.PreviewWindow(dbus.FlagNoAutoStart, win)
	return dbusutil.ToError(err)
}

func (m *Manager) CancelPreviewWindow() *dbus.Error {
	err := m.wm.CancelPreviewWindow(dbus.FlagNoAutoStart)
	return dbusutil.ToError(err)
}

// for debug
func (m *Manager) GetEntryIDs() (list []string, busErr *dbus.Error) {
	entries := &m.Entries
	entries.mu.RLock()
	list = make([]string, 0, len(entries.items))
	for _, entry := range entries.items {
		var appId string
		if entry.appInfo != nil {
			appId = entry.appInfo.GetId()
		} else {
			appId = entry.innerId
		}
		list = append(list, appId)
	}
	entries.mu.RUnlock()
	return list, nil
}

func (m *Manager) SetFrontendWindowRect(x, y int32, width, height uint32) *dbus.Error {
	if m.FrontendWindowRect.X == x &&
		m.FrontendWindowRect.Y == y &&
		m.FrontendWindowRect.Width == width &&
		m.FrontendWindowRect.Height == height {
		logger.Debug("SetFrontendWindowRect no changed")
		return nil
	}
	m.FrontendWindowRect.X = x
	m.FrontendWindowRect.Y = y
	m.FrontendWindowRect.Width = width
	m.FrontendWindowRect.Height = height
	err := m.service.EmitPropertyChanged(m, "FrontendWindowRect", m.FrontendWindowRect)
	if err != nil {
		logger.Warning("EmitPropertyChanged error:", err)
	}
	m.updateHideState(false)
	return nil
}

func (m *Manager) IsDocked(desktopFile string) (docked bool, busErr *dbus.Error) {
	desktopFile = toLocalPath(desktopFile)
	entry, err := m.getDockedAppEntryByDesktopFilePath(desktopFile)
	if err != nil {
		return false, dbusutil.ToError(err)
	}
	return entry != nil, nil
}

func (m *Manager) requestDock(desktopFile string, index int32) (bool, error) {
	logger.Debug("requestDock", desktopFile, index)
	desktopFile = toLocalPath(desktopFile)
	appInfo := NewAppInfoFromFile(desktopFile)
	if appInfo == nil {
		return false, errors.New("invalid desktopFilePath")
	}
	var newlyCreated bool
	entry := m.Entries.GetByInnerId(appInfo.innerId)
	if entry == nil {
		entry = newAppEntry(m, appInfo.innerId, appInfo)
		newlyCreated = true
	}

	docked, err := m.dockEntry(entry)
	if err != nil {
		return false, err
	}

	if newlyCreated {
		err = m.exportAppEntry(entry)
		if err != nil {
			return false, err
		}
		m.Entries.Insert(entry, int(index))
	}

	if docked {
		// need to save after insert
		m.saveDockedApps()
	}
	return docked, nil
}

func (m *Manager) RequestDock(desktopFile string, index int32) (docked bool, busErr *dbus.Error) {
	docked, err := m.requestDock(desktopFile, index)
	return docked, dbusutil.ToError(err)
}

func (m *Manager) RequestUndock(desktopFile string) (undocked bool, busErr *dbus.Error) {
	undocked, err := m.requestUndock(desktopFile)
	return undocked, dbusutil.ToError(err)
}

func (m *Manager) requestUndock(desktopFile string) (bool, error) {
	desktopFile = toLocalPath(desktopFile)
	entry, err := m.getDockedAppEntryByDesktopFilePath(desktopFile)
	if err != nil {
		return false, err
	}
	if entry == nil {
		return false, nil
	}
	m.undockEntry(entry)
	return true, nil
}

func (m *Manager) MoveEntry(index, newIndex int32) *dbus.Error {
	err := m.Entries.Move(int(index), int(newIndex))
	if err != nil {
		logger.Warning("MoveEntry failed:", err)
		return dbusutil.ToError(err)
	}
	logger.Debug("MoveEntry ok")
	m.saveDockedApps()
	return nil
}

func (m *Manager) IsOnDock(desktopFile string) (onDock bool, busErr *dbus.Error) {
	desktopFile = toLocalPath(desktopFile)
	entry, err := m.Entries.GetByDesktopFilePath(desktopFile)
	if err != nil {
		return false, dbusutil.ToError(err)
	}
	return entry != nil, nil
}

func (m *Manager) QueryWindowIdentifyMethod(wid uint32) (method string, busErr *dbus.Error) {
	m.Entries.mu.RLock()
	defer m.Entries.mu.RUnlock()

	for _, entry := range m.Entries.items {
		winInfo, ok := entry.windows[x.Window(wid)]
		if ok {
			if winInfo.appInfo != nil {
				return winInfo.appInfo.identifyMethod, nil
			} else {
				return "Failed", nil
			}
		}
	}
	return "", dbusutil.ToError(fmt.Errorf("window %d not found", wid))
}

func (m *Manager) GetDockedAppsDesktopFiles() (desktopFiles []string, busErr *dbus.Error) {
	for _, entry := range m.Entries.FilterDocked() {
		if entry.appInfo != nil {
			desktopFiles = append(desktopFiles, entry.appInfo.GetFileName())
		}
	}
	return desktopFiles, nil
}

func (m *Manager) GetPluginSettings() (jsonStr string, busErr *dbus.Error) {
	jsonStr, err := m.pluginSettings.getJsonStr()
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return jsonStr, nil
}

func (m *Manager) SetPluginSettings(jsonStr string) *dbus.Error {
	var v pluginSettings
	err := json.Unmarshal([]byte(jsonStr), &v)
	if err != nil {
		return dbusutil.ToError(err)
	}
	m.pluginSettings.set(v)
	return nil
}

func (m *Manager) MergePluginSettings(jsonStr string) *dbus.Error {
	var v pluginSettings
	err := json.Unmarshal([]byte(jsonStr), &v)
	if err != nil {
		return dbusutil.ToError(err)
	}

	m.pluginSettings.merge(v)
	return nil
}

func (m *Manager) RemovePluginSettings(key1 string, key2List []string) *dbus.Error {
	m.pluginSettings.remove(key1, key2List)
	return nil
}

// 在Dock添加上添加图标的时候，有时候windowInfo不完整
// 会重复尝试10次，为了避免阻塞其他功能，放在goroutine里处理
// 窗口的增加和减少是有顺序的，在这个单独的goroutine里处理
func (m *Manager) accessEntries() {
	for {
		fun := <-m.entryDealChan
		fun()
	}
}
