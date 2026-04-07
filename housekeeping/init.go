// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package housekeeping

import (
	"os"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/loader"
	notifications "github.com/linuxdeepin/go-dbus-factory/session/org.freedesktop.notifications"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
	. "github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/utils"
)

const (
	// 500MB
	fsMinLeftSpace = 1024 * 1024 * 500
)

func init() {
	loader.Register(NewDaemon(logger))
}

type Daemon struct {
	*loader.ModuleBase
	ticker   *time.Ticker
	stopChan chan struct{}
}

func NewDaemon(logger *log.Logger) *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("housekeeping", daemon, logger)
	return daemon
}

func (*Daemon) GetDependencies() []string {
	return []string{}
}

var (
	logger = log.NewLogger("housekeeping")
)

func (d *Daemon) Start() error {
	if d.stopChan != nil {
		return nil
	}

	d.ticker = time.NewTicker(time.Minute * 1)
	d.stopChan = make(chan struct{})
	go func() {
		for {
			select {
			case _, ok := <-d.ticker.C:
				if !ok {
					logger.Error("Invalid ticker event")
					return
				}

				if !d.checkSpace("HOME", true) {
					break
				}
				if !d.checkSpace("/tmp", false) {
					break
				}
			case <-d.stopChan:
				logger.Debug("Stop housekeeping")
				if d.ticker != nil {
					d.ticker.Stop()
					d.ticker = nil
				}
				return
			}
		}
	}()
	return nil
}

func (d *Daemon) Stop() error {
	if d.stopChan != nil {
		close(d.stopChan)
		d.stopChan = nil
	}
	return nil
}

func sendNotify(icon, summary, body string) error {
	sessionConn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	notifier := notifications.NewNotifications(sessionConn)
	_, err = notifier.Notify(0, "dde-control-center", 0,
		icon, summary, body,
		nil, nil, -1)
	return err
}

func sendNotify2(icon, summary, body, action, call string, timeout int32) error {
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	notifier := notifications.NewNotifications(sessionBus)
	_, err = notifier.Notify(0, "dde-control-center", 0,
		icon, summary, body,
		[]string{"_dbus", action},
		map[string]dbus.Variant{
			"x-deepin-action-_dbus":       dbus.MakeVariant(call),
			"x-deepin-ClickToDisappear":   dbus.MakeVariant(false),
			"x-deepin-DisappearAfterLock": dbus.MakeVariant(false),
		}, timeout)
	return err
}

func isHMIScreenServiceAvailable() bool {
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning("Failed to connect session bus:", err)
		return false
	}

	dbusDaemon := ofdbus.NewDBus(sessionBus)

	names, err := dbusDaemon.ListActivatableNames(0)
	if err != nil {
		logger.Warning("Failed to list activatable names:", err)
		return false
	}

	for _, name := range names {
		if name == "com.deepin.defender.hmiscreen" {
			return true
		}
	}

	return false
}

func (d *Daemon) checkSpace(dir string, state bool) bool {
	if state {
		dir = os.Getenv(dir)
	}
	fs, err := utils.QueryFilesytemInfo(dir)
	if err != nil {
		logger.Error("Failed to get filesystem info for :", dir, err)
		return false
	}

	if fs.AvailSize > fsMinLeftSpace {
		logger.Debug("Sufficient space for:", dir)
		return true
	}
	logger.Info("checkSpace fs.AvailSize(M) : ", dir, fs.AvailSize/1024/1024)
	
	var notifyErr error
	// 通过判断 hmiscreen D-Bus 服务是否存在，决定是否展示“前往清理”动作。
	if isHMIScreenServiceAvailable() {
		notifyErr = sendNotify2("dialog-warning", "",
			Tr("Insufficient disk space, please clean up in time!"),
			Tr("Go to clean up"),
			"dbus-send,--type=method_call,--dest=com.deepin.defender.hmiscreen,/com/deepin/defender/hmiscreen,com.deepin.defender.hmiscreen.ShowModule,string:diskcleaner",
			5000,
		)
	} else {
		notifyErr = sendNotify("dialog-warning", "",
			Tr("Insufficient disk space, please clean up in time!"),
		)
	}

	if notifyErr != nil {
		logger.Warning("Failed to send notification for", dir, ":", notifyErr)
	}
	return false
}
