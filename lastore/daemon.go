// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package lastore

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/godbus/dbus"
	abrecovery "github.com/linuxdeepin/go-dbus-factory/com.deepin.abrecovery"
	lastore "github.com/linuxdeepin/go-dbus-factory/com.deepin.lastore"
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	notifications "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.notifications"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/log"

	"github.com/linuxdeepin/dde-daemon/loader"
)

const (
	dbusPath        = "/com/deepin/LastoreSessionHelper"
	dbusServiceName = "com.deepin.LastoreSessionHelper"
)

const (
	dSettingsAppID            = "org.deepin.lastore"
	dSettingsLastoreName      = "org.deepin.lastore"
	dSettingsKeyUpgradeStatus = "upgrade-status"
)

const (
	UpgradeReady   = "ready"
	UpgradeRunning = "running"
	UpgradeFailed  = "failed"
)

type UpgradeReasonType string

const (
	NoError                      UpgradeReasonType = "NoError"
	ErrorUnknown                 UpgradeReasonType = "ErrorUnknown"
	ErrorFetchFailed             UpgradeReasonType = "fetchFailed"
	ErrorDpkgError               UpgradeReasonType = "dpkgError"
	ErrorPkgNotFound             UpgradeReasonType = "pkgNotFound"
	ErrorUnmetDependencies       UpgradeReasonType = "unmetDependencies"
	ErrorNoInstallationCandidate UpgradeReasonType = "noInstallationCandidate"
	ErrorInsufficientSpace       UpgradeReasonType = "insufficientSpace"
	ErrorUnauthenticatedPackages UpgradeReasonType = "unauthenticatedPackages"
	ErrorOperationNotPermitted   UpgradeReasonType = "operationNotPermitted"
	ErrorIndexDownloadFailed     UpgradeReasonType = "IndexDownloadFailed"
)

const (
	NotifyExpireTimeoutDefault = -1
	NotifyExpireTimeoutNoHide  = 0
)

var logger = log.NewLogger("daemon/lastore")

func init() {
	loader.Register(newDaemon())
}

type Daemon struct {
	lastore *Lastore
	agent   *Agent
	*loader.ModuleBase
}

func newDaemon() *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("lastore", daemon, logger)
	return daemon
}

func (*Daemon) GetDependencies() []string {
	return []string{}
}

func (d *Daemon) Start() error {
	var lastoreOnce sync.Once
	service := loader.GetService()
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return err
	}
	sysDBusDaemon := ofdbus.NewDBus(sysBus)
	systemSigLoop := dbusutil.NewSignalLoop(sysBus, 10)
	systemSigLoop.Start()
	initLastore := func() error {
		lastoreObj, err := newLastore(service)
		if err != nil {
			logger.Warning(err)
			return err
		}
		d.lastore = lastoreObj
		err = service.Export(dbusPath, lastoreObj, lastoreObj.syncConfig)
		if err != nil {
			logger.Warning(err)
			return err
		}

		err = service.RequestName(dbusServiceName)
		if err != nil {
			logger.Warning(err)
			return err
		}
		err = lastoreObj.syncConfig.Register()
		if err != nil {
			logger.Warning("Failed to register sync service:", err)
		}
		defer func() {
			sysDBusDaemon.RemoveAllHandlers()
			systemSigLoop.Stop()
		}()
		agent, err := newAgent(lastoreObj)
		if err != nil {
			logger.Warning(err)
			return err
		}
		d.agent = agent
		return agent.init()

	}
	// 处理更新失败/中断的记录
	go func() {
		// 获取lastore-daemon记录的更新状态
		ds := ConfigManager.NewConfigManager(sysBus)
		dsPath, err := ds.AcquireManager(0, dSettingsAppID, dSettingsLastoreName, "")
		if err != nil {
			logger.Warning(err)
			return
		}
		dsLastoreManager, err := ConfigManager.NewManager(sysBus, dsPath)
		if err != nil {
			logger.Warning(err)
			return
		}
		v, err := dsLastoreManager.Value(0, dSettingsKeyUpgradeStatus)
		if err != nil {
			logger.Warning(err)
			return
		} else {
			upgradeStatus := struct {
				Status     string
				ReasonCode UpgradeReasonType
			}{}
			statusContent := v.Value().(string)
			err = json.Unmarshal([]byte(statusContent), &upgradeStatus)
			if err != nil {
				logger.Warning(err)
				return
			}
			// 记录处理异常更新的通知
			osd := notifications.NewNotifications(service.Conn())
			abObj := abrecovery.NewABRecovery(service.Conn())
			valid, err := abObj.ConfigValid().Get(0) // config失效时,无法回滚,提示重新更新
			var msg string
			switch upgradeStatus.Status {
			// running状态,更新被中断
			case UpgradeRunning:
				if err != nil || !valid {
					msg = gettext.Tr("upgrade abort,need try to upgrade again")
				} else {
					msg = gettext.Tr("upgrade abort,need rollback")
				}
			case UpgradeFailed:
				switch upgradeStatus.ReasonCode {
				case ErrorDpkgError:
					if err != nil || !valid {
						msg = gettext.Tr("upgrade failed because an error occurred during dpkg installation")
					} else {
						msg = gettext.Tr("upgrade failed because an error occurred during dpkg installation, need rollback")
					}
				case ErrorPkgNotFound:
					if err != nil || !valid {
						msg = gettext.Tr("upgrade failed because unable to locate package")
					} else {
						msg = gettext.Tr("upgrade failed because unable to locate package, need rollback")
					}
				case ErrorNoInstallationCandidate:
					if err != nil || !valid {
						msg = gettext.Tr("upgrade failed because has no installation candidate")
					} else {
						msg = gettext.Tr("upgrade failed because has no installation candidate, need rollback")
					}
				case ErrorUnknown:
					if err != nil || !valid {
						msg = gettext.Tr("upgrade failed")
					} else {
						msg = gettext.Tr("upgrade failed, need rollback")
					}
				}
			}
			if len(msg) != 0 {
				_, _ = osd.Notify(0, "dde-control-center", 0, "preferences-system", "", msg, nil, nil, NotifyExpireTimeoutNoHide)
			}
			// 通知发完之后,恢复配置文件的内容
			upgradeStatus.Status = UpgradeReady
			upgradeStatus.ReasonCode = NoError
			v, err := json.Marshal(upgradeStatus)
			if err != nil {
				logger.Warning(err)
			} else {
				err = dsLastoreManager.SetValue(0, dSettingsKeyUpgradeStatus, dbus.MakeVariant(v))
				if err != nil {
					logger.Warning(err)
				}
			}
		}
	}()
	time.AfterFunc(10*time.Minute, func() {
		lastoreOnce.Do(func() {
			err := initLastore()
			if err != nil {
				logger.Warning(err)
			}
		})
	})
	core := lastore.NewLastore(sysBus)
	sysDBusDaemon.InitSignalExt(systemSigLoop, true)
	_, err = sysDBusDaemon.ConnectNameOwnerChanged(func(name, oldOwner, newOwner string) {
		if name == core.ServiceName_() && newOwner != "" {
			lastoreOnce.Do(func() {
				err := initLastore()
				if err != nil {
					logger.Warning(err)
				}
			})
		}
	})
	if err != nil {
		logger.Warning(err)
	}
	hasOwner, err := sysDBusDaemon.NameHasOwner(0, core.ServiceName_())
	if err != nil {
		logger.Warning(err)
	} else if hasOwner {
		lastoreOnce.Do(func() {
			err := initLastore()
			if err != nil {
				logger.Warning(err)
			}
		})
	}
	return nil
}

func (d *Daemon) Stop() error {
	if d.lastore != nil {
		service := loader.GetService()
		err := service.ReleaseName(dbusServiceName)
		if err != nil {
			logger.Warning(err)
		}
		d.lastore.destroy()
		err = service.StopExport(d.lastore)
		if err != nil {
			logger.Warning(err)
		}

		d.lastore = nil
	}

	if d.agent != nil {
		service := loader.GetService()
		d.agent.destroy()
		err := service.StopExport(d.agent)
		if err != nil {
			logger.Warning(err)
		}
		d.agent = nil
	}
	return nil
}
