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
	"time"

	"github.com/godbus/dbus"
)

type DBusAgentConfig struct {
	Name             string
	Dest             string
	Interface        string
	Path             dbus.ObjectPath
	RegisterMethod   string
	RegisterDelaySec int

	AgentInterface string
	AgentPath      dbus.ObjectPath
	AgentMethods   []DBusAgentMethod
}

type DBusAgentMethod struct {
	Name string
	fn   interface{}
}

func newAgent(agentCfg *DBusAgentConfig) (*agent, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	var agent = &agent{
		name: agentCfg.Name,
		cfg:  agentCfg,
	}
	methodTable := agent.getMethodTable()
	logger.Debugf("export agent %v, methodTable: %#v, path: %v, ifc: %v", agent.name, methodTable,
		agentCfg.AgentPath, agentCfg.AgentInterface)
	err = sysBus.ExportMethodTable(methodTable, agentCfg.AgentPath, agentCfg.AgentInterface)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

func (a *agent) getRegisterDelay() time.Duration {
	sec := a.cfg.RegisterDelaySec
	if sec == 0 {
		sec = 3
	}
	return time.Second * time.Duration(sec)
}

func (a *agent) register() error {
	agentCfg := a.cfg
	if agentCfg.Dest == "" {
		// Dest 为空的特指 Dest 是 dde-system-daemon 的 com.deepin.system.Bluetooth 服务
		// 复用了 system/bluetooth 提供的 agent 机制。
		return nil
	}
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	serverObj := sysBus.Object(agentCfg.Dest, agentCfg.Path)
	regMethodName := agentCfg.RegisterMethod
	if regMethodName == "" {
		regMethodName = "RegisterAgent"
	}
	err = serverObj.Call(agentCfg.Interface+"."+regMethodName, 0, agentCfg.AgentPath).Err
	return err
}

type agent struct {
	name string
	cfg  *DBusAgentConfig
}

func (a *agent) getMethodTable() map[string]interface{} {
	methods := a.cfg.AgentMethods
	if len(methods) == 0 {
		return nil
	}
	result := make(map[string]interface{}, len(methods))
	for _, method := range methods {
		if method.Name != "" && method.fn != nil {
			result[method.Name] = method.fn
		}
	}
	return result
}

func (a *agent) GetInterfaceName() string {
	return a.cfg.AgentInterface
}
