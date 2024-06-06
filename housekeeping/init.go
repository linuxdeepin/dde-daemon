// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package housekeeping

import (
	"os"
	"time"

	"github.com/godbus/dbus/v5"
	notifications "github.com/linuxdeepin/go-dbus-factory/session/org.freedesktop.notifications"
	. "github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/dde-daemon/loader"
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

				fs, err := utils.QueryFilesytemInfo(os.Getenv("HOME"))
				if err != nil {
					logger.Error("Failed to get filesystem info:", err)
					break
				}
				logger.Debug("Home filesystem info(total, free, avail):",
					fs.TotalSize, fs.FreeSize, fs.AvailSize)
				if fs.AvailSize > fsMinLeftSpace {
					break
				}
				err = sendNotify("dialog-warning", "",
					Tr("Insufficient disk space, please clean up in time!"))
				if err != nil {
					logger.Warning(err)
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
	_, err = notifier.Notify(0, Tr("dde-control-center"), 0,
		icon, summary, body,
		nil, nil, -1)
	return err
}
