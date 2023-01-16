// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package lastore

import (
	"sync"
	"time"

	"github.com/godbus/dbus"
	lastore "github.com/linuxdeepin/go-dbus-factory/com.deepin.lastore"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"

	"github.com/linuxdeepin/dde-daemon/loader"
)

const (
	dbusPath        = "/com/deepin/LastoreSessionHelper"
	dbusServiceName = "com.deepin.LastoreSessionHelper"
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
