// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package lastore

import (
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/powersupply/battery"
	"github.com/linuxdeepin/dde-daemon/common/dsync"
	sessionmanager "github.com/linuxdeepin/go-dbus-factory/session/org.deepin.dde.sessionmanager1"
	notifications "github.com/linuxdeepin/go-dbus-factory/session/org.freedesktop.notifications"
	abrecovery "github.com/linuxdeepin/go-dbus-factory/system/com.deepin.abrecovery"
	lastore "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.lastore1"
	power "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.power1"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/gsettings"
	"github.com/linuxdeepin/go-lib/strv"
)

//go:generate dbusutil-gen em -type Lastore

type Lastore struct {
	service        *dbusutil.Service
	sysSigLoop     *dbusutil.SignalLoop
	sessionSigLoop *dbusutil.SignalLoop
	jobStatus      map[dbus.ObjectPath]CacheJobInfo
	lang           string
	inhibitFd      dbus.UnixFD

	power          power.Power
	core           lastore.Lastore
	sysDBusDaemon  ofdbus.DBus
	notifications  notifications.Notifications
	sessionManager sessionmanager.SessionManager

	syncConfig              *dsync.Config
	settings                *gio.Settings
	rebootTimer             *time.Timer
	updatingLowPowerPercent float64
	updateNotifyId          uint32
	isBackuping             bool
	isUpdating              bool
	// 默认间隔时间2小时,但当设置了稍后提醒时间后,需要修改默认时间,单位分钟
	intervalTime                  uint16
	needShowUpgradeFinishedNotify bool

	notifiedBattery     bool
	notifyIdHidMap      map[uint32]dbusutil.SignalHandlerId
	lastoreRule         dbusutil.MatchRule
	jobsPropsChangedHId dbusutil.SignalHandlerId

	// prop:
	PropsMu            sync.RWMutex
	SourceCheckEnabled bool
}

type CacheJobInfo struct {
	Id       string
	Status   Status
	Name     string
	Progress float64
	Type     string
}

const (
	gsSchemaPower                = "com.deepin.dde.power"
	gsKeyUpdatingLowPowerPercent = "low-power-percent-in-updating-notify"
	intervalTime10Min            = 10
	intervalTime30Min            = 30
	intervalTime120Min           = 120
	intervalTime360Min           = 360
)

func newLastore(service *dbusutil.Service) (*Lastore, error) {
	l := &Lastore{
		service:                       service,
		jobStatus:                     make(map[dbus.ObjectPath]CacheJobInfo),
		inhibitFd:                     -1,
		lang:                          QueryLang(),
		updateNotifyId:                0,
		isUpdating:                    false,
		intervalTime:                  intervalTime120Min,
		needShowUpgradeFinishedNotify: false,
	}

	logger.Debugf("CurrentLang: %q", l.lang)
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}

	l.sysSigLoop = dbusutil.NewSignalLoop(systemBus, 100)
	l.sysSigLoop.Start()

	l.sessionSigLoop = dbusutil.NewSignalLoop(sessionBus, 10)
	l.sessionSigLoop.Start()

	l.sessionManager = sessionmanager.NewSessionManager(sessionBus)

	l.settings = gio.NewSettings(gsSchemaPower)
	l.updatingLowPowerPercent = l.settings.GetDouble(gsKeyUpdatingLowPowerPercent)
	l.notifyGSettingsChanged()
	l.initCore(systemBus)
	l.initNotify(sessionBus)
	l.initSysDBusDaemon(systemBus)
	l.initPower(systemBus)
	l.initABRecovery(systemBus)
	l.listenBattery()

	l.syncConfig = dsync.NewConfig("updater", &syncConfig{l: l},
		l.sessionSigLoop, dbusPath, logger)
	return l, nil
}

func (l *Lastore) initPower(systemBus *dbus.Conn) {
	l.power = power.NewPower(systemBus)
	l.power.InitSignalExt(l.sysSigLoop, true)
	err := l.power.HasBattery().ConnectChanged(func(hasValue bool, hasBattery bool) {
		if !hasBattery {
			l.notifiedBattery = false
		}
	})
	if err != nil {
		logger.Warning(err)
	}

	err = l.power.OnBattery().ConnectChanged(func(hasValue bool, onBattery bool) {
		// 充电状态时，清除低电量横幅提示
		if hasValue && !onBattery {
			l.removeLowBatteryInUpdatingNotify()
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (l *Lastore) initABRecovery(systemBus *dbus.Conn) {
	abRecovery := abrecovery.NewABRecovery(systemBus)
	abRecovery.InitSignalExt(l.sysSigLoop, true)

	err := abRecovery.BackingUp().ConnectChanged(func(hasValue bool, hasBackup bool) {
		if hasValue {
			if hasBackup {
				l.isBackuping = true
				if l.needLowBatteryNotify() {
					l.lowBatteryInUpdatingNotify()
				}
			} else {
				l.isBackuping = false
			}
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (l *Lastore) listenBattery() {
	err := l.power.BatteryPercentage().ConnectChanged(func(hasValue bool, value float64) {
		if hasValue {
			if value > l.updatingLowPowerPercent {
				return
			}

			if l.needLowBatteryNotify() {
				l.lowBatteryInUpdatingNotify()
			}
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (l *Lastore) initNotify(sessionBus *dbus.Conn) {
	l.notifications = notifications.NewNotifications(sessionBus)
	l.notifications.InitSignalExt(l.sessionSigLoop, true)

	l.notifyIdHidMap = make(map[uint32]dbusutil.SignalHandlerId)
	_, err := l.notifications.ConnectNotificationClosed(func(id uint32, reason uint32) {
		logger.Debug("notification closed id", id)
		hid, ok := l.notifyIdHidMap[id]
		if ok {
			logger.Debugf("remove id: %d, hid: %d", id, hid)
			delete(l.notifyIdHidMap, id)

			// delay call removeHandler
			time.AfterFunc(100*time.Millisecond, func() {
				l.notifications.RemoveHandler(hid)
			})
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (l *Lastore) initSysDBusDaemon(systemBus *dbus.Conn) {
	l.sysDBusDaemon = ofdbus.NewDBus(systemBus)
	l.sysDBusDaemon.InitSignalExt(l.sysSigLoop, true)
	_, err := l.sysDBusDaemon.ConnectNameOwnerChanged(
		func(name string, oldOwner string, newOwner string) {
			if name == l.core.ServiceName_() {
				if newOwner == "" {
					l.offline()
				} else {
					l.online()
				}
			}
		})
	if err != nil {
		logger.Warning(err)
	}
}

var allowPathList = []string{
	"/org/deepin/dde/Lastore1/Jobprepare_system_upgrade",
	"/org/deepin/dde/Lastore1/Jobprepare_appstore_upgrade",
	"/org/deepin/dde/Lastore1/Jobprepare_security_upgrade",
	"/org/deepin/dde/Lastore1/Jobprepare_unknown_upgrade",
	"/org/deepin/dde/Lastore1/Jobsystem_upgrade",
	"/org/deepin/dde/Lastore1/Jobappstore_upgrade",
	"/org/deepin/dde/Lastore1/Jobsecurity_upgrade",
	"/org/deepin/dde/Lastore1/Jobunknown_upgrade",
	"/org/deepin/dde/Lastore1/Jobdist_upgrade",
	"/org/deepin/dde/Lastore1/Jobprepare_dist_upgrade",
}

func (l *Lastore) isUpgradeJobType(path dbus.ObjectPath) bool {
	job, ok := l.jobStatus[path]
	if !ok {
		return false
	}
	if !strv.Strv(allowPathList).Contains(string(path)) {
		return false
	}
	if strings.Contains(string(path), "prepare") && strings.Contains(job.Name, "OnlyDownload") {
		return false
	}
	return true
}

func (l *Lastore) checkUpdateNotify(path dbus.ObjectPath) {
	if !l.isUpgradeJobType(path) {
		return
	}

	l.isUpdating = true
	// 当前job处于 end 状态时，不处理
	if l.jobStatus[path].Status == EndStatus {
		l.isUpdating = false
		return
	}

	if l.needLowBatteryNotify() {
		l.lowBatteryInUpdatingNotify()
	}
	if l.jobStatus[path].Id == SystemUpgradeJobType {
		l.needShowUpgradeFinishedNotify = true
	}
	if !l.needShowUpgradeFinishedNotify {
		return
	}

	jobList, err := l.core.Manager().JobList().Get(0)
	if err != nil {
		logger.Warning(err)
	}

	if l.jobStatus[path].Status == SucceedStatus {
		for _, jobPath := range jobList {
			if job, ok := l.jobStatus[jobPath]; ok {
				if l.isUpgradeJobType(jobPath) {
					if job.Status != SucceedStatus && job.Status != EndStatus && job.Status != FailedStatus {
						return
					}
					if strings.Contains(string(jobPath), "prepare") {
						return
					}
				}
			}
		}

		l.removeLowBatteryInUpdatingNotify()
		l.updateSucceedNotify(l.createUpdateSucceedActions())
		l.isUpdating = false
		l.needShowUpgradeFinishedNotify = false
	}
}

func (l *Lastore) needLowBatteryNotify() bool {
	if l.updateNotifyId != 0 {
		return false
	}

	// 横幅未弹出，且处于ABRecovery或update状态时才弹出横幅
	if l.isUpdating || l.isBackuping {
		batteryStatus, _ := l.power.BatteryStatus().Get(0)
		hasBattery, _ := l.power.HasBattery().Get(0)
		onBattery, _ := l.power.OnBattery().Get(0)
		percent, _ := l.power.BatteryPercentage().Get(0)
		if hasBattery && onBattery && percent < l.updatingLowPowerPercent &&
			batteryStatus != uint32(battery.StatusCharging) {
			return true
		}
	}

	return false
}

func (l *Lastore) initCore(systemBus *dbus.Conn) {
	l.core = lastore.NewLastore(systemBus)
	l.lastoreRule = dbusutil.NewMatchRuleBuilder().
		Sender(l.core.ServiceName_()).
		Type("signal").
		Interface("org.freedesktop.DBus.Properties").
		Member("PropertiesChanged").
		Build()
	err := l.lastoreRule.AddTo(systemBus)
	if err != nil {
		logger.Warning(err)
	}

	l.core.InitSignalExt(l.sysSigLoop, false)
	err = l.core.Manager().JobList().ConnectChanged(func(hasValue bool, value []dbus.ObjectPath) {
		if !hasValue {
			return
		}
		l.updateJobList(value)
	})
	if err != nil {
		logger.Warning(err)
	}

	l.jobsPropsChangedHId = l.sysSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "org.freedesktop.DBus.Properties.PropertiesChanged",
	}, func(sig *dbus.Signal) {
		if len(sig.Body) != 3 {
			return
		}
		props, _ := sig.Body[1].(map[string]dbus.Variant)
		ifc, _ := sig.Body[0].(string)
		if ifc == "org.deepin.dde.Lastore1.Job" {
			l.updateCacheJobInfo(sig.Path, props)
		}
	})

	jobList, err := l.core.Manager().JobList().Get(0)
	if err != nil {
		logger.Warning(err)
	}

	l.updateJobList(jobList)
}

func (l *Lastore) destroy() {
	l.sessionSigLoop.Stop()
	l.sysSigLoop.Stop()
	l.syncConfig.Destroy()

	systemBus := l.sysSigLoop.Conn()
	err := l.lastoreRule.RemoveFrom(systemBus)
	if err != nil {
		logger.Warning(err)
	}

	l.sysSigLoop.RemoveHandler(l.jobsPropsChangedHId)
	l.power.RemoveHandler(proxy.RemoveAllHandlers)
	l.core.RemoveHandler(proxy.RemoveAllHandlers)
	l.notifications.RemoveHandler(proxy.RemoveAllHandlers)
}

func (l *Lastore) GetInterfaceName() string {
	return "org.deepin.dde.LastoreSessionHelper1"
}

// updateJobList clean invalid cached Job status
// The list is the newest JobList.
func (l *Lastore) updateJobList(list []dbus.ObjectPath) {
	var invalids []dbus.ObjectPath
	for jobPath := range l.jobStatus {
		safe := false
		for _, p := range list {
			if p == jobPath {
				safe = true
				break
			}
		}
		if !safe {
			invalids = append(invalids, jobPath)
		}
	}
	for _, jobPath := range invalids {
		delete(l.jobStatus, jobPath)
	}
	logger.Debugf("UpdateJobList: %v - %v", list, invalids)
}

func (l *Lastore) offline() {
	logger.Info("Lastore.Daemon Offline")
	l.jobStatus = make(map[dbus.ObjectPath]CacheJobInfo)
}

func (l *Lastore) online() {
	logger.Info("Lastore.Daemon Online")
}

func (l *Lastore) createJobFailedActions(jobId string) []NotifyAction {
	ac := []NotifyAction{
		{
			Id:   "retry",
			Name: gettext.Tr("Retry"),
			Callback: func() {
				err := l.core.Manager().StartJob(dbus.FlagNoAutoStart, jobId)
				logger.Infof("StartJob %q : %v", jobId, err)
			},
		},
		{
			Id:   "cancel",
			Name: gettext.Tr("Cancel"),
			Callback: func() {
				err := l.core.Manager().CleanJob(dbus.FlagNoAutoStart, jobId)
				logger.Infof("CleanJob %q : %v", jobId, err)
			},
		},
	}
	return ac
}

func (l *Lastore) createUpdateActions() []NotifyAction {
	ac := []NotifyAction{
		{
			Id:   "update",
			Name: gettext.Tr("Update Now"),
			Callback: func() {
				go func() {
					err := exec.Command("dde-control-center", "-m", "update", "-p", "Checking").Run()
					if err != nil {
						logger.Warningf("createUpdateActions: %v", err)
					}
				}()
			},
		},
	}

	return ac
}

func (l *Lastore) resetUpdateSucceedNotifyTimer(intervalTimeMinute uint16) {
	l.intervalTime = intervalTimeMinute
	if l.rebootTimer == nil {
		l.rebootTimer = time.AfterFunc(time.Minute*time.Duration(intervalTimeMinute), func() {
			l.updateSucceedNotify(l.createUpdateSucceedActions())
		})
	} else {
		l.rebootTimer.Reset(time.Minute * time.Duration(intervalTimeMinute))
	}
}

func (l *Lastore) createUpdateSucceedActions() []NotifyAction {
	ac := []NotifyAction{
		{
			Id:   notifyActKeyRebootNow,
			Name: gettext.Tr("Reboot Now"),
		},
		{
			Id:   notifyActKeyRebootLater,
			Name: gettext.Tr("Remind Me Later"),
			Callback: func() {
				l.resetUpdateSucceedNotifyTimer(intervalTime120Min)
			},
		},
		{
			Id:   notifyActKeyReboot10Minutes,
			Name: gettext.Tr("10 mins later"),
			Callback: func() {
				l.resetUpdateSucceedNotifyTimer(intervalTime10Min)
			},
		},
		{
			Id:   notifyActKeyReboot30Minutes,
			Name: gettext.Tr("30 mins later"),
			Callback: func() {
				l.resetUpdateSucceedNotifyTimer(intervalTime30Min)
			},
		},
		{
			Id:   notifyActKeyReboot2Hours,
			Name: gettext.Tr("2h later"),
			Callback: func() {
				l.resetUpdateSucceedNotifyTimer(intervalTime120Min)
			},
		},
		{
			Id:   notifyActKeyReboot6Hours,
			Name: gettext.Tr("6h later"),
			Callback: func() {
				l.resetUpdateSucceedNotifyTimer(intervalTime360Min)
			},
		},
	}

	return ac
}

func (l *Lastore) notifyJob(path dbus.ObjectPath) {
	l.checkBattery()

	info := l.jobStatus[path]
	status := info.Status
	logger.Debugf("notifyJob: %q %q --> %v", path, status, info)
	if info.Name == "uos-release-note" {
		logger.Debug("do not notify when package name is uos-release-note")
		return
	}
	switch guestJobTypeFromPath(path) {
	case InstallJobType:
		switch status {
		case FailedStatus:
			l.notifyInstall(info.Name, false, l.createJobFailedActions(info.Id))
		case SucceedStatus:
			if info.Progress == 1 {
				l.notifyInstall(info.Name, true, nil)
			}
		}
	case RemoveJobType:
		switch status {
		case FailedStatus:
			l.notifyRemove(info.Name, false, l.createJobFailedActions(info.Id))
		case SucceedStatus:
			l.notifyRemove(info.Name, true, nil)
		}

	case CleanJobType:
		if status == SucceedStatus &&
			strings.Contains(info.Name, "+notify") {
			l.notifyAutoClean()
		}
	case UpdateSourceJobType, CustomUpdateJobType:
		val, _ := l.core.Updater().UpdatablePackages().Get(0)
		if status == SucceedStatus && len(val) > 0 &&
			strings.Contains(info.Name, "+notify") {
			l.notifyUpdateSource(l.createUpdateActions())
		}
	}
}

func (*Lastore) IsDiskSpaceSufficient() (result bool, busErr *dbus.Error) {
	avail, err := queryVFSAvailable("/")
	if err != nil {
		return false, dbusutil.ToError(err)
	}
	return avail > 1024*1024*10 /* 10 MB */, nil
}

func (l *Lastore) updateCacheJobInfo(path dbus.ObjectPath, props map[string]dbus.Variant) {
	info := l.jobStatus[path]
	oldStatus := info.Status

	systemBus := l.sysSigLoop.Conn()
	job, err := lastore.NewJob(systemBus, path)
	if err != nil {
		logger.Warning(err)
		return
	}

	if info.Id == "" {
		if v, ok := props["Id"]; ok {
			info.Id, _ = v.Value().(string)
		}
		if info.Id == "" {
			id, _ := job.Id().Get(dbus.FlagNoAutoStart)
			info.Id = id
		}
	}

	if info.Name == "" {
		if v, ok := props["Name"]; ok {
			info.Name, _ = v.Value().(string)
		}
		if info.Name == "" {
			name, _ := job.Name().Get(dbus.FlagNoAutoStart)

			if name == "" {
				pkgs, _ := job.Packages().Get(dbus.FlagNoAutoStart)
				if len(pkgs) == 0 {
					name = "unknown"
				} else {
					name = PackageName(pkgs[0], l.lang)
				}
			}

			info.Name = name
		}
	}

	if v, ok := props["Progress"]; ok {
		info.Progress, _ = v.Value().(float64)
	}

	if v, ok := props["Status"]; ok {
		status := v.Value().(string)
		info.Status = Status(status)
	}

	if info.Type == "" {
		if v, ok := props["Type"]; ok {
			info.Type, _ = v.Value().(string)
		}
		if info.Type == "" {
			info.Type, _ = job.Type().Get(dbus.FlagNoAutoStart)
		}
	} else {
		if v, ok := props["Type"]; ok {
			info.Type, _ = v.Value().(string)
		}
	}
	l.jobStatus[path] = info
	logger.Debugf("updateCacheJobInfo: %#v", info)

	if oldStatus != info.Status {
		l.notifyJob(path)
		l.checkUpdateNotify(path)
	}
}

// guestJobTypeFromPath guest the JobType from object path
// We can't get the JobType when the DBusObject destroyed.
func guestJobTypeFromPath(path dbus.ObjectPath) string {
	_path := string(path)
	for _, jobType := range []string{
		// job types:
		InstallJobType, DownloadJobType, RemoveJobType,
		UpdateSourceJobType, DistUpgradeJobType, CleanJobType, CustomUpdateJobType,
	} {
		if strings.Contains(_path, jobType) {
			return jobType
		}
	}
	return ""
}

var MinBatteryPercent = 30.0

func (l *Lastore) checkBattery() {
	if l.notifiedBattery {
		return
	}
	hasBattery, _ := l.power.HasBattery().Get(0)
	onBattery, _ := l.power.OnBattery().Get(0)
	percent, _ := l.power.BatteryPercentage().Get(0)
	if hasBattery && onBattery && percent <= MinBatteryPercent {
		l.notifiedBattery = true
		l.notifyLowPower()
	}
}

func (l *Lastore) notifyGSettingsChanged() {
	if l.settings == nil {
		l.updatingLowPowerPercent = 50
		logger.Warning("failed to get gsetting")
		return
	}

	gsettings.ConnectChanged(gsSchemaPower, "*", func(key string) {
		switch key {
		case gsKeyUpdatingLowPowerPercent:
			l.updatingLowPowerPercent = l.settings.GetDouble(key)
			return
		default:
			return
		}
	})
}
