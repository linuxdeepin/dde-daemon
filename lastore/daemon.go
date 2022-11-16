// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package lastore

import (
	"github.com/linuxdeepin/dde-daemon/loader"
	"github.com/linuxdeepin/go-lib/log"
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
	service := loader.GetService()

	lastoreObj, err := newLastore(service)
	if err != nil {
		return err
	}
	d.lastore = lastoreObj
	err = service.Export(dbusPath, lastoreObj, lastoreObj.syncConfig)
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}
	err = lastoreObj.syncConfig.Register()
	if err != nil {
		logger.Warning("Failed to register sync service:", err)
	}
	agent, err := newAgent(lastoreObj)
	if err != nil {
		logger.Warning(err)
		return err
	}
	d.agent = agent
	agent.init()
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
