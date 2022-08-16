/*
 * Copyright (C) 2022 ~ 2022 Deepin Technology Co., Ltd.
 *
 * Author:     wubowen <wubowen@uniontech.com>
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

package power

import (
	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/utils"
)

func interfaceToArrayString(v interface{}) (d []interface{}) {
	if utils.IsInterfaceNil(v) {
		return
	}

	d, ok := v.([]interface{})
	if !ok {
		logger.Errorf("interfaceToArrayString() failed: %#v", v)
		return
	}
	return
}

func interfaceToString(v interface{}) (d string) {
	if utils.IsInterfaceNil(v) {
		return
	}
	d, ok := v.(string)
	if !ok {
		logger.Errorf("interfaceToString() failed: %#v", v)
		return
	}
	return
}

func interfaceToBool(v interface{}) (d bool) {
	if utils.IsInterfaceNil(v) {
		return
	}
	d, ok := v.(bool)
	if !ok {
		logger.Errorf("interfaceToBool() failed: %#v", v)
		return
	}
	return
}

func (m *Manager) initDsgConfig(conn *dbus.Conn) error {
	// 加载dsg配置
	systemConnObj := conn.Object(configManagerId, "/")
	err := systemConnObj.Call(configManagerId+".acquireManager", 0, "org.deepin.dde.daemon", "org.deepin.dde.daemon.power", "").Store(&m.configManagerPath)
	if err != nil {
		logger.Warning(err)
		return err
	}
	err = dbusutil.NewMatchRuleBuilder().Type("signal").
		PathNamespace(string(m.configManagerPath)).
		Interface("org.desktopspec.ConfigManager.Manager").
		Member("valueChanged").Build().AddTo(conn)
	if err != nil {
		logger.Warning(err)
		return err
	}
	return nil
}

func (m *Manager) getDsgData(key string) interface{} {
	systemConn, err := dbus.SystemBus()
	if err != nil {
		logger.Warning("getDsgData systemConn err: ", err)
		return nil
	}
	systemConnObj := systemConn.Object("org.desktopspec.ConfigManager", m.configManagerPath)
	var value interface{}
	err = systemConnObj.Call("org.desktopspec.ConfigManager.Manager.value",0, key).Store(&value)
	if err != nil {
		logger.Warningf("getDsgData key : %s. err : %s", key, err)
		return nil
	}
	logger.Info(" getDsgData key : ", key, " , value : ", value)
	return value
}

func (m *Manager) setDsgData(key string, value interface{}) bool {
	systemConn, err := dbus.SystemBus()
	if err != nil {
		logger.Warning("setDsgData systemConn err: ", err)
		return false
	}
	systemConnObj := systemConn.Object("org.desktopspec.ConfigManager", m.configManagerPath)
	err = systemConnObj.Call("org.desktopspec.ConfigManager.Manager.setValue",0, key, dbus.MakeVariant(value)).Store()
	if err != nil {
		logger.Warningf("setDsgData key : %s. err : %s", key, err)
		return false
	}
	logger.Infof("setDsgData key : %s , value : %s", key, value)

	return true
}