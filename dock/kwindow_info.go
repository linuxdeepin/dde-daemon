// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"time"

	kwayland "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.kwayland"
	x "github.com/linuxdeepin/go-x11-client"
)

type WindowInfoImp interface {
	getXid() x.Window
	setEntry(*AppEntry)
	getEntry() *AppEntry
	shouldSkip() bool
	getEntryInnerId() string
	getInnerId() string
	setEntryInnerId(string)
	getAppInfo() *AppInfo
	setAppInfo(*AppInfo)
	print()
	getDisplayName() string
	getIcon() string
	getTitle() string
	isDemandingAttention() bool
	allowClose() bool
	close(timestamp uint32) error
	getPid() uint
	getProcess() *ProcessInfo
	activate() error
	minimize() error
	maximize() error
	makeWindowAbove() error
	isMinimized() bool
	killClient() error
	changeXid(x.Window) bool
	getCreatedTime() int64
}

type KWindowInfo struct {
	baseWindowInfo
	winObj       kwayland.Window
	updateCalled bool

	appId              string
	internalId         uint32
	demandingAttention bool
	closeable          bool
	minimized          bool
	geometry           kwayland.Rect
}

type baseWindowInfo struct {
	xid          x.Window
	Title        string
	Icon         string
	pid          uint
	entryInnerId string
	innerId      string
	entry        *AppEntry
	appInfo      *AppInfo
	process      *ProcessInfo
	createdTime  int64
}

func (winInfo *KWindowInfo) print() {
	// TODO
	return
}

func (winInfo *KWindowInfo) getDisplayName() string {
	// TODO
	return ""
}

func (winInfo *KWindowInfo) shouldSkip() bool {
	if !winInfo.updateCalled {
		winInfo.update()
		winInfo.updateCalled = true
	}

	skip, err := winInfo.winObj.SkipTaskbar(0)
	if err != nil {
		logger.Warning(err)
		return true
	}
	// + 添加窗口能否最小化判断，如果窗口不能最小化则隐藏任务栏图标
	canMinimize, _ := winInfo.winObj.IsMinimizeable(0)
	if canMinimize == false {
		skip = true
	}

	if skip {
		// + 白名单(临时方案，待窗口增加wayland下窗口规则后再修改)： 修复类似欢迎应用没有最小化窗口,但是需要在任务栏显示图标
		for _, app := range []string{"dde-introduction"} {
			if app == winInfo.appId {
				skip = false
			}
		}
	}
	return skip
}

// baseWindowInfo
func (winInfo *baseWindowInfo) getInnerId() string {
	return winInfo.innerId
}

func (winInfo *baseWindowInfo) getPid() uint {
	return winInfo.pid
}

func (winInfo *baseWindowInfo) getIcon() string {
	return winInfo.Icon
}

func (winInfo *baseWindowInfo) getTitle() string {
	return winInfo.Title
}

func (winInfo *baseWindowInfo) getProcess() *ProcessInfo {
	return winInfo.process
}

func (winInfo *baseWindowInfo) getEntry() *AppEntry {
	return winInfo.entry
}

func (winInfo *baseWindowInfo) setEntry(v *AppEntry) {
	winInfo.entry = v
}

func (winInfo *baseWindowInfo) getEntryInnerId() string {
	return winInfo.entryInnerId
}

func (winInfo *baseWindowInfo) setEntryInnerId(v string) {
	winInfo.entryInnerId = v
}

func (winInfo *baseWindowInfo) getAppInfo() *AppInfo {
	return winInfo.appInfo
}

func (winInfo *baseWindowInfo) setAppInfo(v *AppInfo) {
	winInfo.appInfo = v
}

func (winInfo *baseWindowInfo) getXid() x.Window {
	return winInfo.xid
}

func (winInfo *baseWindowInfo) getCreatedTime() int64 {
	return winInfo.createdTime
}

func newKWindowInfo(winObj kwayland.Window, xid uint32) *KWindowInfo {
	winInfo := &KWindowInfo{
		winObj: winObj,
	}
	winInfo.xid = x.Window(xid)
	winInfo.createdTime = time.Now().UnixNano()
	return winInfo
}

func (winInfo *KWindowInfo) fetchTitle() string {
	title, err := winInfo.winObj.Title(0)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return title
}

func (winInfo *KWindowInfo) updateTitle() {
	winInfo.Title = winInfo.fetchTitle()
}

func (winInfo *KWindowInfo) fetchIcon() string {
	icon, err := winInfo.winObj.Icon(0)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return icon
}

func (winInfo *KWindowInfo) updateIcon() {
	winInfo.Icon = winInfo.fetchIcon()
}

func (winInfo *KWindowInfo) updateGeometry() bool {
	rect, err := winInfo.winObj.Geometry(0)
	if err != nil {
		logger.Warning(err)
		return false
	}
	if winInfo.geometry == rect {
		return false
	}
	winInfo.geometry = rect
	return true
}

func (winInfo *KWindowInfo) allowClose() bool {
	return winInfo.closeable
}

func (winInfo *KWindowInfo) isDemandingAttention() bool {
	return winInfo.demandingAttention
}

func (winInfo *KWindowInfo) updateAppId() {
	appId, err := winInfo.winObj.AppId(0)
	if err != nil {
		logger.Warning(err)
	}
	winInfo.appId = appId
}

func (winInfo *KWindowInfo) update() {
	logger.Debug("update window info", winInfo.winObj.Path_())
	winInfo.updateInternalId()
	winInfo.updateAppId()
	winInfo.updateIcon()
	winInfo.updateTitle()
	winInfo.updateGeometry()
	winInfo.updateDemandingAttention()
	winInfo.updateCloseable()
	winInfo.updateProcessInfo()
}

func (winInfo *KWindowInfo) updateInternalId() {
	id, err := winInfo.winObj.InternalId(0)
	if err != nil {
		logger.Warning(err)
	}
	winInfo.internalId = id
}

func (winInfo *KWindowInfo) updateDemandingAttention() {
	isDA, err := winInfo.winObj.IsDemandingAttention(0)
	if err != nil {
		logger.Warning(err)
	}
	winInfo.demandingAttention = isDA
}

func (winInfo *KWindowInfo) updateCloseable() {
	closeable, err := winInfo.winObj.IsCloseable(0)
	if err != nil {
		logger.Warning(err)
		closeable = true
	}
	winInfo.closeable = closeable
}

func (winInfo *KWindowInfo) updateMinimized() {
	minimized, err := winInfo.winObj.IsMinimized(0)
	if err != nil {
		logger.Warning(err)
	}
	winInfo.minimized = minimized
}

func (winInfo *KWindowInfo) isMinimized() bool {
	return winInfo.minimized
}

func (winInfo *KWindowInfo) updateProcessInfo() {
	pid, err := winInfo.winObj.Pid(0)
	if err != nil {
		logger.Warning(err)
		return
	}
	winInfo.pid = uint(pid)
	winInfo.process, err = NewProcessInfo(winInfo.pid)
	if err != nil {
		logger.Warning(err)
		return
	}
	logger.Debugf("process: %#v", winInfo.process)
}

func (winInfo *KWindowInfo) activate() error {
	return winInfo.winObj.RequestActivate(0)
}

func (winInfo *KWindowInfo) minimize() error {
	return winInfo.winObj.RequestToggleMinimized(0)
}

func (winInfo *KWindowInfo) maximize() error {
	return winInfo.winObj.RequestToggleMaximized(0)
}

func (winInfo *KWindowInfo) makeWindowAbove() error {
	return winInfo.winObj.RequestToggleKeepAbove(0)
}

func (winInfo *KWindowInfo) close(timestamp uint32) error {
	return winInfo.winObj.RequestClose(0)
}

func (winInfo *KWindowInfo) killClient() error {
	return winInfo.winObj.RequestClose(0)
}

func (winInfo *KWindowInfo) changeXid(xid x.Window) bool {
	winInfo.xid = xid
	return true
}
