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
	// DConfig相关常量
	dsettingsKeyboardName = "org.deepin.dde.daemon.keyboard"

	// DConfig键值常量 - 对应keyboard.json中的配置项
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
	CurrentLayout  string `prop:"access:rw"`
	appLayoutCfg   appLayoutConfig
	// dbusutil-gen: equal=nil
	UserLayoutList []string `prop:"access:rw"`

	// dbusutil-gen: ignore-below
	LayoutScope    uint32 `prop:"access:rw"`
	RepeatEnabled  bool   `prop:"access:rw"`
	CapslockToggle bool   `prop:"access:rw"`

	CursorBlink int `prop:"access:rw"`

	RepeatInterval uint32 `prop:"access:rw"`
	RepeatDelay    uint32 `prop:"access:rw"`

	UserOptionList []string `prop:"access:rw"`

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

	if err := kbd.initKeyboardDConfig(service); err != nil {
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
			kbd.CurrentLayout = fixLayout(layout)
			kbd.PropsMu.Unlock()
		}

		// set layout list
		layoutList, err := kbd.user.HistoryLayout().Get(0)
		if err != nil {
			logger.Warning(err)
		} else {
			kbd.PropsMu.Lock()
			kbd.UserLayoutList = fixLayoutList(layoutList)
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
	currentLayout := kbd.CurrentLayout
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
	layoutList := kbd.UserLayoutList
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
	currentLayout := kbd.CurrentLayout
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
	options := kbd.UserOptionList
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

	if kbd.LayoutScope == layoutScopeGlobal {
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
	newLayoutList, added := addItemToList(layout, kbd.UserLayoutList)
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
	newLayoutList, deleted := delItemFromList(layout, kbd.UserLayoutList)
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

	ret, added := addItemToList(option, kbd.UserOptionList)
	if !added {
		return
	}
	kbd.UserOptionList = ret
}

func (kbd *Keyboard) delUserOption(option string) {
	if len(option) == 0 {
		return
	}

	ret, deleted := delItemFromList(option, kbd.UserOptionList)
	if !deleted {
		return
	}
	kbd.UserOptionList = ret
}

func (kbd *Keyboard) applyCursorBlink() {
	value := kbd.CursorBlink
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

	delay = kbd.RepeatDelay
	// 重复延迟过低的话，键盘事件的重复率太高了，处理最低档位，上层配置保持不变
	if delay < repeatDelayLevel[minRepeatDelayLevel] {
		return repeatDelayLevel[minRepeatDelayLevel]
	}
	return
}

func (kbd *Keyboard) applyKwinWaylandRepeat() {
	var (
		delay    = kbd.getRepeatDelay()
		interval = kbd.RepeatInterval
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
		repeat   = kbd.RepeatEnabled
		delay    = kbd.getRepeatDelay()
		interval = kbd.RepeatInterval
	)

	err := dxinput.SetKeyboardRepeat(repeat, delay, interval)
	if err != nil {
		logger.Debug("failed to set repeat:", err, repeat, delay, interval)
	}
	setWMKeyboardRepeat(repeat, delay, interval)
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

	if kbd.LayoutScope != layoutScopeApp {
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
	for idx, item := range kbd.UserLayoutList {
		if kbd.CurrentLayout == item {
			var index = (idx + 1) % len(kbd.UserLayoutList)
			kbd.setLayout(kbd.UserLayoutList[index])
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

func (kbd *Keyboard) initKeyboardDConfig(service *dbusutil.Service) error {
	err := error(nil)

	kbd.dsgKeyboardConfig, err = dconfig.NewDConfig(dsettingsAppID, dsettingsKeyboardName, "")
	if err != nil {
		return fmt.Errorf("create keyboard dconfig manager failed: %v", err)
	}

	// 从dconfig初始化所有属性
	kbd.initKeyboardPropsFromDConfig()

	kbd.dsgKeyboardConfig.ConnectValueChanged(func(key string) {
		logger.Debugf("Keyboard dconfig value changed: %s", key)
		switch key {
		case dconfigKeyRepeatEnabled:
			kbd.updateRepeatEnabledFromDConfig()
		case dconfigKeyRepeatInterval:
			kbd.updateRepeatIntervalFromDConfig()
		case dconfigKeyRepeatDelay:
			kbd.updateRepeatDelayFromDConfig()
		case dconfigKeyCursorBlinkTime:
			kbd.updateCursorBlinkFromDConfig()
		case dconfigKeyCapslockToggle:
			kbd.updateCapslockToggleFromDConfig()
		case dconfigKeyLayoutOptions:
			kbd.updateLayoutOptionsFromDConfig()
		case dconfigKeyLayoutScope:
			kbd.updateLayoutScopeFromDConfig()
		case dconfigKeyUserLayoutList:
			kbd.updateUserLayoutListFromDConfig()
		case dconfigKeyLayout:
			kbd.updateCurrentLayoutFromDConfig()
		default:
			logger.Debugf("Unhandled keyboard dconfig key change: %s", key)
		}
	})

	logger.Info("Keyboard DConfig initialization completed successfully")
	return nil
}

// initKeyboardPropsFromDConfig 从dconfig初始化键盘属性
func (kbd *Keyboard) initKeyboardPropsFromDConfig() error {
	// RepeatEnabled
	err := error(nil)
	kbd.RepeatEnabled, err = kbd.dsgKeyboardConfig.GetValueBool(dconfigKeyRepeatEnabled)
	if err != nil {
		return err
	}
	// RepeatInterval
	kbd.RepeatInterval, err = kbd.dsgKeyboardConfig.GetValueUInt32(dconfigKeyRepeatInterval)
	if err != nil {
		return err
	}

	// RepeatDelay
	kbd.RepeatDelay, err = kbd.dsgKeyboardConfig.GetValueUInt32(dconfigKeyRepeatDelay)
	if err != nil {
		return err
	}

	// CursorBlink
	kbd.CursorBlink, err = kbd.dsgKeyboardConfig.GetValueInt(dconfigKeyCursorBlinkTime)
	if err != nil {
		return err
	}

	// CapslockToggle
	kbd.CapslockToggle, err = kbd.dsgKeyboardConfig.GetValueBool(dconfigKeyCapslockToggle)
	if err != nil {
		return err
	}

	// UserOptionList (layoutOptions)
	list, err := kbd.dsgKeyboardConfig.GetValueStringList(dconfigKeyLayoutOptions)
	if err != nil {
		return err
	}
	kbd.UserOptionList = list
	// LayoutScope
	kbd.LayoutScope, err = kbd.dsgKeyboardConfig.GetValueUInt32(dconfigKeyLayoutScope)
	if err != nil {
		return err
	}

	// UserLayoutList - 用户布局列表
	userLayoutList, err := kbd.dsgKeyboardConfig.GetValueStringList(dconfigKeyUserLayoutList)
	if err != nil {
		return err
	}
	kbd.UserLayoutList = userLayoutList

	// CurrentLayout - 键盘布局
	kbd.CurrentLayout, err = kbd.dsgKeyboardConfig.GetValueString(dconfigKeyLayout)
	if err != nil {
		return err
	}

	return nil
}

// SetKeyboardWriteCallbacks 为键盘属性设置DBus写回调
func (kbd *Keyboard) SetKeyboardWriteCallbacks(service *dbusutil.Service) error {
	kbdServerObj := service.GetServerObject(kbd)
	var err error

	// RepeatEnabled 写回调
	err = kbdServerObj.SetWriteCallback(kbd, "RepeatEnabled",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid RepeatEnabled type: %T", write.Value))
			}
			if saveErr := kbd.saveToKeyboardDConfig(dconfigKeyRepeatEnabled, value); saveErr != nil {
				logger.Warning("Failed to save RepeatEnabled to dconfig:", saveErr)
			}
			kbd.applyRepeat()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set RepeatEnabled write callback:", err)
	}

	// RepeatInterval 写回调
	err = kbdServerObj.SetWriteCallback(kbd, "RepeatInterval",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(uint32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid RepeatInterval type: %T", write.Value))
			}
			if saveErr := kbd.saveToKeyboardDConfig(dconfigKeyRepeatInterval, value); saveErr != nil {
				logger.Warning("Failed to save RepeatInterval to dconfig:", saveErr)
			}
			kbd.applyRepeat()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set RepeatInterval write callback:", err)
	}

	// RepeatDelay 写回调
	err = kbdServerObj.SetWriteCallback(kbd, "RepeatDelay",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(uint32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid RepeatDelay type: %T", write.Value))
			}
			if saveErr := kbd.saveToKeyboardDConfig(dconfigKeyRepeatDelay, value); saveErr != nil {
				logger.Warning("Failed to save RepeatDelay to dconfig:", saveErr)
			}
			kbd.applyRepeat()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set RepeatDelay write callback:", err)
	}

	// CursorBlink 写回调
	err = kbdServerObj.SetWriteCallback(kbd, "CursorBlink",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid CursorBlink type: %T", write.Value))
			}
			if saveErr := kbd.saveToKeyboardDConfig(dconfigKeyCursorBlinkTime, value); saveErr != nil {
				logger.Warning("Failed to save CursorBlink to dconfig:", saveErr)
			}
			kbd.applyCursorBlink()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set CursorBlink write callback:", err)
	}

	// CapslockToggle 写回调
	err = kbdServerObj.SetWriteCallback(kbd, "CapslockToggle",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(bool)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid CapslockToggle type: %T", write.Value))
			}
			if saveErr := kbd.saveToKeyboardDConfig(dconfigKeyCapslockToggle, value); saveErr != nil {
				logger.Warning("Failed to save CapslockToggle to dconfig:", saveErr)
			}
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set CapslockToggle write callback:", err)
	}

	// UserOptionList 写回调
	err = kbdServerObj.SetWriteCallback(kbd, "UserOptionList",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.([]string)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid UserOptionList type: %T", write.Value))
			}
			if saveErr := kbd.saveToKeyboardDConfig(dconfigKeyLayoutOptions, value); saveErr != nil {
				logger.Warning("Failed to save UserOptionList to dconfig:", saveErr)
			}
			kbd.applyOptions()
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set UserOptionList write callback:", err)
	}

	// LayoutScope 写回调
	err = kbdServerObj.SetWriteCallback(kbd, "LayoutScope",
		func(write *dbusutil.PropertyWrite) *dbus.Error {
			value, ok := write.Value.(int32)
			if !ok {
				return dbusutil.ToError(fmt.Errorf("invalid LayoutScope type: %T", write.Value))
			}
			if saveErr := kbd.saveToKeyboardDConfig(dconfigKeyLayoutScope, value); saveErr != nil {
				logger.Warning("Failed to save LayoutScope to dconfig:", saveErr)
			}
			return nil
		})
	if err != nil {
		logger.Warning("Failed to set LayoutScope write callback:", err)
	}

	// UserLayoutList 写回调
	err = kbdServerObj.SetWriteCallback(kbd, "UserLayoutList", func(write *dbusutil.PropertyWrite) *dbus.Error {
		value, ok := write.Value.([]string)
		if !ok {
			return dbusutil.ToError(fmt.Errorf("invalid UserLayoutList type: %T", write.Value))
		}
		if saveErr := kbd.saveToKeyboardDConfig(dconfigKeyUserLayoutList, value); saveErr != nil {
			logger.Warning("Failed to save UserLayoutList to dconfig:", saveErr)
		}
		return nil
	})
	if err != nil {
		logger.Warning("Failed to set UserLayoutList write callback:", err)
	}

	// CurrentLayout 写回调
	err = kbdServerObj.SetWriteCallback(kbd, "CurrentLayout", func(write *dbusutil.PropertyWrite) *dbus.Error {
		value, ok := write.Value.(string)
		if !ok {
			return dbusutil.ToError(fmt.Errorf("invalid CurrentLayout type: %T", write.Value))
		}
		if saveErr := kbd.saveToKeyboardDConfig(dconfigKeyLayout, value); saveErr != nil {
			logger.Warning("Failed to save CurrentLayout to dconfig:", saveErr)
		}
		return nil
	})
	if err != nil {
		logger.Warning("Failed to set CurrentLayout write callback:", err)
	}

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

// 从dconfig更新属性的方法实现
func (kbd *Keyboard) updateRepeatEnabledFromDConfig() {
	value, err := kbd.dsgKeyboardConfig.GetValueBool(dconfigKeyRepeatEnabled)
	if err != nil {
		logger.Warning("Failed to get RepeatEnabled from dconfig:", err)
		return
	}
	kbd.RepeatEnabled = value
	kbd.applyRepeat()
}

func (kbd *Keyboard) updateRepeatIntervalFromDConfig() {
	value, err := kbd.dsgKeyboardConfig.GetValueUInt32(dconfigKeyRepeatInterval)
	if err != nil {
		logger.Warning("Failed to get RepeatInterval from dconfig:", err)
		return
	}
	kbd.RepeatInterval = value
	kbd.applyRepeat()
}

func (kbd *Keyboard) updateRepeatDelayFromDConfig() {
	value, err := kbd.dsgKeyboardConfig.GetValueUInt32(dconfigKeyRepeatDelay)
	if err != nil {
		logger.Warning("Failed to get RepeatDelay from dconfig:", err)
		return
	}
	kbd.RepeatDelay = value
	kbd.applyRepeat()
}

func (kbd *Keyboard) updateCursorBlinkFromDConfig() {
	value, err := kbd.dsgKeyboardConfig.GetValueInt(dconfigKeyCursorBlinkTime)
	if err != nil {
		logger.Warning("Failed to get CursorBlink from dconfig:", err)
		return
	}
	kbd.CursorBlink = value
	kbd.applyCursorBlink()
}

func (kbd *Keyboard) updateCapslockToggleFromDConfig() {
	value, err := kbd.dsgKeyboardConfig.GetValueBool(dconfigKeyCapslockToggle)
	if err != nil {
		logger.Warning(err)
	}
	kbd.CapslockToggle = value
}

func (kbd *Keyboard) updateLayoutOptionsFromDConfig() {
	value, err := kbd.dsgKeyboardConfig.GetValue(dconfigKeyLayoutOptions)
	if err != nil {
		logger.Warning(err)
	}
	kbd.UserOptionList = value.([]string)
	kbd.applyOptions()
}

func (kbd *Keyboard) updateLayoutScopeFromDConfig() {
	value, err := kbd.dsgKeyboardConfig.GetValueUInt32(dconfigKeyLayoutScope)
	if err != nil {
		logger.Warning(err)
	}
	kbd.LayoutScope = value
	kbd.applyLayout()
}

// updateUserLayoutListFromDConfig 从 DConfig 更新用户布局列表
func (kbd *Keyboard) updateUserLayoutListFromDConfig() {
	userLayoutList, err := kbd.dsgKeyboardConfig.GetValueStringList(dconfigKeyUserLayoutList)
	if err != nil {
		logger.Warning("Failed to update UserLayoutList from DConfig:", err)
		return
	}
	kbd.UserLayoutList = userLayoutList
	logger.Debug("Updated UserLayoutList from DConfig:", userLayoutList)
}

// updateCurrentLayoutFromDConfig 从 DConfig 更新当前布局
func (kbd *Keyboard) updateCurrentLayoutFromDConfig() {
	currentLayout, err := kbd.dsgKeyboardConfig.GetValueString(dconfigKeyLayout)
	if err != nil {
		logger.Warning("Failed to update CurrentLayout from DConfig:", err)
		return
	}
	kbd.CurrentLayout = currentLayout
	logger.Debug("Updated CurrentLayout from DConfig:", currentLayout)
}
