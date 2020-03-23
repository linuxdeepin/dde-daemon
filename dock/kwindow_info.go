package dock

import (
	kwayland "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.kwayland"
	x "github.com/linuxdeepin/go-x11-client"
)

type WindowInfo interface {
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
	isMinimized() bool
	killClient() error
}

type KWindowInfo struct {
	baseWindowInfo
	winObj       *kwayland.Window
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

func newKWindowInfo(winObj *kwayland.Window, xid uint32) *KWindowInfo {
	winInfo := &KWindowInfo{
		winObj: winObj,
	}
	winInfo.xid = x.Window(xid)
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
		logger.Debug(err)
	}
	logger.Debugf("process: %#v", winInfo.process)
}

func (winInfo *KWindowInfo) activate() error {
	return winInfo.winObj.RequestActivate(0)
}

func (winInfo *KWindowInfo) minimize() error {
	return winInfo.winObj.RequestToggleMinimized(0)
}

func (winInfo *KWindowInfo) close(timestamp uint32) error {
	return winInfo.winObj.RequestClose(0)
}

func (winInfo *KWindowInfo) killClient() error {
	return nil
}
