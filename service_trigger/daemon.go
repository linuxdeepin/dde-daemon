/*
 *  Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:
 *
 * Maintainer:
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

package service_trigger

import (
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/dde-daemon/loader"
)

func init() {
	loader.Register(NewDaemon())
}

var logger = log.NewLogger("daemon/" + moduleName)

const moduleName = "service-trigger"

type Daemon struct {
	*loader.ModuleBase
	manager *Manager
}

func NewDaemon() *Daemon {
	d := &Daemon{}
	d.ModuleBase = loader.NewModuleBase(moduleName, d, logger)
	return d
}

func (d *Daemon) Start() error {
	service := loader.GetService()
	m := newManager(service)
	err := m.start()
	if err != nil {
		return err
	}
	d.manager = m
	return nil
}

func (d *Daemon) Stop() error {
	if d.manager != nil {
		err := d.manager.stop()
		if err != nil {
			return err
		}
		d.manager = nil
	}
	return nil
}

func (*Daemon) GetDependencies() []string {
	return []string{}
}
