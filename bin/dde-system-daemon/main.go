/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"os"

	// modules:
	_ "github.com/linuxdeepin/dde-daemon/accounts"
	_ "github.com/linuxdeepin/dde-daemon/apps"
	_ "github.com/linuxdeepin/dde-daemon/fprintd"
	_ "github.com/linuxdeepin/dde-daemon/image_effect"
	_ "github.com/linuxdeepin/dde-daemon/system/airplane_mode"
	_ "github.com/linuxdeepin/dde-daemon/system/bluetooth"
	_ "github.com/linuxdeepin/dde-daemon/system/display"
	_ "github.com/linuxdeepin/dde-daemon/system/gesture"
	_ "github.com/linuxdeepin/dde-daemon/system/hostname"
	_ "github.com/linuxdeepin/dde-daemon/system/inputdevices"
	_ "github.com/linuxdeepin/dde-daemon/system/keyevent"
	_ "github.com/linuxdeepin/dde-daemon/system/lang"
	_ "github.com/linuxdeepin/dde-daemon/system/network"
	_ "github.com/linuxdeepin/dde-daemon/system/power"
	_ "github.com/linuxdeepin/dde-daemon/system/power_manager"
	_ "github.com/linuxdeepin/dde-daemon/system/scheduler"
	_ "github.com/linuxdeepin/dde-daemon/system/swapsched"
	_ "github.com/linuxdeepin/dde-daemon/system/systeminfo"
	_ "github.com/linuxdeepin/dde-daemon/system/timedated"
	_ "github.com/linuxdeepin/dde-daemon/system/uadp"

	"github.com/linuxdeepin/dde-daemon/loader"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	glib "github.com/linuxdeepin/go-gir/glib-2.0"
	"github.com/linuxdeepin/go-lib/dbusutil"
	. "github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/log"
)

//go:generate dbusutil-gen em -type Daemon

type Daemon struct {
	loginManager  login1.Manager
	systemSigLoop *dbusutil.SignalLoop
	service       *dbusutil.Service
	signals       *struct { //nolint
		HandleForSleep struct {
			start bool
		}
	}
}

const (
	dbusServiceName = "com.deepin.daemon.Daemon"
	dbusPath        = "/com/deepin/daemon/Daemon"
	dbusInterface   = dbusServiceName
)

var logger = log.NewLogger("daemon/dde-system-daemon")
var _daemon *Daemon

func main() {
	service, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Fatal("failed to new system service", err)
	}

	hasOwner, err := service.NameHasOwner(dbusServiceName)
	if err != nil {
		logger.Fatal("failed to call NameHasOwner:", err)
	}
	if hasOwner {
		logger.Warningf("name %q already has the owner", dbusServiceName)
		os.Exit(1)
	}

	// fix no PATH when was launched by dbus
	if os.Getenv("PATH") == "" {
		logger.Warning("No PATH found, manual special")
		err = os.Setenv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
		if err != nil {
			logger.Warning(err)
		}
	}

	// 系统级服务，无需设置LANG和LANGUAGE，保证翻译不受到影响
	_ = os.Setenv("LANG", "")
	_ = os.Setenv("LANGUAGE", "")

	InitI18n()
	BindTextdomainCodeset("dde-daemon", "UTF-8")
	Textdomain("dde-daemon")

	logger.SetRestartCommand("/usr/lib/deepin-daemon/dde-system-daemon")

	_daemon = &Daemon{}
	_daemon.service = service
	err = service.Export(dbusPath, _daemon)
	if err != nil {
		logger.Fatal("failed to export:", err)
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		logger.Fatal("failed to request name:", err)
	}

	startBacklightHelperAsync(service.Conn())
	loader.SetService(service)
	loader.StartAll()
	defer loader.StopAll()

	// NOTE: system/power module requires glib loop
	go glib.StartLoop()

	err = _daemon.forwardPrepareForSleepSignal(service)
	if err != nil {
		logger.Warning(err)
	}
	service.Wait()
}

func (*Daemon) GetInterfaceName() string {
	return dbusInterface
}
