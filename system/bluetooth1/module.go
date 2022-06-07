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

package bluetooth1

import (
	"github.com/linuxdeepin/dde-daemon/common/sessionmsg"
	"github.com/linuxdeepin/dde-daemon/loader"
	"github.com/linuxdeepin/go-lib/log"
)

type module struct {
	*loader.ModuleBase
}

func newBluetoothModule(logger *log.Logger) *module {
	var d = new(module)
	d.ModuleBase = loader.NewModuleBase("bluetooth1", d, logger)
	return d
}

func (*module) GetDependencies() []string {
	return nil
}

var _bt *SysBluetooth

func (m *module) Start() error {
	return nil

	if _bt != nil {
		return nil
	}

	service := loader.GetService()
	_bt = newSysBluetooth(service)

	sessionmsg.SetAgentInfoPublisher(_bt.userAgents)

	err := service.Export(dbusPath, _bt)
	if err != nil {
		logger.Warning("failed to export bluetooth:", err)
		_bt = nil
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	_bt.agent = newAgent(service)
	_bt.agent.b = _bt

	err = service.Export(agentDBusPath, _bt.agent)
	if err != nil {
		logger.Warning("failed to export agent:", err)
		return err
	}

	// initialize bluetooth after dbus interface installed
	go _bt.init()
	return nil
}

func (*module) Stop() error {
	if _bt == nil {
		return nil
	}

	sessionmsg.SetAgentInfoPublisher(nil)

	service := loader.GetService()
	err := service.ReleaseName(dbusServiceName)
	if err != nil {
		logger.Warning(err)
	}

	_bt.destroy()
	_bt = nil
	return nil
}
