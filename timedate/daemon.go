/*
 * Copyright (C) 2013 ~ 2018 Deepin Technology Co., Ltd.
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

package timedate

import (
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/dde-daemon/loader"
)

var (
	logger = log.NewLogger("daemon/timedate")
)

type Daemon struct {
	*loader.ModuleBase
	manager *Manager
	managerFormat *ManagerFormat
}

func NewDaemon(logger *log.Logger) *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("timedate", daemon, logger)
	return daemon
}

func (d *Daemon) GetDependencies() []string {
	return []string{}
}

// Start to run time date manager
func (d *Daemon) Start() error {
	if d.manager != nil {
		return nil
	}
	service := loader.GetService()

	var err error
	d.manager, err = NewManager(service)
	if err != nil {
		return err
	}

	err = service.Export(dbusPath, d.manager)
	if err != nil {
		return err
	}

	d.managerFormat, err = newManagerFormat(service)
	if err != nil {
		return err
	}

	err = service.Export(dbusFormatPath, d.managerFormat)
	if err != nil {
		return err
	}

	err = d.managerFormat.initPropertyWriteCallback(service)
	if err != nil {
		logger.Warning("call SetWriteCallback err:", err)
	} else {
		logger.Info("call SetWriteCallback successful.")
	}

	err = service.RequestName(dbusFormatServiceName)
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	go func() {
		d.manager.init()
	}()

	go func() {
		d.managerFormat.init()
	}()

	return nil
}

// Stop the time date manager
func (d *Daemon) Stop() error {
	if d.manager == nil {
		return nil
	}

	service := loader.GetService()
	err := service.ReleaseName(dbusServiceName)
	if err != nil {
		return err
	}

	err = service.StopExport(d.manager)
	if err != nil {
		return err
	}

	d.manager.destroy()
	d.manager = nil

	d.managerFormat.destroy()
	d.managerFormat = nil
	return nil
}
