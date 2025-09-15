// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/dxinput"
	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	ddbus "github.com/linuxdeepin/dde-daemon/dbus"
	accounts "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.accounts1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	dutils "github.com/linuxdeepin/go-lib/utils"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/wm/ewmh"
	"github.com/linuxdeepin/go-x11-client/util/wm/icccm"
)

const (
	dsettingsKeyboardName = "org.deepin.dde.daemon.keyboard"

	dconfigKeyRepeatEnabled   = "repeatEnabled"
	dconfigKeyRepeatInterval  = "repeatInterval"
	dconfigKeyRepeatDelay     = "delay"
	dconfigKeyLayoutOptions   = "layoutOptions"
	dconfigKeyCursorBlinkTime = "cursorBlinkTime"
	dconfigKeyCapslockToggle  = "capslockToggle"
	dconfigKeyAppLayoutMap    = "app-layout-map"
	dconfigKeyLayoutScope     = "layout-scope"
	dconfigKeyUserLayoutList  = "user-layout-list"
	dconfigKeyLayout          = "layout"

	// 系统级keyboard配置
	dsettingsKeyboardEnabledKey = "keyboardEnabled"

	// 布局相关常量
	layoutScopeGlobal = 0
	layoutScopeApp    = 1
	layoutDelim       = ";"
	kbdDefaultLayout  = "us" + layoutDelim

	// 系统配置文件
	kbdSystemConfig = "/etc/default/keyboard"
	qtDefaultConfig = ".config/Trolltech.conf"
	cmdSetKbd       = "/usr/bin/setxkbmap"
)

type Keyboard struct {
	xConn          *x.Conn
	activeWindow   x.Window
	activeWinClass string
	service        *dbusutil.Service
	sysSigLoop     *dbusutil.SignalLoop
	PropsMu        sync.RWMutex
	CurrentLayout  dconfig.String `prop:"access:rw"`
	appLayoutCfg   appLayoutConfig
	// dbusutil-gen: equal=nil
	UserLayoutList dconfig.StringList `prop:"access:rw"`

	// dbusutil-gen: ignore-below
	LayoutScope    dconfig.Uint32 `prop:"access:rw"`
	RepeatEnabled  dconfig.Bool   `prop:"access:rw"`
	CapslockToggle dconfig.Bool   `prop:"access:rw"`

	CursorBlink dconfig.Int `prop:"access:rw"`

	RepeatInterval dconfig.Uint32 `prop:"access:rw"`
	RepeatDelay    dconfig.Uint32 `prop:"access:rw"`

	UserOptionList dconfig.StringList `prop:"access:rw"`

	dsgKeyboardConfig *dconfig.DConfig
	user              accounts.User
	layoutMap         layoutMap

	devNumber      int
	devInfos       Keyboards
	dsgInputConfig *dconfig.DConfig
}

func newKeyboard(service *dbusutil.Service) *Keyboard {
	var kbd = new(Keyboard)

	kbd.service = service

	if err := kbd.initKeyboardDConfig(); err != nil {
		logger.Errorf("Failed to initialize keyboard dconfig: %v", err)
		panic("Keyboard DConfig initialization failed - cannot continue without dconfig support")
	}

	var err error
	err = kbd.loadAppLayoutConfig()
	if err != nil {
		logger.Warning("failed to load app layout config:", err)
	}
	if kbd.appLayoutCfg.Map == nil {
		kbd.appLayoutCfg.Map = make(map[string]int)
	}

	// Treeland环境没有X
	if !hasTreeLand {
		kbd.xConn, err = x.NewConn()
		if err != nil {
			logger.Error("failed to get X conn:", err)
			return nil
		}
	}

	kbd.layoutMap, err = getLayoutsFromFile(kbdLayoutsXml)
	if err != nil {
		logger.Error("failed to get layouts description:", err)
		return nil
	}

	sysConn, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return nil
	}
	kbd.sysSigLoop = dbusutil.NewSignalLoop(sysConn, 10)
	kbd.sysSigLoop.Start()
	kbd.initUser()

	if kbd.user != nil {
		// set current layout
		layout, err := kbd.user.Layout().Get(0)
		if err != nil {
			logger.Warning(err)
		} else {
			kbd.PropsMu.Lock()
			kbd.CurrentLayout.Set(fixLayout(layout))
			kbd.PropsMu.Unlock()
		}

		// set layout list
		layoutList, err := kbd.user.HistoryLayout().Get(0)
		if err != nil {
			logger.Warning(err)
		} else {
			kbd.PropsMu.Lock()
			kbd.UserLayoutList.Set(fixLayoutList(layoutList))
			kbd.PropsMu.Unlock()
		}
	}

	kbd.devNumber = getKeyboardNumber()

	kbd.listenRootWindowXEvent()
	kbd.startXEventLoop()
	kbd.initDsgConfig()
	kbd.updateDXKeyboards()
	return kbd
}

func fixLayout(layout string) string {
	if !strings.Contains(layout, layoutDelim) {
		return layout + layoutDelim
	}
	return layout
}

func fixLayoutList(layouts []string) []string {
	result := make([]string, len(layouts))
	for idx, layout := range layouts {
		result[idx] = fixLayout(layout)
	}
	return result
}

func (kbd *Keyboard) initUser() {
	systemConn, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return
	}

	cur, err := user.Current()
	if err != nil {
		logger.Warning("failed to get current user:", err)
		return
	}

	kbd.user, err = ddbus.NewUserByUid(systemConn, cur.Uid)
	if err != nil {
		logger.Warningf("failed to new user by uid %s: %v", cur.Uid, err)
		return
	}
	kbd.user.InitSignalExt(kbd.sysSigLoop, true)
	err = kbd.user.Layout().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}

		layout := fixLayout(value)
		kbd.setLayout(layout)
	})
	if err != nil {
		logger.Warning(err)
	}
	err = kbd.user.HistoryLayout().ConnectChanged(func(hasValue bool, value []string) {
		if !hasValue {
			return
		}

		kbd.PropsMu.Lock()
		kbd.setPropUserLayoutList(fixLayoutList(value))
		kbd.PropsMu.Unlock()
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (kbd *Keyboard) destroy() {
	if kbd.user != nil {
		kbd.user.RemoveHandler(proxy.RemoveAllHandlers)
		kbd.user = nil
	}

	kbd.sysSigLoop.Stop()
}

func (kbd *Keyboard) init() {
	if kbd.user != nil {
		kbd.correctLayout()
	}
	kbd.applySettings()
}

func (kbd *Keyboard) applySettings() {
	kbd.applyLayout()
	kbd.applyOptions()
	kbd.applyRepeat()
}

func (kbd *Keyboard) correctLayout() {
	kbd.PropsMu.RLock()
	currentLayout := kbd.CurrentLayout.Get()
	kbd.PropsMu.RUnlock()

	if currentLayout == "" {
		layoutFromSysCfg, err := getSystemLayout(kbdSystemConfig)
		if err != nil {
			logger.Warning(err)
		}
		if layoutFromSysCfg == "" {
			layoutFromSysCfg = kbdDefaultLayout
		}

		kbd.setLayoutForAccountsUser(layoutFromSysCfg)
	}

	kbd.PropsMu.RLock()
	layoutList := kbd.UserLayoutList.Get()
	kbd.PropsMu.RUnlock()

	if len(layoutList) == 0 {
		kbd.setLayoutListForAccountsUser([]string{currentLayout})
	}
}

func (kbd *Keyboard) handleDeviceChanged() {
	num := getKeyboardNumber()
	logger.Debug("Keyboard changed:", num, kbd.devNumber)
	if num > kbd.devNumber {
		kbd.applySettings()
	}
	kbd.devNumber = num

	kbd.updateDXKeyboards()
}

func (kbd *Keyboard) applyLayout() {
	kbd.PropsMu.RLock()
	currentLayout := kbd.CurrentLayout.Get()
	kbd.PropsMu.RUnlock()

	err := applyLayout(currentLayout)
	if err != nil {
		logger.Warningf("failed to set layout to %q: %v", currentLayout, err)
		return
	}

	err = applyXmodmapConfig()
	if err != nil {
		logger.Warning("failed to apply xmodmap:", err)
	}
}

func (kbd *Keyboard) applyOptions() {
	options := kbd.UserOptionList.Get()
	if len(options) == 0 {
		return
	}

	// the old value wouldn't be cleared, so we will force clear it.
	err := doAction(cmdSetKbd + " -option")
	if err != nil {
		logger.Warning("failed to clear keymap option:", err)
		return
	}

	cmd := cmdSetKbd
	for _, opt := range options {
		cmd += fmt.Sprintf(" -option %q", opt)
	}
	err = doAction(cmd)
	if err != nil {
		logger.Warning("failed to set keymap options:", err)
	}
}

var errInvalidLayout = errors.New("invalid layout")

func (kbd *Keyboard) checkLayout(layout string) error {
	if layout == "" {
		return dbusutil.ToError(errInvalidLayout)
	}

	_, ok := kbd.layoutMap[layout]
	if !ok {
		return dbusutil.ToError(errInvalidLayout)
	}
	return nil
}

func (kbd *Keyboard) setLayout(layout string) {
	logger.Debug("set layout to", layout)
	kbd.PropsMu.Lock()
	kbd.setPropCurrentLayout(layout)
	kbd.PropsMu.Unlock()

	kbd.applyLayout()
}

func (kbd *Keyboard) setCurrentLayout(write *dbusutil.PropertyWrite) *dbus.Error {
	layout := write.Value.(string)
	logger.Debugf("setCurrentLayout %q", layout)

	if kbd.user == nil {
		return dbusutil.ToError(errors.New("kbd.user is nil"))
	}

	err := kbd.checkLayout(layout)
	if err != nil {
		return dbusutil.ToError(err)
	}
	kbd.addUserLayout(layout)

	if kbd.LayoutScope.Get() == layoutScopeGlobal {
		kbd.setLayoutForAccountsUser(layout)
	} else {
		kbd.setLayoutScopeApp(layout)
	}
	return nil
}

func (kbd *Keyboard) setLayoutScopeApp(layout string) {
	if kbd.activeWinClass == "" {
		return
	}

	if kbd.appLayoutCfg.set(kbd.activeWinClass, layout) {
		kbd.saveAppLayoutConfig()
	}

	kbd.setLayout(layout)
}

func (kbd *Keyboard) saveAppLayoutConfig() {
	jsonStr := kbd.appLayoutCfg.toJson()
	logger.Debug("save app layout config", jsonStr)
	if err := kbd.saveToKeyboardDConfig(dconfigKeyAppLayoutMap, jsonStr); err != nil {
		logger.Warning("Failed to save app layout config to dconfig:", err)
	}
}

func (kbd *Keyboard) addUserLayout(layout string) {
	kbd.PropsMu.Lock()
	newLayoutList, added := addItemToList(layout, kbd.UserLayoutList.Get())
	kbd.PropsMu.Unlock()

	if !added {
		return
	}

	kbd.setLayoutListForAccountsUser(newLayoutList)
}

func (kbd *Keyboard) delUserLayout(layout string) {
	if layout == "" {
		return
	}

	kbd.PropsMu.Lock()
	newLayoutList, deleted := delItemFromList(layout, kbd.UserLayoutList.Get())
	kbd.PropsMu.Unlock()
	if !deleted {
		return
	}
	kbd.setLayoutListForAccountsUser(newLayoutList)
	if kbd.appLayoutCfg.deleteLayout(layout) {
		kbd.saveAppLayoutConfig()
	}
}

func (kbd *Keyboard) addUserOption(option string) {
	if len(option) == 0 {
		return
	}

	// TODO: check option validity

	ret, added := addItemToList(option, kbd.UserOptionList.Get())
	if !added {
		return
	}
	kbd.UserOptionList.Set(ret)
}

func (kbd *Keyboard) delUserOption(option string) {
	if len(option) == 0 {
		return
	}

	ret, deleted := delItemFromList(option, kbd.UserOptionList.Get())
	if !deleted {
		return
	}
	kbd.UserOptionList.Set(ret)
}

func (kbd *Keyboard) applyCursorBlink() {
	value := kbd.CursorBlink.Get()
	xsSetInt32(xsPropBlinkTimeut, int32(value))

	err := setQtCursorBlink(int32(value), path.Join(os.Getenv("HOME"),
		qtDefaultConfig))
	if err != nil {
		logger.Debugf("failed to set qt cursor blink to '%v': %v",
			value, err)
	}
}

func (kbd *Keyboard) setLayoutForAccountsUser(layout string) {
	if kbd.user == nil {
		return
	}

	err := kbd.user.SetLayout(0, layout)
	if err != nil {
		logger.Debug("failed to set layout for accounts user:", err)
	}
}

func (kbd *Keyboard) setLayoutListForAccountsUser(layoutList []string) {
	if kbd.user == nil {
		return
	}

	layoutList = filterSpaceStr(layoutList)

	err := kbd.user.SetHistoryLayout(0, layoutList)
	if err != nil {
		logger.Debug("failed to set layout list for accounts user:", err)
	}
}

func (kbd *Keyboard) getRepeatDelay() (delay uint32) {
	// 重复延迟列表
	var repeatDelayLevel = []uint32{
		20, 80, 150, 250, 360, 480, 600,
	}
	// 最低重复延迟等级
	const minRepeatDelayLevel = 1

	delay = kbd.RepeatDelay.Get()
	// 重复延迟过低的话，键盘事件的重复率太高了，处理最低档位，上层配置保持不变
	if delay < repeatDelayLevel[minRepeatDelayLevel] {
		return repeatDelayLevel[minRepeatDelayLevel]
	}
	return
}

func (kbd *Keyboard) applyKwinWaylandRepeat() {
	var (
		delay    = kbd.getRepeatDelay()
		interval = kbd.RepeatInterval.Get()
	)

	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}

	//+ 前端interval范围是20-100,kwin中值是相反的,kwin无法得知范围,故后端处理下
	interval = 120 - interval
	obj := sessionBus.Object("org.kde.KWin", "/KWin")
	err = obj.Call("org.kde.KWin.setRepeatRateAndDelay", 0, int32(interval), int32(delay)).Err
	if err != nil {
		logger.Warning(err)
		return
	}
}

func (kbd *Keyboard) applyX11Repeat() {
	var (
		repeat   = kbd.RepeatEnabled.Get()
		delay    = kbd.getRepeatDelay()
		interval = kbd.RepeatInterval.Get()
	)

	err := dxinput.SetKeyboardRepeat(repeat, delay, interval)
	if err != nil {
		logger.Debug("failed to set repeat:", err, repeat, delay, interval)
	}
}

func (kbd *Keyboard) applyRepeat() {
	// TODO: treeland环境无法设置
	if hasTreeLand {
		return
	} else if globalWayland {
		kbd.applyKwinWaylandRepeat()
	} else {
		kbd.applyX11Repeat()
	}
}

func applyLayout(value string) error {
	array := strings.Split(value, layoutDelim)
	if len(array) != 2 {
		return fmt.Errorf("invalid layout: %s", value)
	}

	layout, variant := array[0], array[1]
	if layout != "us" {
		layout += ",us"

		if variant != "" {
			variant += ","
		}
	}

	var cmd = fmt.Sprintf("%s -layout \"%s\" -variant \"%s\"",
		cmdSetKbd, layout, variant)
	return doAction(cmd)
}

func setQtCursorBlink(rate int32, file string) error {
	ok := dutils.WriteKeyToKeyFile(file, "Qt", "cursorFlashTime", rate)
	if !ok {
		return fmt.Errorf("write failed")
	}

	return nil
}

func getSystemLayout(file string) (string, error) {
	fr, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer fr.Close()

	var (
		found   int
		layout  string
		variant string

		regLayout  = regexp.MustCompile(`^XKBLAYOUT=`)
		regVariant = regexp.MustCompile(`^XKBVARIANT=`)

		scanner = bufio.NewScanner(fr)
	)
	for scanner.Scan() {
		if found == 2 {
			break
		}

		var line = scanner.Text()
		if regLayout.MatchString(line) {
			layout = strings.Trim(getValueFromLine(line, "="), "\"")
			found += 1
			continue
		}

		if regVariant.MatchString(line) {
			variant = strings.Trim(getValueFromLine(line, "="), "\"")
			found += 1
		}
	}

	if len(layout) == 0 {
		return "", fmt.Errorf("not found default layout")
	}

	return layout + layoutDelim + variant, nil
}

func getValueFromLine(line, delim string) string {
	array := strings.Split(line, delim)
	if len(array) != 2 {
		return ""
	}

	return strings.TrimSpace(array[1])
}

func applyXmodmapConfig() error {
	err := doAction("xmodmap -e 'keycode 247 = XF86Away NoSymbol NoSymbol'")
	if err != nil {
		return err
	}
	config := os.Getenv("HOME") + "/.Xmodmap"
	if !dutils.IsFileExist(config) {
		return nil
	}
	return doAction("xmodmap " + config)
}

func (kbd *Keyboard) listenRootWindowXEvent() {
	if kbd.xConn == nil {
		return
	}
	rootWin := kbd.xConn.GetDefaultScreen().Root
	const eventMask = x.EventMaskPropertyChange
	err := x.ChangeWindowAttributesChecked(kbd.xConn, rootWin, x.CWEventMask,
		[]uint32{eventMask}).Check(kbd.xConn)
	if err != nil {
		logger.Warning(err)
	}
	kbd.handleActiveWindowChanged()
}

func (kbd *Keyboard) handleActiveWindowChanged() {
	if kbd.xConn == nil {
		return
	}
	activeWindow, err := ewmh.GetActiveWindow(kbd.xConn).Reply(kbd.xConn)
	if err != nil {
		logger.Warning(err)
		return
	}
	if activeWindow == kbd.activeWindow || activeWindow == 0 {
		return
	}
	kbd.activeWindow = activeWindow
	logger.Debug("active window changed to", activeWindow)
	wmClass, err := icccm.GetWMClass(kbd.xConn, activeWindow).Reply(kbd.xConn)
	if err != nil {
		logger.Warning(err)
		return
	}
	class := strings.ToLower(wmClass.Class)
	if class == kbd.activeWinClass || class == "" {
		return
	}
	kbd.activeWinClass = class
	logger.Debug("wm class changed to", class)

	if kbd.LayoutScope.Get() != layoutScopeApp {
		return
	}

	layout, ok := kbd.appLayoutCfg.get(class)
	if ok {
		kbd.setLayout(layout)
	}
	// 否则不改变布局
}

func (kbd *Keyboard) startXEventLoop() {
	if kbd.xConn == nil {
		return
	}
	eventChan := make(chan x.GenericEvent, 10)
	kbd.xConn.AddEventChan(eventChan)

	go func() {
		for ev := range eventChan {
			switch ev.GetEventCode() {
			case x.PropertyNotifyEventCode:
				event, _ := x.NewPropertyNotifyEvent(ev)
				kbd.handlePropertyNotifyEvent(event)
			}
		}
	}()
}

func (kbd *Keyboard) handlePropertyNotifyEvent(ev *x.PropertyNotifyEvent) {
	if kbd.xConn == nil {
		return
	}
	rootWin := kbd.xConn.GetDefaultScreen().Root
	if ev.Window == rootWin {
		kbd.handleActiveWindowChanged()
	}
}

func (kbd *Keyboard) toggleNextLayout() {
	for idx, item := range kbd.UserLayoutList.Get() {
		if kbd.CurrentLayout.Get() == item {
			var index = (idx + 1) % len(kbd.UserLayoutList.Get())
			kbd.setLayout(kbd.UserLayoutList.Get()[index])
			return
		}
	}
}

func (kbd *Keyboard) updateDXKeyboards() {
	logger.Debug("updateDXKeyboards")
	kbd.devInfos = Keyboards{}
	for _, info := range getKeyboardInfos(false) {
		tmp := kbd.devInfos.get(info.Id)
		if tmp != nil {
			continue
		}
		kbd.devInfos = append(kbd.devInfos, info)
	}

	enableKeyboard, err := kbd.dsgInputConfig.GetValueBool(dsettingsKeyboardEnabledKey)
	if err != nil {
		logger.Warningf("getDsgData key : %s. err : %s", dsettingsKeyboardEnabledKey, err)
		return
	}

	for _, dev := range kbd.devInfos {
		dev.Enable(enableKeyboard)
	}
}

func (kbd *Keyboard) initDsgConfig() error {
	var err error
	kbd.dsgInputConfig, err = dconfig.NewDConfig(dsettingsAppID, dsettingsInputdevices, "")
	if err != nil {
		logger.Warning(err)
		return err
	}

	kbd.dsgInputConfig.ConnectConfigChanged(dsettingsKeyboardEnabledKey, func(value interface{}) {
		kbd.updateDXKeyboards()
	})

	return nil
}

func (kbd *Keyboard) initKeyboardDConfig() error {
	err := error(nil)

	kbd.dsgKeyboardConfig, err = dconfig.NewDConfig(dsettingsAppID, dsettingsKeyboardName, "")
	if err != nil {
		return fmt.Errorf("create keyboard dconfig manager failed: %v", err)
	}

	kbd.CurrentLayout.Bind(kbd.dsgKeyboardConfig, dconfigKeyLayout)
	kbd.UserLayoutList.Bind(kbd.dsgKeyboardConfig, dconfigKeyUserLayoutList)
	kbd.LayoutScope.Bind(kbd.dsgKeyboardConfig, dconfigKeyLayoutScope)
	kbd.RepeatEnabled.Bind(kbd.dsgKeyboardConfig, dconfigKeyRepeatEnabled)
	kbd.CapslockToggle.Bind(kbd.dsgKeyboardConfig, dconfigKeyCapslockToggle)
	kbd.CursorBlink.Bind(kbd.dsgKeyboardConfig, dconfigKeyCursorBlinkTime)
	kbd.RepeatInterval.Bind(kbd.dsgKeyboardConfig, dconfigKeyRepeatInterval)
	kbd.RepeatDelay.Bind(kbd.dsgKeyboardConfig, dconfigKeyRepeatDelay)
	kbd.UserOptionList.Bind(kbd.dsgKeyboardConfig, dconfigKeyLayoutOptions)

	kbd.dsgKeyboardConfig.ConnectValueChanged(func(key string) {
		logger.Debugf("Keyboard dconfig value changed: %s", key)
		switch key {
		case dconfigKeyRepeatEnabled:
			kbd.applyRepeat()
		case dconfigKeyRepeatInterval:
			kbd.applyRepeat()
		case dconfigKeyRepeatDelay:
			kbd.applyRepeat()
		case dconfigKeyCursorBlinkTime:
			kbd.applyCursorBlink()
		case dconfigKeyLayoutOptions:
			kbd.applyOptions()
		case dconfigKeyLayoutScope:
			kbd.applyLayout()
		default:
			logger.Debugf("Unhandled keyboard dconfig key change: %s", key)
		}
	})

	logger.Info("Keyboard DConfig initialization completed successfully")
	return nil
}

// saveToKeyboardDConfig 保存配置值到keyboard dconfig
func (kbd *Keyboard) saveToKeyboardDConfig(key string, value interface{}) error {
	if kbd.dsgKeyboardConfig == nil {
		return errors.New("keyboard dconfig not initialized")
	}

	err := kbd.dsgKeyboardConfig.SetValue(key, value)
	if err != nil {
		return fmt.Errorf("failed to save %s to keyboard dconfig: %v", key, err)
	}

	logger.Debugf("Saved %s = %v to keyboard dconfig", key, value)
	return nil
}
