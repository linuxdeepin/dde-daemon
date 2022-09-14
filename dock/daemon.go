// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/dde-daemon/loader"

	x "github.com/linuxdeepin/go-x11-client"
)

type Daemon struct {
	*loader.ModuleBase
}

const moduleName = "dock"

func NewDaemon(logger *log.Logger) *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase(moduleName, daemon, logger)
	return daemon
}

func (d *Daemon) Stop() error {
	if dockManager != nil {
		dockManager.destroy()
		dockManager = nil
	}

	if globalXConn != nil {
		globalXConn.Close()
		globalXConn = nil
	}

	return nil
}

func (d *Daemon) startFailed() {
	err := d.Stop()
	if err != nil {
		logger.Warning("Stop error:", err)
	}
}

func (d *Daemon) Start() error {
	if dockManager != nil {
		return nil
	}

	var err error

	globalXConn, err = x.NewConn()
	if err != nil {
		d.startFailed()
		return err
	}

	initAtom()
	initDir()
	initPathDirCodeMap()

	service := loader.GetService()

	dockManager, err = newManager(service)
	if err != nil {
		d.startFailed()
		return err
	}

	err = dockManager.service.Export(dbusPath, dockManager, dockManager.syncConfig)
	if err != nil {
		d.startFailed()
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		d.startFailed()
		return err
	}

	err = dockManager.syncConfig.Register()
	if err != nil {
		logger.Warning(err)
	}

	err = service.Emit(dockManager, "ServiceRestarted")
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

func (d *Daemon) GetDependencies() []string {
	return []string{}
}

func (d *Daemon) Name() string {
	return moduleName
}
