// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"crypto/md5" //#nosec G501
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/wm/ewmh"
	"github.com/linuxdeepin/go-x11-client/util/wm/icccm"
)

const windowHashPrefix = "w:"

type WindowInfo struct {
	baseWindowInfo

	x                        int16
	y                        int16
	width                    uint16
	height                   uint16
	lastConfigureNotifyEvent *x.ConfigureNotifyEvent
	mu                       sync.Mutex
	updateConfigureTimer     *time.Timer

	wmState           []x.Atom
	wmWindowType      []x.Atom
	wmAllowedActions  []x.Atom
	hasXEmbedInfo     bool
	hasWmTransientFor bool
	wmClass           *icccm.WMClass
	wmName            string
	motifWmHints      *MotifWmHints

	gtkAppId     string
	flatpakAppID string
	wmRole       string

	updateCalled bool
	sync.Mutex
}

func NewWindowInfo(win x.Window) *WindowInfo {
	winInfo := &WindowInfo{}
	winInfo.xid = win
	winInfo.createdTime = time.Now().UnixNano()
	return winInfo
}

// window type
func (winInfo *WindowInfo) updateWmWindowType() {
	var err error
	winInfo.wmWindowType, err = ewmh.GetWMWindowType(globalXConn, winInfo.xid).Reply(globalXConn)
	if err != nil {
		logger.Debugf("failed to get WMWindowType for window %d: %v", winInfo.xid, err)
	}
}

// wm allowed actions
func (winInfo *WindowInfo) updateWmAllowedActions() {
	var err error
	winInfo.wmAllowedActions, err = ewmh.GetWMAllowedActions(globalXConn,
		winInfo.xid).Reply(globalXConn)
	if err != nil {
		logger.Debugf("failed to get WMAllowedActions for window %d: %v", winInfo.xid, err)
	}
}

// wm state
func (winInfo *WindowInfo) updateWmState() {
	var err error
	winInfo.wmState, err = ewmh.GetWMState(globalXConn, winInfo.xid).Reply(globalXConn)
	if err != nil {
		logger.Debugf("failed to get WMState for window %d: %v", winInfo.xid, err)
	}
}

// wm class
func (winInfo *WindowInfo) updateWmClass() {
	var err error
	winInfo.wmClass, err = getWmClass(winInfo.xid)
	if err != nil {
		logger.Debugf("failed to get wmClass for window %d: %v", winInfo.xid, err)
	}
}

func (winInfo *WindowInfo) updateMotifWmHints() {
	var err error
	winInfo.motifWmHints, err = getMotifWmHints(globalXConn, winInfo.xid)
	if err != nil {
		logger.Debugf("failed to get Motif WM Hints for window %d: %v",
			winInfo.xid, err)
	}
}

// wm name
func (winInfo *WindowInfo) updateWmName() {
	winInfo.wmName = getWmName(winInfo.xid)
	winInfo.Title = winInfo.getTitle()
}

func (winInfo *WindowInfo) updateIcon() {
	winInfo.Icon = getIconFromWindow(winInfo.xid)
}

// XEmbed info
// 一般 tray icon 会带有 _XEMBED_INFO 属性
func (winInfo *WindowInfo) updateHasXEmbedInfo() {
	reply, err := x.GetProperty(globalXConn, false, winInfo.xid, atomXEmbedInfo, x.AtomAny, 0, 2).Reply(globalXConn)
	if err != nil {
		logger.Debug(err)
		return
	}
	if reply.Format != 0 {
		// has property
		winInfo.hasXEmbedInfo = true
	}
}

// WM_TRANSIENT_FOR
func (winInfo *WindowInfo) updateHasWmTransientFor() {
	_, err := icccm.GetWMTransientFor(globalXConn, winInfo.xid).Reply(globalXConn)
	winInfo.hasWmTransientFor = err == nil
}

func (winInfo *WindowInfo) isActionMinimizeAllowed() bool {
	logger.Debugf("wmAllowedActions: %#v", winInfo.wmAllowedActions)
	return atomsContains(winInfo.wmAllowedActions, atomNetWmActionMinimize)
}

func (winInfo *WindowInfo) hasWmStateDemandsAttention() bool {
	return atomsContains(winInfo.wmState, atomWmStateDemandsAttention)
}

func (winInfo *WindowInfo) hasWmStateSkipTaskBar() bool {
	return atomsContains(winInfo.wmState, atomNetWmStateSkipTaskbar)
}

func (winInfo *WindowInfo) hasWmStateModal() bool {
	return atomsContains(winInfo.wmState, atomNetWmStateModal)
}

func (winInfo *WindowInfo) isValidModal() bool {
	return winInfo.hasWmTransientFor && winInfo.hasWmStateModal()
}

// 通过 wmClass 判断是否需要隐藏此窗口
func (winInfo *WindowInfo) shouldSkipWithWMClass() bool {
	wmClass := winInfo.wmClass
	if wmClass == nil {
		return false
	}
	if wmClass.Instance == "explorer.exe" && wmClass.Class == "Wine" {
		return true
	} else if wmClass.Class == "dde-launcher" {
		return true
	}

	return false
}

func (winInfo *WindowInfo) getDisplayName() (name string) {
	name = winInfo.getDisplayName0()
	nameTitle := strings.Title(name)
	// NOTE: although name is valid, nameTitle is not necessarily valid.
	if utf8.ValidString(nameTitle) {
		name = nameTitle
	}
	return
}

func (winInfo *WindowInfo) getDisplayName0() string {
	win := winInfo.xid
	role := winInfo.wmRole
	if !utf8.ValidString(role) {
		role = ""
	}

	var class, instance string
	if winInfo.wmClass != nil {
		class = winInfo.wmClass.Class
		if !utf8.ValidString(class) {
			class = ""
		}

		instance = filepath.Base(winInfo.wmClass.Instance)
		if !utf8.ValidString(instance) {
			instance = ""
		}
	}
	logger.Debugf("getDisplayName class: %q, instance: %q", class, instance)

	if role != "" && class != "" {
		return class + " " + role
	}

	if class != "" {
		return class
	}

	if instance != "" {
		return instance
	}

	wmName := winInfo.wmName
	if wmName != "" {
		var shortWmName string
		lastIndex := strings.LastIndex(wmName, "-")
		if lastIndex > 0 {
			shortWmName = wmName[lastIndex:]
			if shortWmName != "" && utf8.ValidString(shortWmName) {
				return shortWmName
			}
		}
	}

	if winInfo.process != nil {
		exeBasename := filepath.Base(winInfo.process.exe)
		if utf8.ValidString(exeBasename) {
			return exeBasename
		}
	}

	return fmt.Sprintf("window: %v", win)
}

func (winInfo *WindowInfo) getTitle() string {
	wmName := winInfo.wmName
	if wmName == "" || !utf8.ValidString(wmName) {
		return winInfo.getDisplayName()
	}
	return wmName
}

func (winInfo *WindowInfo) getIcon() string {
	if winInfo.Icon == "" {
		logger.Debug("get icon from window", winInfo.xid)
		winInfo.Icon = getIconFromWindow(winInfo.xid)
	}
	return winInfo.Icon
}

var skipTaskBarWindowTypes = []string{
	"_NET_WM_WINDOW_TYPE_UTILITY",
	"_NET_WM_WINDOW_TYPE_COMBO",
	"_NET_WM_WINDOW_TYPE_DESKTOP",
	"_NET_WM_WINDOW_TYPE_DND",
	"_NET_WM_WINDOW_TYPE_DOCK",
	"_NET_WM_WINDOW_TYPE_DROPDOWN_MENU",
	"_NET_WM_WINDOW_TYPE_MENU",
	"_NET_WM_WINDOW_TYPE_NOTIFICATION",
	"_NET_WM_WINDOW_TYPE_POPUP_MENU",
	"_NET_WM_WINDOW_TYPE_SPLASH",
	"_NET_WM_WINDOW_TYPE_TOOLBAR",
	"_NET_WM_WINDOW_TYPE_TOOLTIP",
}

func (winInfo *WindowInfo) shouldSkip() bool {
	logger.Debugf("win %d shouldSkip?", winInfo.xid)
	if !winInfo.updateCalled {
		winInfo.update()
		winInfo.updateCalled = true
	}

	logger.Debugf("hasXEmbedInfo: %v", winInfo.hasXEmbedInfo)
	logger.Debugf("wmWindowType: %#v", winInfo.wmWindowType)
	logger.Debugf("wmState: %#v", winInfo.wmState)
	logger.Debugf("wmClass: %#v", winInfo.wmClass)

	if winInfo.hasWmStateSkipTaskBar() || winInfo.isValidModal() ||
		winInfo.hasXEmbedInfo || winInfo.shouldSkipWithWMClass() {
		return true
	}

	for _, winType := range winInfo.wmWindowType {
		winTypeStr, _ := getAtomName(winType)
		if winType == atomNetWmWindowTypeDialog &&
			!winInfo.isActionMinimizeAllowed() {
			return true
		} else if strSliceContains(skipTaskBarWindowTypes, winTypeStr) {
			return true
		}
	}
	return false
}

func (winInfo *WindowInfo) updateProcessInfo() {
	win := winInfo.xid
	winInfo.pid = getWmPid(win)
	var err error
	winInfo.process, err = NewProcessInfo(winInfo.pid)
	if err != nil {
		logger.Debug(err)
		// Try WM_COMMAND
		wmCommand, err := getWmCommand(win)
		if err == nil {
			winInfo.process = NewProcessInfoWithCmdline(wmCommand)
		}
	}
	logger.Debugf("process: %#v", winInfo.process)
}

func (winInfo *WindowInfo) update() {
	win := winInfo.xid
	logger.Debugf("update window %v info", win)
	winInfo.updateWmClass()
	winInfo.updateMotifWmHints()
	winInfo.updateWmState()
	winInfo.updateWmWindowType()
	winInfo.updateWmAllowedActions()
	if len(winInfo.wmWindowType) == 0 {
		winInfo.updateHasXEmbedInfo()
	}
	winInfo.updateHasWmTransientFor()
	winInfo.updateProcessInfo()
	winInfo.wmRole = getWmWindowRole(win)
	winInfo.gtkAppId = getWindowGtkApplicationId(win)
	winInfo.flatpakAppID = getWindowFlatpakAppID(win)
	winInfo.updateWmName()
	winInfo.innerId = genInnerId(winInfo)
}

func filterFilePath(args []string) string {
	var filtered []string
	for _, arg := range args {
		if strings.Contains(arg, "/") || arg == "." || arg == ".." {
			filtered = append(filtered, "%F")
		} else {
			filtered = append(filtered, arg)
		}
	}
	return strings.Join(filtered, " ")
}

func genInnerId(winInfo *WindowInfo) string {
	win := winInfo.xid
	var wmClass string
	var wmInstance string
	if winInfo.wmClass != nil {
		wmClass = winInfo.wmClass.Class
		wmInstance = filepath.Base(winInfo.wmClass.Instance)
	}
	var exe string
	var args string
	if winInfo.process != nil {
		exe = winInfo.process.exe
		args = filterFilePath(winInfo.process.args)
	}
	hasPid := winInfo.pid != 0

	var str string
	// NOTE: 不要使用 wmRole，有些程序总会改变这个值比如 GVim
	if wmInstance == "" && wmClass == "" && exe == "" && winInfo.gtkAppId == "" {
		if winInfo.wmName != "" {
			str = fmt.Sprintf("wmName:%q", winInfo.wmName)
		} else {
			str = fmt.Sprintf("windowId:%v", winInfo.xid)
		}
	} else {
		str = fmt.Sprintf("wmInstance:%q,wmClass:%q,exe:%q,args:%q,hasPid:%v,gtkAppId:%q",
			wmInstance, wmClass, exe, args, hasPid, winInfo.gtkAppId)
	}
	// #nosec G401
	md5hash := md5.New()
	_, err := md5hash.Write([]byte(str))
	if err != nil {
		logger.Warning("Write error:", err)
	}
	innerId := windowHashPrefix + hex.EncodeToString(md5hash.Sum(nil))
	logger.Debugf("genInnerId win: %v str: %s, innerId: %s", win, str, innerId)
	return innerId
}

func (winInfo *WindowInfo) isDemandingAttention() bool {
	return winInfo.hasWmStateDemandsAttention()
}

func (winInfo *WindowInfo) isMinimized() bool {
	return atomsContains(winInfo.wmState, atomNetWmStateHidden)
}

func (winInfo *WindowInfo) activate() error {
	return activateWindow(winInfo.xid)
}

func (winInfo *WindowInfo) minimize() error {
	return minimizeWindow(winInfo.xid)
}

func (winInfo *WindowInfo) maximize() error {
	return maximizeWindow(winInfo.xid)
}

func (winInfo *WindowInfo) makeWindowAbove() error {
	return makeWindowAbove(winInfo.xid)
}

func (winInfo *WindowInfo) close(timestamp uint32) error {
	return closeWindow(winInfo.xid, x.Timestamp(timestamp))
}

func (winInfo *WindowInfo) killClient() error {
	return killClient(winInfo.xid)
}

func (winInfo *WindowInfo) changeXid(xid x.Window) bool {
	logger.Warning("XWindowInfo should not change xid!")
	return false
}

func (winInfo *WindowInfo) print() {
	wmClassStr := "-"
	if winInfo.wmClass != nil {
		wmClassStr = fmt.Sprintf("%q %q", winInfo.wmClass.Class, winInfo.wmClass.Instance)
	}
	logger.Infof("id: %d, wmClass: %s, wmState: %v,"+
		" wmWindowType: %v, wmAllowedActions: %v, hasXEmbedInfo: %v, hasWmTransientFor: %v",
		winInfo.xid, wmClassStr,
		winInfo.wmState, winInfo.wmWindowType, winInfo.wmAllowedActions, winInfo.hasXEmbedInfo,
		winInfo.hasWmTransientFor)
}

func (winInfo *WindowInfo) allowClose() bool {
	if winInfo.motifWmHints != nil {
		if winInfo.motifWmHints.allowedClose() {
			return true
		}
	}

	for _, action := range winInfo.wmAllowedActions {
		if action == atomNetWmActionClose {
			return true
		}
	}
	return false
}
