// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-api/theme_thumb"
	accounts "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.accounts"
	display "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.display"
	imageeffect "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.imageeffect"
	sessiontimedate "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.timedate"
	sessionmanager "github.com/linuxdeepin/go-dbus-factory/com.deepin.sessionmanager"
	wm "github.com/linuxdeepin/go-dbus-factory/com.deepin.wm"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	timedate "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.timedate1"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/gsprop"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/strv"
	dutils "github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"

	"github.com/linuxdeepin/dde-daemon/appearance/background"
	"github.com/linuxdeepin/dde-daemon/appearance/fonts"
	"github.com/linuxdeepin/dde-daemon/appearance/subthemes"
	"github.com/linuxdeepin/dde-daemon/common/dsync"
	ddbus "github.com/linuxdeepin/dde-daemon/dbus"
	"github.com/linuxdeepin/dde-daemon/session/common"
)

//go:generate dbusutil-gen em -type Manager

// The supported types
const (
	TypeGtkTheme          = "gtk"
	TypeIconTheme         = "icon"
	TypeCursorTheme       = "cursor"
	TypeBackground        = "background"
	TypeGreeterBackground = "greeterbackground"
	TypeStandardFont      = "standardfont"
	TypeMonospaceFont     = "monospacefont"
	TypeFontSize          = "fontsize"
	TypeCompactFontSize   = "compactfontsize"
	TypeDTKSizeMode       = "dtksizemode"
)

const (
	wrapBgSchema    = "com.deepin.wrap.gnome.desktop.background"
	gnomeBgSchema   = "org.gnome.desktop.background"
	gsKeyBackground = "picture-uri"
	zonePath        = "/usr/share/zoneinfo/zone1970.tab"

	appearanceSchema        = "com.deepin.dde.appearance"
	xSettingsSchema         = "com.deepin.xsettings"
	gsKeyGtkTheme           = "gtk-theme"
	gsKeyIconTheme          = "icon-theme"
	gsKeyCursorTheme        = "cursor-theme"
	gsKeyFontStandard       = "font-standard"
	gsKeyFontMonospace      = "font-monospace"
	gsKeyFontSize           = "font-size"
	gsKeyCompactFontSize    = "compact-font-size"
	gsKeyBackgroundURIs     = "background-uris"
	gsKeyOpacity            = "opacity"
	gsKeyWallpaperSlideshow = "wallpaper-slideshow"
	gsKeyWallpaperURIs      = "wallpaper-uris"
	gsKeyQtActiveColor      = "qt-active-color"
	gsKeyDTKWindowRadius    = "dtk-window-radius"
	gsKeyQtScrollBarPolicy  = "qt-scrollbar-policy"
	gsKeyDTKSizeMode        = "dtk-size-mode"

	propQtActiveColor = "QtActiveColor"
	propFontSize      = "FontSize"
	propDTKSizeMode   = "DTKSizeMode"

	wsPolicyLogin  = "login"
	wsPolicyWakeup = "wakeup"

	defaultIconTheme     = "bloom"
	defaultGtkTheme      = "deepin"
	autoGtkTheme         = "deepin-auto"
	defaultCursorTheme   = "bloom"
	defaultStandardFont  = "Noto Sans"
	defaultMonospaceFont = "Noto Mono"

	dbusServiceName = "com.deepin.daemon.Appearance"
	dbusPath        = "/com/deepin/daemon/Appearance"
	dbusInterface   = dbusServiceName
)

// TODO 后续需要设计成用 subpath + 独立的 dconfig 配置来实现,增加可读性
const (
	dsettingsAppID                    = "org.deepin.dde.daemon"
	dsettingsAppearanceName           = "org.deepin.dde.daemon.appearance"
	dsettingsIrregularFontOverrideKey = "irregularFontOverride"
)

var wsConfigFile = filepath.Join(basedir.GetUserConfigDir(), "deepin/dde-daemon/appearance/wallpaper-slideshow.json")

// Manager shows current themes and fonts settings, emit 'Changed' signal if modified
// if themes list changed will emit 'Refreshed' signal
type Manager struct {
	service        *dbusutil.Service
	sessionSigLoop *dbusutil.SignalLoop
	sysSigLoop     *dbusutil.SignalLoop
	xConn          *x.Conn
	syncConfig     *dsync.Config
	bgSyncConfig   *dsync.Config

	GtkTheme           gsprop.String
	IconTheme          gsprop.String
	CursorTheme        gsprop.String
	Background         gsprop.String
	StandardFont       gsprop.String
	MonospaceFont      gsprop.String
	Opacity            gsprop.Double `prop:"access:rw"`
	FontSize           gsprop.Double `prop:"access:rw"`
	WallpaperSlideShow gsprop.String `prop:"access:rw"`
	WallpaperURIs      gsprop.String
	QtActiveColor      string `prop:"access:rw"`
	// 社区版定制需求，保存窗口圆角值，默认 18
	WindowRadius      gsprop.Int `prop:"access:rw"`
	QtScrollBarPolicy gsprop.Int `prop:"access:rw"`
	DTKSizeMode       gsprop.Int `prop:"access:rw"`

	wsLoopMap      map[string]*WSLoop
	wsSchedulerMap map[string]*WSScheduler
	monitorMap     map[string]string
	coordinateMap  map[string]*coordinate

	userObj             accounts.User
	imageBlur           accounts.ImageBlur
	timeDate            timedate.Timedate
	sessionTimeDate     sessiontimedate.Timedate
	imageEffect         imageeffect.ImageEffect
	xSettings           sessionmanager.XSettings
	login1Manager       login1.Manager
	themeAutoTimer      *time.Timer
	display             display.Display
	latitude            float64
	longitude           float64
	locationValid       bool
	detectSysClockTimer *time.Timer
	ts                  int64
	loc                 *time.Location

	setting        *gio.Settings
	xSettingsGs    *gio.Settings
	wrapBgSetting  *gio.Settings
	gnomeBgSetting *gio.Settings

	watcher    *fsnotify.Watcher
	endWatcher chan struct{}

	desktopBgs      []string
	greeterBg       string
	curMonitorSpace string
	wm              wm.Wm

	//nolint
	signals *struct {
		// Theme setting changed
		Changed struct {
			type0 string
			value string
		}

		// Theme list refreshed
		Refreshed struct {
			type0 string
		}
	}
}

type mapMonitorWorkspaceWSPolicy map[string]string
type mapMonitorWorkspaceWSConfig map[string]WSConfig
type mapMonitorWorkspaceWallpaperURIs map[string]string

type coordinate struct {
	latitude, longitude float64
}

// NewManager will create a 'Manager' object
func newManager(service *dbusutil.Service) *Manager {
	var m = new(Manager)
	m.service = service
	m.setting = gio.NewSettings(appearanceSchema)
	m.xSettingsGs = gio.NewSettings(xSettingsSchema)
	m.wrapBgSetting = gio.NewSettings(wrapBgSchema)

	m.GtkTheme.Bind(m.setting, gsKeyGtkTheme)
	m.IconTheme.Bind(m.setting, gsKeyIconTheme)
	m.CursorTheme.Bind(m.setting, gsKeyCursorTheme)
	m.StandardFont.Bind(m.setting, gsKeyFontStandard)
	m.MonospaceFont.Bind(m.setting, gsKeyFontMonospace)
	m.Background.Bind(m.wrapBgSetting, gsKeyBackground)
	m.Opacity.Bind(m.setting, gsKeyOpacity)
	m.WallpaperSlideShow.Bind(m.setting, gsKeyWallpaperSlideshow)
	m.WallpaperURIs.Bind(m.setting, gsKeyWallpaperURIs)
	// 社区版定制需求  保存窗口圆角值
	m.WindowRadius.Bind(m.xSettingsGs, gsKeyDTKWindowRadius)
	m.QtScrollBarPolicy.Bind(m.xSettingsGs, gsKeyQtScrollBarPolicy)
	m.DTKSizeMode.Bind(m.setting, gsKeyDTKSizeMode)

	var err error
	m.QtActiveColor, err = m.getQtActiveColor()
	if err != nil {
		logger.Warning(err)
	}

	if m.isHasDTKSizeModeKey() && (m.DTKSizeMode.Get() == 1) {
		m.FontSize.Bind(m.setting, gsKeyCompactFontSize)
	} else {
		m.FontSize.Bind(m.setting, gsKeyFontSize)
	}

	m.wsLoopMap = make(map[string]*WSLoop)
	m.wsSchedulerMap = make(map[string]*WSScheduler)
	m.coordinateMap = make(map[string]*coordinate)

	m.initCoordinate()

	m.gnomeBgSetting, _ = dutils.CheckAndNewGSettings(gnomeBgSchema)

	m.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		logger.Warning("New file watcher failed:", err)
	} else {
		m.endWatcher = make(chan struct{})
	}

	return m
}

func getLicenseAuthorizationProperty(conn *dbus.Conn) uint32 {
	var variant dbus.Variant
	err := conn.Object("com.deepin.license", "/com/deepin/license/Info").Call(
		"org.freedesktop.DBus.Properties.Get", 0, "com.deepin.license.Info", "AuthorizationProperty").Store(&variant)
	if err != nil {
		logger.Warning(err)
		return 0
	}
	if variant.Signature().String() != "u" {
		logger.Warning("not excepted value type")
		return 0
	}
	ret := variant.Value().(uint32)
	logger.Debug(" [getLicenseAuthorizationProperty] com.deepin.license.Info.AuthorizationProperty : ", ret)
	return ret
}

func listenLicenseInfoDBusPropChanged(conn *dbus.Conn, sigLoop *dbusutil.SignalLoop) {
	err := conn.Object("com.deepin.license.Info",
		"/com/deepin/license/Info").AddMatchSignal("com.deepin.license.Info", "LicenseStateChange").Err
	if err != nil {
		logger.Warning(err)
		return
	}

	sigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "com.deepin.license.Info.LicenseStateChange",
	}, func(sig *dbus.Signal) {
		if strings.Contains(string(sig.Name), "com.deepin.license.Info.LicenseStateChange") {
			go func() {
				licenseState := getLicenseAuthorizationProperty(conn)
				background.SetLicenseAuthorizationProperty(licenseState)
				background.UpdateLicenseAuthorizationProperty()
				logger.Info("[listenLicenseInfoDBusPropChanged] com.deepin.license.Info.LicenseStateChange : ", licenseState)
			}()
		}
	})
}

func (m *Manager) initCurrentBgs() {
	m.desktopBgs = m.getBackgroundURIs()

	if m.userObj == nil {
		return
	}
	greeterBg, err := m.userObj.GreeterBackground().Get(0)
	if err == nil {
		m.greeterBg = greeterBg
	} else {
		logger.Warning(err)
	}
}

func (m *Manager) getBackgroundURIs() []string {
	return m.setting.GetStrv(gsKeyBackgroundURIs)
}

func (m *Manager) isBgInUse(file string) bool {
	if file == m.greeterBg {
		return true
	}
	// 检查所有的工作区的屏幕壁纸是否占用
	mapWallpaperURIs, err := doUnmarshalMonitorWorkspaceWallpaperURIs(m.WallpaperURIs.Get())
	if err != nil {
		logger.Error(err)
		return false
	}
	for _, bg := range mapWallpaperURIs {
		if file == bg {
			return true
		}
	}

	return false
}

func (m *Manager) listBackground() background.Backgrounds {
	origin := background.ListBackground()
	result := make(background.Backgrounds, len(origin))

	for idx, bg := range origin {
		var deletable bool
		if bg.Deletable {
			// custom
			if !m.isBgInUse(bg.Id) {
				deletable = true
			}
		}
		result[idx] = &background.Background{
			Id:        bg.Id,
			Deletable: deletable,
		}
	}
	return result
}

func (m *Manager) destroy() {
	m.sessionSigLoop.Stop()
	m.xSettings.RemoveHandler(proxy.RemoveAllHandlers)
	m.syncConfig.Destroy()
	m.bgSyncConfig.Destroy()

	m.sysSigLoop.Stop()
	m.login1Manager.RemoveHandler(proxy.RemoveAllHandlers)
	for iSche := range m.wsSchedulerMap {
		m.wsSchedulerMap[iSche].stop()
	}

	if m.setting != nil {
		m.setting.Unref()
		m.setting = nil
	}

	if m.wrapBgSetting != nil {
		m.wrapBgSetting.Unref()
		m.wrapBgSetting = nil
	}

	if m.gnomeBgSetting != nil {
		m.gnomeBgSetting.Unref()
		m.gnomeBgSetting = nil
	}

	if m.watcher != nil {
		close(m.endWatcher)
		err := m.watcher.Close()
		if err != nil {
			logger.Warning(err)
		}
		m.watcher = nil
	}

	if m.xConn != nil {
		m.xConn.Close()
		m.xConn = nil
	}

	m.endCursorChangedHandler()
}

// resetFonts reset StandardFont and MonospaceFont
func (m *Manager) resetFonts() {
	err := fonts.Reset()
	if err != nil {
		logger.Debug("ResetFonts failed", err)
		return
	}
	m.checkFontConfVersion()
}

func (m *Manager) initUserObj(systemConn *dbus.Conn) {
	cur, err := user.Current()
	if err != nil {
		logger.Warning("failed to get current user:", err)
		return
	}

	err = common.ActivateSysDaemonService("com.deepin.daemon.Accounts")
	if err != nil {
		logger.Warning(err)
	}

	m.userObj, err = ddbus.NewUserByUid(systemConn, cur.Uid)
	if err != nil {
		logger.Warning("failed to new user object:", err)
		return
	}

	// sync desktop backgrounds
	userBackgrounds, err := m.userObj.DesktopBackgrounds().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	gsBackgrounds := m.setting.GetStrv(gsKeyBackgroundURIs)
	if !strv.Strv(userBackgrounds).Equal(gsBackgrounds) {
		m.setDesktopBackgrounds(gsBackgrounds)
	}
}

func (m *Manager) init() error {
	background.SetCustomWallpaperDeleteCallback(func(file string) {
		logger.Debug("imageBlur delete", file)
		err := m.imageBlur.Delete(0, file)
		if err != nil {
			logger.Warning("imageBlur delete err:", err)
		}

		logger.Debug("imageEffect delete", file)
		err = m.imageEffect.Delete(0, "all", file)
		if err != nil {
			logger.Warning("imageEffect delete err:", err)
		}
	})

	sessionBus := m.service.Conn()
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	go background.SetLicenseAuthorizationProperty(getLicenseAuthorizationProperty(systemBus))
	m.xConn, err = x.NewConn()
	if err != nil {
		return err
	}

	_, err = randr.QueryVersion(m.xConn, randr.MajorVersion,
		randr.MinorVersion).Reply(m.xConn)
	if err != nil {
		logger.Warning(err)
	}

	m.sessionSigLoop = dbusutil.NewSignalLoop(sessionBus, 10)
	m.sessionSigLoop.Start()

	m.wm = wm.NewWm(sessionBus)
	m.wm.InitSignalExt(m.sessionSigLoop, true)
	_, err = m.wm.ConnectWorkspaceCountChanged(m.handleWmWorkspaceCountChanged)
	if err != nil {
		logger.Warning(err)
	}
	_, err = m.wm.ConnectWorkspaceSwitched(m.handleWmWorkspaceSwithched)
	if err != nil {
		logger.Warning(err)
	}
	m.imageBlur = accounts.NewImageBlur(systemBus)
	m.imageEffect = imageeffect.NewImageEffect(systemBus)

	m.xSettings = sessionmanager.NewXSettings(sessionBus)
	theme_thumb.Init(m.getScaleFactor())

	m.xSettings.InitSignalExt(m.sessionSigLoop, true)
	_, err = m.xSettings.ConnectSetScaleFactorStarted(handleSetScaleFactorStarted)
	if err != nil {
		logger.Warning(err)
	}
	_, err = m.xSettings.ConnectSetScaleFactorDone(handleSetScaleFactorDone)
	if err != nil {
		logger.Warning(err)
	}

	m.initUserObj(systemBus)
	background.NotifyFunc = func(type0, value string) {
		m.emitSignalChanged(type0, value)
	}
	m.initCurrentBgs()
	m.display = display.NewDisplay(sessionBus)
	m.display.InitSignalExt(m.sessionSigLoop, true)
	err = m.display.Monitors().ConnectChanged(func(hasValue bool, value []dbus.ObjectPath) {
		m.handleDisplayChanged(hasValue)
	})
	if err != nil {
		logger.Warning("failed to connect Monitors changed:", err)
	}
	err = m.display.Primary().ConnectChanged(func(hasValue bool, value string) {
		m.handleDisplayChanged(hasValue)
	})
	if err != nil {
		logger.Warning("failed to connect Primary changed:", err)
	}
	m.updateMonitorMap()
	m.syncConfig = dsync.NewConfig("appearance", &syncConfig{m: m}, m.sessionSigLoop, dbusPath, logger)
	m.bgSyncConfig = dsync.NewConfig("background", &backgroundSyncConfig{m: m}, m.sessionSigLoop,
		backgroundDBusPath, logger)

	m.sysSigLoop = dbusutil.NewSignalLoop(systemBus, 10)
	m.sysSigLoop.Start()
	m.login1Manager = login1.NewManager(systemBus)
	m.login1Manager.InitSignalExt(m.sysSigLoop, true)
	if m.WallpaperURIs.Get() == "" { // 壁纸设置更新为v2.0版本，但是数据依旧为v1.0的数据
		err := m.updateNewVersionData()
		if err != nil {
			logger.Warning(err)
		}
	}
	m.initWallpaperSlideshow()

	m.timeDate = timedate.NewTimedate(systemBus)
	m.timeDate.InitSignalExt(m.sysSigLoop, true)

	listenLicenseInfoDBusPropChanged(systemBus, m.sysSigLoop)

	m.sessionTimeDate = sessiontimedate.NewTimedate(sessionBus)
	m.sessionTimeDate.InitSignalExt(m.sessionSigLoop, true)

	zone, err := m.timeDate.Timezone().Get(0)
	if err != nil {
		logger.Warning("Get Timezone Failed:", err)
	}
	l, err := time.LoadLocation(zone)
	if err != nil {
		logger.Warning("LoadLocation Failed :", err)
	}
	m.loc = l
	err = m.timeDate.Timezone().ConnectChanged(func(hasValue bool, value string) {
		if err != nil {
			logger.Warning(err)
		}
		for ct, coordinate := range m.coordinateMap {
			if value == ct {
				m.longitude = coordinate.longitude
				m.latitude = coordinate.latitude
			}
		}
		l, err := time.LoadLocation(value)
		if err != nil {
			logger.Warning("LoadLocation Failed :", err)
		}
		m.loc = l
		logger.Debug("value", value, m.longitude, m.latitude)
		if m.GtkTheme.Get() == autoGtkTheme {
			m.autoSetTheme(m.latitude, m.longitude)
			m.resetThemeAutoTimer()
		}
	})

	if err != nil {
		logger.Warning(err)
	}

	//修改时间后通过信号通知自动改变主题
	_, err = m.sessionTimeDate.ConnectTimeUpdate(func() {
		time.AfterFunc(2*time.Second, m.handleSysClockChanged)
	})
	if err != nil {
		logger.Warning("connect signal TimeUpdate failed:", err)
	}
	err = m.timeDate.NTP().ConnectChanged(func(hasValue bool, value bool) {
		time.AfterFunc(2*time.Second, m.handleSysClockChanged)
	})
	if err != nil {
		logger.Warning("connect NTP failed:", err)
	}

	// set gtk theme
	gtkThemes := subthemes.ListGtkTheme()
	currentGtkTheme := m.GtkTheme.Get()

	if currentGtkTheme == autoGtkTheme {
		m.updateThemeAuto(true)
	} else {
		if gtkThemes.Get(currentGtkTheme) == nil {
			m.GtkTheme.Set(defaultGtkTheme)
			err = m.doSetGtkTheme(defaultGtkTheme)
			if err != nil {
				logger.Warning("failed to set gtk theme:", err)
			}
		}
	}

	// set icon theme
	iconThemes := subthemes.ListIconTheme()
	currentIconTheme := m.IconTheme.Get()
	if iconThemes.Get(currentIconTheme) == nil {
		m.IconTheme.Set(defaultIconTheme)
		currentIconTheme = defaultIconTheme
	}
	err = m.doSetIconTheme(currentIconTheme)
	if err != nil {
		logger.Warning("failed to set icon theme:", err)
	}

	// set cursor theme
	cursorThemes := subthemes.ListCursorTheme()
	currentCursorTheme := m.CursorTheme.Get()
	if cursorThemes.Get(currentCursorTheme) == nil {
		m.CursorTheme.Set(defaultCursorTheme)
		currentCursorTheme = defaultCursorTheme
	}
	err = m.doSetCursorTheme(currentCursorTheme)
	if err != nil {
		logger.Warning("failed to set cursor theme:", err)
	}

	// Init IrregularFontWhiteList
	m.initAppearanceDSettings()
	return nil
}

// initFont 初始化字体设置
func (m *Manager) initFont() {
	value := fonts.FcFont_Match("system-ui")
	if m.StandardFont.Get() != value {
		m.StandardFont.Set(value)
	}
	value = fonts.FcFont_Match("monospace")
	if m.MonospaceFont.Get() != value {
		m.MonospaceFont.Set(value)
	}
	err := setDQtTheme(dQtFile, dQtSectionTheme,
		[]string{
			dQtKeyIcon,
			dQtKeyFont,
			dQtKeyMonoFont,
			dQtKeyFontSize},
		[]string{
			m.IconTheme.Get(),
			m.StandardFont.Get(),
			m.MonospaceFont.Get(),
			strconv.FormatFloat(m.FontSize.Get(), 'f', 1, 64)})
	if err != nil {
		logger.Warning("failed to set deepin qt theme:", err)
	}
	err = saveDQtTheme(dQtFile)
	if err != nil {
		logger.Warning("Failed to save deepin qt theme:", err)
	}
}

func (m *Manager) handleDisplayChanged(hasValue bool) {
	if !hasValue {
		return
	}
	m.updateMonitorMap()
	err := m.doUpdateWallpaperURIs()
	if err != nil {
		logger.Warning("failed to update WallpaperURIs:", err)
	}
}

func (m *Manager) handleWmWorkspaceCountChanged(count int32) {
	logger.Debug("wm workspace count changed", count)
	bgs := m.setting.GetStrv(gsKeyBackgroundURIs)
	if len(bgs) < int(count) {
		allBgs := background.ListBackground()

		numAdded := int(count) - len(bgs)
		for i := 0; i < numAdded; i++ {
			idx := rand.Intn(len(allBgs)) // #nosec G404
			// Id is file url
			bgs = append(bgs, allBgs[idx].Id)
		}
		m.setting.SetStrv(gsKeyBackgroundURIs, bgs)
	} else if len(bgs) > int(count) {
		bgs = bgs[:int(count)]
		m.setting.SetStrv(gsKeyBackgroundURIs, bgs)
	}
	err := m.doUpdateWallpaperURIs()
	if err != nil {
		logger.Warning("failed to update WallpaperURIs:", err)
	}
}

// 切换工作区
func (m *Manager) handleWmWorkspaceSwithched(from, to int32) {
	logger.Debugf("wm workspace switched from %d to %d", from, to)
	if m.userObj != nil {
		err := m.userObj.SetCurrentWorkspace(0, to)
		if err != nil {
			logger.Warning("call userObj.SetCurrentWorkspace err:", err)
		}
	}
}

func (m *Manager) doSetGtkTheme(value string) error {
	if value == autoGtkTheme {
		return nil
	}
	if !subthemes.IsGtkTheme(value) {
		return fmt.Errorf("invalid gtk theme '%v'", value)
	}

	// set dde-kwin decoration theme
	var ddeKWinTheme string
	switch value {
	case "deepin":
		ddeKWinTheme = "light"
	case "deepin-dark":
		ddeKWinTheme = "dark"
	}
	if ddeKWinTheme != "" {
		err := m.wm.SetDecorationDeepinTheme(0, ddeKWinTheme)
		if err != nil {
			logger.Warning(err)
		}
	}

	return subthemes.SetGtkTheme(value)
}

func (m *Manager) doSetIconTheme(value string) error {
	if !subthemes.IsIconTheme(value) {
		return fmt.Errorf("invalid icon theme '%v'", value)
	}

	err := subthemes.SetIconTheme(value)
	if err != nil {
		return err
	}

	return m.writeDQtTheme(dQtKeyIcon, value)
}

func (m *Manager) doSetCursorTheme(value string) error {
	if !subthemes.IsCursorTheme(value) {
		return fmt.Errorf("invalid cursor theme '%v'", value)
	}

	return subthemes.SetCursorTheme(value)
}

func (m *Manager) doSetMonitorBackground(monitorName string, imageFile string) (string, error) {
	logger.Debugf("call doSetMonitorBackground monitor:%q file:%q", monitorName, imageFile)
	file, t := background.GetWallpaperType(imageFile)
	if t == background.Unknown {
		return "", errors.New("invalid background")
	}
	// 如果设置的壁纸不是/usr/share/wallpapers/下的，则认为是用户自定义壁纸，需要发送add信号
	needNotify := false
	if !strings.HasPrefix(file, "/usr/share/wallpapers/") {
		needNotify = true
	}
	file, err := background.Prepare(file, t)
	if err != nil {
		logger.Warning("failed to prepare:", err)
		return "", err
	}
	logger.Debug("prepare result:", file)
	uri := dutils.EncodeURI(file, dutils.SCHEME_FILE)
	err = m.wm.SetCurrentWorkspaceBackgroundForMonitor(0, uri, monitorName)
	if err != nil {
		return "", err
	}
	// 如果文件发生增删，发送add/delete的信号
	if needNotify {
		// 由于用户操作，统一将用户自定义设置的壁纸当做新的壁纸处理，发送新增信号，告知上层
		m.emitSignalChanged("background-add", file)
		// 在此处理删除操作
		background.NotifyChanged()
	}
	err = m.doUpdateWallpaperURIs()
	if err != nil {
		logger.Warning("failed to update WallpaperURIs:", err)
	}
	_, err = m.imageBlur.Get(0, file)
	if err != nil {
		logger.Warning("call imageBlur.Get err:", err)
	}
	go func() {
		outputFile, err := m.imageEffect.Get(0, "", file)
		if err != nil {
			logger.Warning("imageEffect Get err:", err)
		} else {
			logger.Warning("imageEffect Get outputFile:", outputFile)
		}
	}()
	return file, nil
}

func (m *Manager) updateMonitorMap() {
	monitorList, _ := m.display.ListOutputNames(0)
	primary, _ := m.display.Primary().Get(0)
	index := 0
	m.monitorMap = make(map[string]string)
	for _, item := range monitorList {
		if item == primary {
			m.monitorMap[item] = "Primary"
		} else {
			m.monitorMap[item] = "Subsidiary" + strconv.Itoa(index)
			index++
		}
	}
}

func (m *Manager) reverseMonitorMap() map[string]string {
	reverseMap := make(map[string]string)
	for k, v := range m.monitorMap {
		reverseMap[v] = k
	}
	return reverseMap
}

func (m *Manager) doUpdateWallpaperURIs() error {
	mapWallpaperURIs := make(mapMonitorWorkspaceWallpaperURIs)
	workspaceCount, _ := m.wm.WorkspaceCount(0)
	monitorList, _ := m.display.ListOutputNames(0)
	for _, monitor := range monitorList {
		for idx := int32(1); idx <= workspaceCount; idx++ {
			wallpaperURI, err := m.wm.GetWorkspaceBackgroundForMonitor(0, idx, monitor)
			if err != nil {
				logger.Warning("get wallpaperURI failed:", err)
				continue
			}
			key := genMonitorKeyString(m.monitorMap[monitor], int(idx))
			mapWallpaperURIs[key] = wallpaperURI
		}
	}

	err := m.setPropertyWallpaperURIs(mapWallpaperURIs)
	if err != nil {
		return err
	}
	return nil
}

func doUnmarshalMonitorWorkspaceWallpaperURIs(jsonString string) (mapMonitorWorkspaceWallpaperURIs, error) {
	var cfg mapMonitorWorkspaceWallpaperURIs
	var byteMonitorWorkspaceWallpaperURIs = []byte(jsonString)
	err := json.Unmarshal(byteMonitorWorkspaceWallpaperURIs, &cfg)
	return cfg, err
}

func doMarshalMonitorWorkspaceWallpaperURIs(cfg mapMonitorWorkspaceWallpaperURIs) (string, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), err
}

func (m *Manager) setPropertyWallpaperURIs(cfg mapMonitorWorkspaceWallpaperURIs) error {
	uris, err := doMarshalMonitorWorkspaceWallpaperURIs(cfg)
	if err != nil {
		logger.Warning(err)
		return err
	}
	m.WallpaperURIs.Set(uris)
	return nil
}

func doUnmarshalWallpaperSlideshow(jsonString string) (mapMonitorWorkspaceWSPolicy, error) {
	var cfg mapMonitorWorkspaceWSPolicy
	var byteWallpaperSlideShow []byte = []byte(jsonString)
	err := json.Unmarshal(byteWallpaperSlideShow, &cfg)
	return cfg, err
}

func doMarshalWallpaperSlideshow(cfg mapMonitorWorkspaceWSPolicy) (string, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), err
}

func (m *Manager) setPropertyWallpaperSlideShow(cfg mapMonitorWorkspaceWSPolicy) error {
	slideshow, err := doMarshalWallpaperSlideshow(cfg)
	if err != nil {
		logger.Warning(err)
		return err
	}
	m.WallpaperSlideShow.Set(slideshow)
	return nil
}

func (m *Manager) doSetWallpaperSlideShow(monitorName string, wallpaperSlideShow string) error {
	idx, err := m.wm.GetCurrentWorkspace(0)
	if err != nil {
		logger.Warning("Get Current Workspace failure:", err)
		return err
	}
	cfg, err := doUnmarshalWallpaperSlideshow(m.WallpaperSlideShow.Get())
	if err != nil {
		logger.Warning("doUnmarshalWallpaperSlideshow Failed:", err)
	}
	if cfg == nil {
		cfg = make(mapMonitorWorkspaceWSPolicy)
	}

	key := genMonitorKeyString(monitorName, int(idx))
	cfg[key] = wallpaperSlideShow
	err = m.setPropertyWallpaperSlideShow(cfg)
	if err != nil {
		return err
	}
	m.curMonitorSpace = key
	return nil
}

func (m *Manager) doGetWallpaperSlideShow(monitorName string) (string, error) {
	idx, err := m.wm.GetCurrentWorkspace(0)
	if err != nil {
		logger.Warning("Get Current Workspace failure:", err)
		return "", err
	}
	cfg, err := doUnmarshalWallpaperSlideshow(m.WallpaperSlideShow.Get())
	if err != nil {
		return "", nil
	}
	key := genMonitorKeyString(monitorName, int(idx))
	wallpaperSlideShow := cfg[key]
	return wallpaperSlideShow, nil
}

func (m *Manager) doSetBackground(value string) (string, error) {
	logger.Debugf("call doSetBackground %q", value)
	file, t := background.GetWallpaperType(value)
	if t == background.Unknown {
		return "", errors.New("invalid background")
	}

	// 如果设置的壁纸不是/usr/share/wallpapers/下的，则认为是用户自定义壁纸，需要发送add信号
	needNotify := false
	if !strings.HasPrefix(file, "/usr/share/wallpapers/") {
		needNotify = true
	}
	file, err := background.Prepare(file, t)
	if err != nil {
		logger.Warning("failed to prepare:", err)
		return "", err
	}
	logger.Debug("prepare result:", file)
	uri := dutils.EncodeURI(file, dutils.SCHEME_FILE)
	err = m.wm.ChangeCurrentWorkspaceBackground(0, uri)
	if err != nil {
		return "", err
	}
	// 如果文件发生增删，发送add/delete的信号
	if needNotify {
		// 由于用户操作，统一将用户自定义设置的壁纸当做新的壁纸处理，发送新增信号，告知上层
		m.emitSignalChanged("background-add", file)
		// 在此处理删除操作
		background.NotifyChanged()
	}
	_, err = m.imageBlur.Get(0, file)
	if err != nil {
		logger.Warning("call imageBlur.Get err:", err)
	}
	go func() {
		outputFile, err := m.imageEffect.Get(0, "", file)
		if err != nil {
			logger.Warning("imageEffect Get err:", err)
		} else {
			logger.Warning("imageEffect Get outputFile:", outputFile)
		}
	}()

	return file, nil
}

func (m *Manager) doSetGreeterBackground(value string) error {
	value = dutils.EncodeURI(value, dutils.SCHEME_FILE)
	m.greeterBg = value
	if m.userObj == nil {
		return errors.New("user object is nil")
	}

	return m.userObj.SetGreeterBackground(0, value)
}

func (m *Manager) doSetStandardFont(value string) error {
	if !fonts.IsFontFamily(value) {
		return fmt.Errorf("invalid font family '%v'", value)
	}

	monoFont := m.MonospaceFont.Get()
	if !fonts.IsFontFamily(monoFont) {
		monoList := fonts.GetFamilyTable().ListMonospace()
		if len(monoList) == 0 {
			return fmt.Errorf("no valid mono font")
		}
		monoFont = monoList[0]
	}

	err := fonts.SetFamily(value, monoFont, m.FontSize.Get())
	if err != nil {
		return err
	}

	err = m.xSettings.SetString(0, "Qt/FontName", value)
	if err != nil {
		return err
	}

	return m.writeDQtTheme(dQtKeyFont, value)
}

func (m *Manager) doSetMonospaceFont(value string) error {
	if !fonts.IsFontFamily(value) {
		return fmt.Errorf("invalid font family '%v'", value)
	}

	standardFont := m.StandardFont.Get()
	if !fonts.IsFontFamily(standardFont) {
		standardList := fonts.GetFamilyTable().ListStandard()
		if len(standardList) == 0 {
			return fmt.Errorf("no valid standard font")
		}
		standardFont = standardList[0]
	}

	err := fonts.SetFamily(standardFont, value, m.FontSize.Get())
	if err != nil {
		return err
	}

	err = m.xSettings.SetString(0, "Qt/MonoFontName", value)
	if err != nil {
		return err
	}

	return m.writeDQtTheme(dQtKeyMonoFont, value)
}

func (m *Manager) doSetFontSize(size float64) error {
	if !fonts.IsFontSizeValid(size) {
		logger.Debug("[doSetFontSize] invalid size:", size)
		return fmt.Errorf("invalid font size '%v'", size)
	}

	err := fonts.SetFamily(m.StandardFont.Get(), m.MonospaceFont.Get(), size)
	if err != nil {
		return err
	}

	err = m.xSettings.SetString(0, "Qt/FontPointSize",
		strconv.FormatFloat(size, 'f', -1, 64))
	if err != nil {
		return err
	}

	return m.writeDQtTheme(dQtKeyFontSize, strconv.FormatFloat(size, 'f', 1, 64))
}

func (m *Manager) doSetDTKSizeMode(enabled int32) error {
	err := m.xSettings.SetInteger(0, "DTK/SizeMode", enabled)
	return err
}

func (m *Manager) isHasDTKSizeModeKey() bool {
	return m.setting.GetSchema().HasKey(gsKeyDTKSizeMode)
}

func (*Manager) doShow(ifc interface{}) (string, error) {
	if ifc == nil {
		return "", fmt.Errorf("not found target")
	}
	content, err := json.Marshal(ifc)
	return string(content), err
}

func (m *Manager) writeDQtTheme(key, value string) error {
	err := setDQtTheme(dQtFile, dQtSectionTheme,
		[]string{key}, []string{value})
	if err != nil {
		logger.Warning("failed to set deepin qt theme:", err)
	}
	return saveDQtTheme(dQtFile)
}

func (m *Manager) setDesktopBackgrounds(val []string) {
	if m.userObj != nil {
		err := m.userObj.SetDesktopBackgrounds(0, val)
		if err != nil {
			logger.Warning("call userObj.SetDesktopBackgrounds err:", err)
		}
	}
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) saveWSConfig(monitorSpace string, t time.Time) error {
	cfg, _ := loadWSConfig(wsConfigFile)
	var tempCfg WSConfig
	tempCfg.LastChange = t
	if m.wsLoopMap[monitorSpace] != nil {
		tempCfg.Showed = m.wsLoopMap[monitorSpace].GetShowed()
	}
	if cfg == nil {
		cfg = make(mapMonitorWorkspaceWSConfig)
	}
	cfg[monitorSpace] = tempCfg
	return cfg.save(wsConfigFile)
}

func (m *Manager) autoChangeBg(monitorSpace string, t time.Time) {
	logger.Debug("autoChangeBg", monitorSpace, t)
	if m.wsLoopMap[monitorSpace] == nil {
		return
	}
	idx, err := m.wm.GetCurrentWorkspace(0)
	if err != nil {
		logger.Warning(err)
	}
	strIdx := strconv.Itoa(int(idx))
	splitter := strings.Index(monitorSpace, "&&")
	if splitter == -1 {
		logger.Warning("monitorSpace format error")
		return
	}
	if strIdx == monitorSpace[splitter+len("&&"):] {
		monitorname := monitorSpace[:splitter]
		// 获取当前monitor的wallpaper，check是否是纯色壁纸
		currentURI, err := m.wm.GetWorkspaceBackgroundForMonitor(0, idx, monitorname)
		if err != nil {
			logger.Warning("get wallpaperURI failed:", err)
			return
		}
		_, t := background.GetWallpaperType(currentURI)
		file := m.wsLoopMap[monitorSpace].GetNext(t)
		if file == "" {
			logger.Warning("file is empty")
			return
		}
		_, err = m.doSetMonitorBackground(monitorname, file)
		if err != nil {
			logger.Warning("failed to set background:", err)
		}
	}
	err = m.saveWSConfig(monitorSpace, t)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) initWallpaperSlideshow() {
	m.loadWSConfig()
	cfg, err := doUnmarshalWallpaperSlideshow(m.WallpaperSlideShow.Get())
	if err == nil {
		for monitorSpace, policy := range cfg {
			_, ok := m.wsSchedulerMap[monitorSpace]
			if !ok {
				m.wsSchedulerMap[monitorSpace] = newWSScheduler(m.autoChangeBg)
			}
			_, ok = m.wsLoopMap[monitorSpace]
			if !ok {
				m.wsLoopMap[monitorSpace] = newWSLoop()
			}
			if isValidWSPolicy(policy) {
				if policy == wsPolicyLogin {
					err := m.changeBgAfterLogin(monitorSpace)
					if err != nil {
						logger.Warning("failed to change background after login:", err)
					}
				} else {
					nSec, err := strconv.ParseUint(policy, 10, 32)
					if err == nil && m.wsSchedulerMap[monitorSpace] != nil {
						m.wsSchedulerMap[monitorSpace].updateInterval(monitorSpace, time.Duration(nSec)*time.Second)
					}
				}
			}
		}
	} else {
		logger.Debug("doUnmarshalWallpaperSlideshow err is ", err)
	}
}

func (m *Manager) changeBgAfterLogin(monitorSpace string) error {
	runDir, err := basedir.GetUserRuntimeDir(true)
	if err != nil {
		return err
	}

	currentSessionId, err := getSessionId("/proc/self/sessionid")
	if err != nil {
		return err
	}

	var needChangeBg bool
	markFile := filepath.Join(runDir, "dde-daemon-wallpaper-slideshow-login"+monitorSpace)
	sessionId, err := getSessionId(markFile)
	if err == nil {
		if sessionId != currentSessionId {
			needChangeBg = true
		}
	} else if os.IsNotExist(err) {
		needChangeBg = true
	} else if err != nil {
		return err
	}

	if needChangeBg {
		m.autoChangeBg(monitorSpace, time.Now())
		err = ioutil.WriteFile(markFile, []byte(currentSessionId), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func getSessionId(filename string) (string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(content)), nil
}

func (m *Manager) loadWSConfig() {
	cfg := loadWSConfigSafe(wsConfigFile)
	for monitorSpace := range cfg {
		_, ok := m.wsSchedulerMap[monitorSpace]
		if !ok {
			m.wsSchedulerMap[monitorSpace] = newWSScheduler(m.autoChangeBg)
		}
		m.wsSchedulerMap[monitorSpace].mu.Lock()
		m.wsSchedulerMap[monitorSpace].lastSetBg = cfg[monitorSpace].LastChange
		m.wsSchedulerMap[monitorSpace].mu.Unlock()

		_, ok = m.wsLoopMap[monitorSpace]
		if !ok {
			m.wsLoopMap[monitorSpace] = newWSLoop()
		}
		m.wsLoopMap[monitorSpace].mu.Lock()
		for _, file := range cfg[monitorSpace].Showed {
			m.wsLoopMap[monitorSpace].showed[file] = struct{}{}
		}
		m.wsLoopMap[monitorSpace].mu.Unlock()
	}
}

func (m *Manager) updateWSPolicy(policy string) {
	cfg, err := doUnmarshalWallpaperSlideshow(policy)
	m.loadWSConfig()
	if err == nil {
		for monitorSpace, policy := range cfg {
			_, ok := m.wsSchedulerMap[monitorSpace]
			if !ok {
				m.wsSchedulerMap[monitorSpace] = newWSScheduler(m.autoChangeBg)
			}
			_, ok = m.wsLoopMap[monitorSpace]
			if !ok {
				m.wsLoopMap[monitorSpace] = newWSLoop()
			}
			if m.curMonitorSpace == monitorSpace && isValidWSPolicy(policy) {
				nSec, err := strconv.ParseUint(policy, 10, 32)
				if err == nil {
					m.wsSchedulerMap[monitorSpace].lastSetBg = time.Now()
					m.wsSchedulerMap[monitorSpace].updateInterval(monitorSpace, time.Duration(nSec)*time.Second)
					err = m.saveWSConfig(monitorSpace, time.Now())
					if err != nil {
						logger.Warning(err)
					}
				} else {
					m.wsSchedulerMap[monitorSpace].stop()
				}
			}
		}
	}
}

func (m *Manager) enableDetectSysClock(enabled bool) {
	nSec := 60 // 1 min
	if logger.GetLogLevel() == log.LevelDebug {
		// debug mode: 10 s
		nSec = 10
	}
	interval := time.Duration(nSec) * time.Second
	if enabled {
		m.ts = time.Now().Unix()
		if m.detectSysClockTimer == nil {
			m.detectSysClockTimer = time.AfterFunc(interval, func() {
				nowTs := time.Now().Unix()
				d := nowTs - m.ts - int64(nSec)
				if !(-2 < d && d < 2) {
					m.handleSysClockChanged()
				}

				m.ts = time.Now().Unix()
				m.detectSysClockTimer.Reset(interval)
			})
		} else {
			m.detectSysClockTimer.Reset(interval)
		}
	} else {
		// disable
		if m.detectSysClockTimer != nil {
			m.detectSysClockTimer.Stop()
		}
	}
}

func (m *Manager) handleSysClockChanged() {
	logger.Debug("system clock changed")
	if m.locationValid {
		m.autoSetTheme(m.latitude, m.longitude)
		m.resetThemeAutoTimer()
	}
}

func (m *Manager) updateThemeAuto(enabled bool) {
	m.enableDetectSysClock(enabled)
	logger.Debug("updateThemeAuto:", enabled)
	if enabled {
		var err error
		if m.themeAutoTimer == nil {
			m.themeAutoTimer = time.AfterFunc(0, func() {
				if m.locationValid {
					m.autoSetTheme(m.latitude, m.longitude)

					time.AfterFunc(5*time.Second, func() {
						m.resetThemeAutoTimer()
					})
				}
			})
		} else {
			m.themeAutoTimer.Reset(0)
		}

		city, err := m.timeDate.Timezone().Get(0)
		if err != nil {
			logger.Warning(err)
		}
		for ct, coordinate := range m.coordinateMap {
			if city == ct {
				m.longitude = coordinate.longitude
				m.latitude = coordinate.latitude
			}
		}

		m.updateLocation(m.latitude, m.longitude)
		logger.Debug("city", city, m.longitude, m.latitude)

	} else {
		m.latitude = 0
		m.longitude = 0
		m.locationValid = false
		if m.themeAutoTimer != nil {
			m.themeAutoTimer.Stop()
		}
	}
}

func (m *Manager) updateLocation(latitude, longitude float64) {
	m.latitude = latitude
	m.longitude = longitude
	m.locationValid = true
	logger.Debugf("update location, latitude: %v, longitude: %v",
		latitude, longitude)
	m.autoSetTheme(latitude, longitude)
	m.resetThemeAutoTimer()
}

func (m *Manager) resetThemeAutoTimer() {
	if m.themeAutoTimer == nil {
		logger.Debug("themeAutoTimer is nil")
		return
	}
	if !m.locationValid {
		logger.Debug("location is invalid")
		return
	}

	now := time.Now().In(m.loc)
	changeTime, err := m.getThemeAutoChangeTime(now, m.latitude, m.longitude)
	if err != nil {
		logger.Warning("failed to get theme auto change time:", err)
		return
	}

	interval := changeTime.Sub(now)
	logger.Debug("change theme after:", interval)
	m.themeAutoTimer.Reset(interval)
}

func (m *Manager) autoSetTheme(latitude, longitude float64) {
	now := time.Now().In(m.loc)
	if m.GtkTheme.Get() != autoGtkTheme {
		return
	}

	sunriseT, sunsetT, err := m.getSunriseSunset(now, latitude, longitude)
	if err != nil {
		logger.Warning(err)
		return
	}
	logger.Debugf("now: %v, sunrise: %v, sunset: %v",
		now, sunriseT, sunsetT)
	themeName := getThemeAutoName(isDaytime(now, sunriseT, sunsetT))
	logger.Debug("auto theme name:", themeName)

	currentTheme := m.GtkTheme.Get()
	if currentTheme != themeName {
		err = m.doSetGtkTheme(themeName)
		if err != nil {
			logger.Warning(err)
		}
	}
}

func (m *Manager) getQtActiveColor() (string, error) {
	str := m.xSettingsGs.GetString(gsKeyQtActiveColor)
	return xsColorToHexColor(str)
}

func xsColorToHexColor(str string) (string, error) {
	fields := strings.Split(str, ",")
	if len(fields) != 4 {
		return "", errors.New("length of fields is not 4")
	}

	var array [4]uint16
	for idx, field := range fields {
		v, err := strconv.ParseUint(field, 10, 16)
		if err != nil {
			return "", err
		}
		array[idx] = uint16(v)
	}

	var byteArr [4]byte
	for idx, value := range array {
		byteArr[idx] = byte((float64(value) / float64(math.MaxUint16)) * float64(math.MaxUint8))
	}
	return byteArrayToHexColor(byteArr), nil
}

func byteArrayToHexColor(p [4]byte) string {
	// p : [R G B A]
	if p[3] == 255 {
		return fmt.Sprintf("#%02X%02X%02X", p[0], p[1], p[2])
	}
	return fmt.Sprintf("#%02X%02X%02X%02X", p[0], p[1], p[2], p[3])
}

var hexColorReg = regexp.MustCompile(`^#([0-9A-F]{6}|[0-9A-F]{8})$`)

func parseHexColor(hexColor string) (array [4]byte, err error) {
	hexColor = strings.ToUpper(hexColor)
	match := hexColorReg.FindStringSubmatch(hexColor)
	if match == nil {
		err = errors.New("invalid hex color format")
		return
	}
	hexNums := string(match[1])
	count := 4
	if len(hexNums) == 6 {
		count = 3
		array[3] = 255
	}

	for i := 0; i < count; i++ {
		array[i], err = parseHexNum(hexNums[i*2 : i*2+2])
		if err != nil {
			return
		}
	}
	return
}

func parseHexNum(str string) (byte, error) {
	v, err := strconv.ParseUint(str, 16, 8)
	return byte(v), err
}

func (m *Manager) setQtActiveColor(hexColor string) error {
	xsColor, err := hexColorToXsColor(hexColor)
	if err != nil {
		return err
	}

	ok := m.xSettingsGs.SetString(gsKeyQtActiveColor, xsColor)
	if !ok {
		return errors.New("failed to save")
	}
	return nil
}

// 原有的壁纸数据保存在background-uris字段中，当壁纸模块升级后，将该数据迁移至wallpaper-uris中;
// wallpaper-slideshow前后版本的格式不同，根据旧数据重新生成新数据
func (m *Manager) updateNewVersionData() error {
	reverseMonitorMap := m.reverseMonitorMap()
	primaryMonitor := reverseMonitorMap["Primary"]
	slideshowConfig := make(mapMonitorWorkspaceWSPolicy)
	slideShow := m.WallpaperSlideShow.Get()
	workspaceCount, _ := m.wm.WorkspaceCount(0)
	_, err := doUnmarshalWallpaperSlideshow(slideShow)
	if err != nil {
		// slideShow的内容无法解析为map[string]string数据表示低版本壁纸，进行数据格式转换
		for i := 1; i <= int(workspaceCount); i++ {
			key := genMonitorKeyString(primaryMonitor, i)
			slideshowConfig[key] = slideShow
		}
		err := m.setPropertyWallpaperSlideShow(slideshowConfig)
		if err != nil {
			return err
		}
	}

	monitorWorkspaceWallpaperURIs := make(mapMonitorWorkspaceWallpaperURIs)
	backgroundURIs := m.getBackgroundURIs()
	for i, uri := range backgroundURIs {
		err := m.wm.SetWorkspaceBackgroundForMonitor(0, int32(i+1), primaryMonitor, uri)
		if err != nil {
			return fmt.Errorf("failed to set background:%v to workspace%v : %v", uri, i+1, err)
		}
		key := genMonitorKeyString("Primary", i+1)
		monitorWorkspaceWallpaperURIs[key] = uri
	}
	err = m.setPropertyWallpaperURIs(monitorWorkspaceWallpaperURIs)
	if err != nil {
		return err
	}

	go func() {
		// V20对应SP3, SP2阶段gsettings background-uris中部分数据丢失，SP2升到SP3通过窗管接口获取壁纸
		// 目前第一次初始化，wm和appearance存在互相依赖，因此这里同步wm数据只能异步，这属于设计问题，后续可以优化交互逻辑
		monitorWorkspaceWallpaperURIs := make(mapMonitorWorkspaceWallpaperURIs)
		for monitorName, convertMonitorName := range m.monitorMap {
			for i := int32(0); i < workspaceCount; i++ {
				uri, err := m.wm.GetWorkspaceBackgroundForMonitor(0, i+1, monitorName)
				if err != nil {
					logger.Warningf("failed to get monitor:%v workspace:%v background:%v", monitorName, i+1, err)
					continue
				}

				key := genMonitorKeyString(convertMonitorName, i+1)
				monitorWorkspaceWallpaperURIs[key] = uri
			}
		}
		err := m.setPropertyWallpaperURIs(monitorWorkspaceWallpaperURIs)
		if err != nil {
			logger.Warning("failed to set wallpaper:", err)
		}
	}()
	return nil
}

func genMonitorKeyString(monitor string, idx interface{}) string {
	return fmt.Sprintf("%v&&%v", monitor, idx)
}

func hexColorToXsColor(hexColor string) (string, error) {
	byteArr, err := parseHexColor(hexColor)
	if err != nil {
		return "", err
	}
	var array [4]uint16
	for idx, value := range byteArr {
		array[idx] = uint16((float64(value) / float64(math.MaxUint8)) * float64(math.MaxUint16))
	}
	return fmt.Sprintf("%d,%d,%d,%d", array[0], array[1],
		array[2], array[3]), nil
}

func (m *Manager) initCoordinate() {
	content, err := ioutil.ReadFile(zonePath)
	if err != nil {
		logger.Warning(err)
		return
	}
	var (
		lines = strings.Split(string(content), "\n")
		match = regexp.MustCompile(`^#`)
	)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		if match.MatchString(line) {
			continue
		}
		strv := strings.Split(line, "\t")
		m.iso6709Parsing(string(strv[2]), strv[1])
	}
}

func (m *Manager) iso6709Parsing(city, coordinates string) {
	var cdn coordinate
	match := regexp.MustCompile(`(\+|-)\d+\.?\d*`)

	temp := match.FindAllString(coordinates, 2)

	temp[0] = temp[0][:3] + "." + temp[0][3:]
	temp[1] = temp[1][:4] + "." + temp[1][4:]
	lat, _ := strconv.ParseFloat(temp[0], 64)
	lon, _ := strconv.ParseFloat(temp[1], 64)
	cdn.longitude = lon
	cdn.latitude = lat
	m.coordinateMap[city] = &cdn
}

func (m *Manager) initAppearanceDSettings() {
	ds := configManager.NewConfigManager(m.sysSigLoop.Conn())

	appearancePath, err := ds.AcquireManager(0, dsettingsAppID, dsettingsAppearanceName, "")
	if err != nil {
		logger.Warning(err)
		return
	}

	dsAppearance, err := configManager.NewManager(m.sysSigLoop.Conn(), appearancePath)
	if err != nil {
		logger.Warning(err)
		return
	}

	getIrregularFontWhiteListKey := func() {
		v, err := dsAppearance.Value(0, dsettingsIrregularFontOverrideKey)
		if err != nil {
			logger.Warning(err)
			return
		}
		var overrideMap fonts.IrregularFontOverrideMap
		overrideMapString := v.Value().(string)
		err = json.Unmarshal([]byte(overrideMapString), &overrideMap)
		if err != nil {
			logger.Warning(err)
			return
		}
		fonts.SetIrregularFontWhiteList(overrideMap)
	}

	getIrregularFontWhiteListKey()

	dsAppearance.InitSignalExt(m.sysSigLoop, true)
	_, err = dsAppearance.ConnectValueChanged(func(key string) {
		if key == dsettingsIrregularFontOverrideKey {
			getIrregularFontWhiteListKey()
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}
