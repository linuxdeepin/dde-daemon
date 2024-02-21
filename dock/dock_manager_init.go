// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus"
	libApps "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.apps"
	kwayland "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.kwayland"
	kwin "github.com/linuxdeepin/go-dbus-factory/org.kde.kwin"
	launcher "github.com/linuxdeepin/go-dbus-factory/com.deepin.dde.daemon.launcher"
	libDDELauncher "github.com/linuxdeepin/go-dbus-factory/com.deepin.dde.launcher"
	sessionmanager "github.com/linuxdeepin/go-dbus-factory/com.deepin.sessionmanager"
	wm "github.com/linuxdeepin/go-dbus-factory/com.deepin.wm"
	wmswitcher "github.com/linuxdeepin/go-dbus-factory/com.deepin.wmswitcher"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gsettings"
	x "github.com/linuxdeepin/go-x11-client"

	"github.com/linuxdeepin/dde-daemon/common/dsync"
)

const (
	ddeDataDir         = "/usr/share/dde/data"
	windowPatternsFile = ddeDataDir + "/window_patterns.json"
)

func (m *Manager) initEntries() {
	m.initDockedApps()
	m.Entries.insertCb = func(entry *AppEntry, index int) {
		entryObjPath := dbus.ObjectPath(entryDBusObjPathPrefix + entry.Id)
		logger.Debug("entry added", entry.Id, index)
		_ = m.service.Emit(m, "EntryAdded", entryObjPath, int32(index))
	}
	m.Entries.removeCb = func(entry *AppEntry) {
		_ = m.service.Emit(m, "EntryRemoved", entry.Id)
		go func() {
			time.Sleep(time.Second)
			err := m.service.StopExport(entry)
			if err != nil {
				logger.Warning("StopExport error:", err)
			}
		}()
	}

	if m.isWaylandSession {
		m.initWaylandWindows()
	} else {
		m.initClientList()
	}

	m.clientListInitEnd = true
}

func (m *Manager) connectSettingKeyChanged(key string, handler func(key string)) {
	gsettings.ConnectChanged(dockSchema, key, handler)
}

func (m *Manager) listenSettingsChanged() {
	// listen hide mode change
	m.connectSettingKeyChanged(settingKeyHideMode, func(key string) {
		mode := HideModeType(m.settings.GetEnum(key))
		logger.Debug(key, "changed to", mode)
		m.updateHideState(false)
	})

	// listen display mode change
	m.connectSettingKeyChanged(settingKeyDisplayMode, func(key string) {
		mode := DisplayModeType(m.settings.GetEnum(key))
		logger.Debug(key, "changed to", mode)
	})

	// listen position change
	m.connectSettingKeyChanged(settingKeyPosition, func(key string) {
		position := positionType(m.settings.GetEnum(key))
		logger.Debug(key, "changed to", position)
	})

	// listen force quit
	m.connectSettingKeyChanged(settingKeyForceQuitApp, func(key string) {
		m.forceQuitAppStatus = forceQuitAppType(m.settings.GetEnum(key))
		logger.Debug(key, "changed to", m.forceQuitAppStatus)

		m.Entries.mu.Lock()
		for _, entry := range m.Entries.items {
			entry.updateMenu()
		}
		m.Entries.mu.Unlock()
	})
}

func (m *Manager) listenWMSwitcherSignal() {
	m.wmSwitcher.InitSignalExt(m.sessionSigLoop, true)
	_, err := m.wmSwitcher.ConnectWMChanged(func(wmName string) {
		m.PropsMu.Lock()
		m.wmName = wmName
		m.PropsMu.Unlock()
		logger.Debugf("wm changed %q", wmName)
	})
	if err != nil {
		logger.Warning(err)
	}
}

const (
	wmName3D = "deepin wm"
	wmName2D = "deepin metacity"
)

func (m *Manager) listenKWinSignal() {
	m.kwin.InitSignalExt(m.sessionSigLoop, true)
	_, err := m.kwin.ConnectMultitaskStateChanged(func(state bool) {
		m.isMultiTaskViewShow = state
		m.updateHideState(false)
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) listenWMSignal() {
	m.wm.InitSignalExt(m.sessionSigLoop, true)
	_, err := m.wm.ConnectCompositingEnabledChanged(func(enabled bool) {
		m.PropsMu.Lock()
		defer m.PropsMu.Unlock()
		if enabled {
			m.wmName = wmName3D
		} else {
			m.wmName = wmName2D
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

// 代码逻辑源自startdde wm_kwin.go
func (m *Manager) currentWM() string {
	enabled, err := m.wm.CompositingEnabled().Get(0)
	if err != nil {
		logger.Warning(err)
		return ""
	}

	wmName := wmName2D
	if enabled {
		wmName = wmName3D
	}
	return wmName
}

func (m *Manager) is3DWM() bool {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()
	if m.wmName == "" {
		m.wmName = m.currentWM()
	}
	return m.wmName == wmName3D
}

func (m *Manager) handleLauncherItemDeleted(itemInfo launcher.ItemInfo) {
	dockedEntries := m.Entries.FilterDocked()
	for _, entry := range dockedEntries {
		file := entry.appInfo.GetFileName()
		if file == itemInfo.Path {
			m.undockEntry(entry)
			return
		}
	}
}

func (m *Manager) handleLauncherItemCreated(itemInfo launcher.ItemInfo) {

}

// 在收到 launcher item 更新的信号后，需要更新相关信息，包括 appInfo、innerId、名称、图标、菜单。
func (m *Manager) handleLauncherItemUpdated(itemInfo launcher.ItemInfo) {
	desktopFile := toLocalPath(itemInfo.Path)
	entry, err := m.Entries.GetByDesktopFilePath(desktopFile)
	if err != nil {
		logger.Warning(err)
		return
	}
	if entry == nil {
		return
	}

	appInfo := NewAppInfoFromFile(desktopFile)
	if appInfo == nil {
		logger.Warningf("failed to new app info from file %q: %v", desktopFile, err)
		return
	}
	entry.setAppInfo(appInfo)
	entry.innerId = appInfo.innerId
	entry.updateName()
	entry.updateMenu()
	entry.forceUpdateIcon() // 可能存在Icon图片改变,但Icon名称未改变的情况,因此强制发Icon的属性改变信号
}

func (m *Manager) listenLauncherSignal() {
	m.launcher.InitSignalExt(m.sessionSigLoop, true)
	_, err := m.launcher.ConnectItemChanged(func(status string, itemInfo launcher.ItemInfo,
		categoryID int64) {
		logger.Debugf("launcher item changed status: %s, itemInfo: %#v",
			status, itemInfo)
		switch status {
		case "deleted":
			m.handleLauncherItemDeleted(itemInfo)
		case "created":
			m.handleLauncherItemCreated(itemInfo)
		case "updated":
			m.handleLauncherItemUpdated(itemInfo)
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	m.ddeLauncher.InitSignalExt(m.sessionSigLoop, true)
	_, err = m.ddeLauncher.ConnectVisibleChanged(func(visible bool) {
		logger.Debug("dde-launcher visible changed", visible)
		m.ddeLauncherVisibleMu.Lock()
		m.ddeLauncherVisible = visible
		m.ddeLauncherVisibleMu.Unlock()

		m.updateHideState(false)
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) isDDELauncherVisible() bool {
	m.ddeLauncherVisibleMu.Lock()
	result := m.ddeLauncherVisible
	m.ddeLauncherVisibleMu.Unlock()
	return result
}

func (m *Manager) getWinIconPreferredApps() []string {
	return m.settings.GetStrv(settingKeyWinIconPreferredApps)
}

func (m *Manager) init() error {
	m.rootWindow = globalXConn.GetDefaultScreen().Root

	var err error
	m.settings = gio.NewSettings(dockSchema)
	m.HideMode.Bind(m.settings, settingKeyHideMode)
	m.DisplayMode.Bind(m.settings, settingKeyDisplayMode)
	m.Position.Bind(m.settings, settingKeyPosition)
	m.IconSize.Bind(m.settings, settingKeyIconSize)
	m.ShowTimeout.Bind(m.settings, settingKeyShowTimeout)
	m.HideTimeout.Bind(m.settings, settingKeyHideTimeout)
	m.WindowSizeEfficient.Bind(m.settings, settingKeyWindowSizeEfficient)
	m.WindowSizeFashion.Bind(m.settings, settingKeyWindowSizeFashion)
	m.DockedApps.Bind(m.settings, settingKeyDockedApps)
	m.appearanceSettings = gio.NewSettings(appearanceSchema)
	m.Opacity.Bind(m.appearanceSettings, settingKeyOpacity)

	m.forceQuitAppStatus = forceQuitAppType(m.settings.GetEnum(settingKeyForceQuitApp))

	m.FrontendWindowRect = NewRect()
	m.smartHideModeTimer = time.AfterFunc(10*time.Second, m.smartHideModeTimerExpired)
	m.smartHideModeTimer.Stop()

	m.listenSettingsChanged()

	m.windowInfoMap = make(map[x.Window]WindowInfoImp)
	m.windowPatterns, err = loadWindowPatterns(windowPatternsFile)
	if err != nil {
		logger.Warning("loadWindowPatterns failed:", err)
	}

	sessionBus := m.service.Conn()
	m.wm = wm.NewWm(sessionBus)

	systemBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	m.appsObj = libApps.NewApps(systemBus)
	m.launcher = launcher.NewLauncher(sessionBus)
	m.ddeLauncher = libDDELauncher.NewLauncher(sessionBus)
	m.startManager = sessionmanager.NewStartManager(sessionBus)
	m.wmSwitcher = wmswitcher.NewWMSwitcher(sessionBus)
	m.kwin = kwin.NewKWin(sessionBus)
	
	sessionType := os.Getenv("XDG_SESSION_TYPE")
	if strings.Contains(sessionType, "wayland") {
		m.isWaylandSession = true
	}

	if m.isWaylandSession {
		m.waylandWM = kwayland.NewWindowManager(sessionBus)
		m.waylandManager = newWaylandManager()
	}

	m.sessionSigLoop = dbusutil.NewSignalLoop(m.service.Conn(), 10)
	m.sessionSigLoop.Start()
	m.sysSigLoop = dbusutil.NewSignalLoop(m.sysService.Conn(), 10)
	m.sysSigLoop.Start()
	m.listenLauncherSignal()
	m.listenWMSwitcherSignal()
	m.listenWMSignal()
	m.listenKWinSignal()

	if strings.Contains(sessionType, "wayland") {
		m.listenWaylandWMSignals()
	}
	m.initDSettings(m.sysService.Conn())
	//systemd拉起bamfdaemon可能会失败，导致阻塞，手动拉一遍
	err = m.startBAMFDaemon(sessionBus)
	if err != nil {
		logger.Warning("startBAMFDaemon failed")
	}

	if m.isWaylandSession {
		m.registerIdentifyKWindowFuncs()
	} else {
		m.registerIdentifyWindowFuncs()
	}

	m.initEntries()
	m.pluginSettings = newPluginSettingsStorage(m)

	m.syncConfig = dsync.NewConfig("dock", &syncConfig{m: m}, m.sessionSigLoop,
		dbusPath, logger)

	// 强制将 ClassicMode 转为 EfficientMode
	if m.DisplayMode.Get() == int32(DisplayModeClassicMode) {
		m.DisplayMode.Set(int32(DisplayModeEfficientMode))
	}

	m.entryDealChan = make(chan func(), 64)
	go m.accessEntries()

	if strings.Contains(sessionType, "x11") {
		go m.eventHandleLoop()
		m.listenRootWindowXEvent()
	}
	return nil
}

func (m *Manager) startBAMFDaemon(bus *dbus.Conn) error {
	systemdUser := bus.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	var jobPath dbus.ObjectPath
	err := systemdUser.Call("org.freedesktop.systemd1.Manager.StartUnit",
		dbus.FlagNoAutoStart, "bamfdaemon.service", "replace").Store(&jobPath)
	if err != nil {
		logger.Warning("failed to start bamfdaemon.service:", err)
		return err
	}
	return nil
}
