/*
 * Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:     zsien <i@zsien.cn>
 *
 * Maintainer: zsien <i@zsien.cn>
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
package inputdevices

import (
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/log"
)

var logger = log.NewLogger("daemon/system/inputdevices")

const (
	dbusServiceName = "com.deepin.system.InputDevices"
	dbusPath        = "/com/deepin/system/InputDevices"
	dbusInterface   = "com.deepin.system.InputDevices"
)

func init() {
	loader.Register(newDaemon())
}

type daemon struct {
	*loader.ModuleBase
	inputdevices *InputDevices
}

func newDaemon() *daemon {
	d := new(daemon)
	d.ModuleBase = loader.NewModuleBase("inputdevices", d, logger)
	return d
}

func (d *daemon) GetDependencies() []string {
	return []string{}
}

func (d *daemon) Start() error {
	if d.inputdevices != nil {
		return nil
	}

	logger.Debug("start inputdevices")
	d.inputdevices = newInputDevices()

	service := loader.GetService()
	d.inputdevices.service = service
	d.inputdevices.init()

	err := service.Export(dbusPath, d.inputdevices)
	if err != nil {
		logger.Warning(err)
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		logger.Warning(err)
		return err
	}

	return nil
}

func (d *daemon) Stop() error {
	return nil
}
