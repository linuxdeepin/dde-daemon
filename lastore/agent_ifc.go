// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package lastore

import (
	"errors"
	"fmt"
	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (*Agent) GetInterfaceName() string {
	return sessionAgentInterface
}

func (a *Agent) GetManualProxy(sender dbus.Sender) (map[string]string, *dbus.Error) {
	if !a.checkCallerAuth(sender) {
		return nil, dbusutil.ToError(fmt.Errorf("not allow %v call this method", sender))
	}
	method, err := a.lastoreObj.network.GetProxyMethod(0)
	if err != nil {
		logger.Warning(err)
		return nil, dbusutil.ToError(err)
	}
	if method != "manual" {
		return nil, dbusutil.ToError(errors.New("only support manual proxy"))
	}
	proxyTypeList := []string{
		proxyTypeHttp, proxyTypeHttps, proxyTypeFtp,
	}
	proxyEnvMap := make(map[string]string)
	for _, typ := range proxyTypeList {
		host, port, err := a.lastoreObj.network.GetProxy(0, typ)
		if err != nil {
			logger.Warning(err)
			continue
		}
		user, passwd, enable, err := a.lastoreObj.network.GetProxyAuthentication(0, typ)
		if err != nil {
			logger.Warning(err)
			continue
		}
		if enable {
			proxyEnvMap[fmt.Sprintf("%s_proxy", typ)] = fmt.Sprintf("%s://%s:%s@%s:%s", proxyTypeHttp, user, passwd, host, port)
		} else {
			proxyEnvMap[fmt.Sprintf("%s_proxy", typ)] = fmt.Sprintf("%s://%s:%s", proxyTypeHttp, host, port)
		}

	}
	host, port, err := a.lastoreObj.network.GetProxy(0, proxyTypeSocks)
	if err != nil {
		logger.Warning(err)
		return proxyEnvMap, nil
	}
	user, passwd, enable, err := a.lastoreObj.network.GetProxyAuthentication(0, proxyTypeSocks)
	if err != nil {
		logger.Warning(err)
		return proxyEnvMap, nil
	}
	if enable {
		proxyEnvMap[envAllProxy] = fmt.Sprintf("%s://%s:%s@%s:%s", proxyTypeSocks, user, passwd, host, port)
	} else {
		proxyEnvMap[envAllProxy] = fmt.Sprintf("%s://%s:%s", proxyTypeSocks, host, port)
	}

	return proxyEnvMap, nil
}

func (a *Agent) SendNotify(sender dbus.Sender, appName string, replacesId uint32, appIcon string, summary string, body string, actions []string, hints map[string]dbus.Variant, expireTimeout int32) (uint32, *dbus.Error) {
	if !a.checkCallerAuth(sender) {
		return 0, dbusutil.ToError(fmt.Errorf("not allow %v call this method", sender))
	}
	logger.Info("receive notify from lastore daemon")
	id, err := a.lastoreObj.notifications.Notify(0, appName, replacesId, appIcon, summary, body, actions, hints, expireTimeout)
	return id, dbusutil.ToError(err)
}

func (a *Agent) CloseNotification(sender dbus.Sender, id uint32) *dbus.Error {
	if !a.checkCallerAuth(sender) {
		return dbusutil.ToError(fmt.Errorf("not allow %v call this method", sender))
	}
	logger.Info("receive close notify from lastore daemon")
	return dbusutil.ToError(a.lastoreObj.notifications.CloseNotification(0, id))
}
