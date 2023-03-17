// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package lastore

import (
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/loader"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/session/org.freedesktop.dbus"
	lastore "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.lastore1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

const (
	dbusPath        = "/org/deepin/dde/LastoreSessionHelper1"
	dbusServiceName = "org.deepin.dde.LastoreSessionHelper1"
)

var logger = log.NewLogger("daemon/lastore")

func init() {
	loader.Register(newDaemon())
}

type Daemon struct {
	lastore *Lastore
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
	initLastore := func() {
		lastoreObj, err := newLastore(service)
		if err != nil {
			logger.Warning(err)
			return
		}
		d.lastore = lastoreObj
		err = service.Export(dbusPath, lastoreObj, lastoreObj.syncConfig)
		if err != nil {
			logger.Warning(err)
			return
		}
		err = service.RequestName(dbusServiceName)
		if err != nil {
			logger.Warning(err)
			return
		}
		err = lastoreObj.syncConfig.Register()
		if err != nil {
			logger.Warning("Failed to register sync service:", err)
		}
		sysDBusDaemon.RemoveAllHandlers()
		systemSigLoop.Stop()
	}
	time.AfterFunc(10*time.Minute, func() {
		lastoreOnce.Do(initLastore)
	})
	core := lastore.NewLastore(sysBus)
	sysDBusDaemon.InitSignalExt(systemSigLoop, true)
	_, err = sysDBusDaemon.ConnectNameOwnerChanged(func(name, oldOwner, newOwner string) {
		if name == core.ServiceName_() && newOwner != "" {
			lastoreOnce.Do(initLastore)
		}
	})
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

func (d *Daemon) Stop() error {
	if d.lastore == nil {
		return nil
	}

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
	return nil
}
