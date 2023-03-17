// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/go-lib/strv"
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

	m.systemSigLoop.AddHandler(&dbusutil.SignalRule{
		Name: "org.desktopspec.ConfigManager.Manager.valueChanged",
	}, func(sig *dbus.Signal) {
		if strings.Contains(sig.Name, "org.desktopspec.ConfigManager.Manager.valueChanged") && len(sig.Body) >= 1 {
			key, ok := sig.Body[0].(string)
			if ok && key == "supportCpuGovernors" {
				// 当supportCpuGovernors被人为改动后，就和availableArrGovernors比较，如果不同则直接设置成availableArrGovernors数据
				cpuGovernors := interfaceToArrayString(m.getDsgData("supportCpuGovernors"))
				availableArrGovernors := getLocalAvailableGovernors()
				if len(cpuGovernors) != len(availableArrGovernors) {
					m.setDsgData("supportCpuGovernors", availableArrGovernors)
					logger.Info("Modification availableGovernors not allowed.")
				} else {
					for _, v := range cpuGovernors {
						if !strv.Strv(availableArrGovernors).Contains(v.(string)) {
							m.setDsgData("supportCpuGovernors", availableArrGovernors)
							logger.Info("Modification availableGovernors not allowed.")
							return
						}
					}
				}
			}
		}
	})
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
	err = systemConnObj.Call("org.desktopspec.ConfigManager.Manager.value", 0, key).Store(&value)
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
	err = systemConnObj.Call("org.desktopspec.ConfigManager.Manager.setValue", 0, key, dbus.MakeVariant(value)).Store()
	if err != nil {
		logger.Warningf("setDsgData key : %s. err : %s", key, err)
		return false
	}
	logger.Infof("setDsgData key : %s , value : %s", key, value)

	return true
}
