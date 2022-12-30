// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package lastore

import (
	"sync"

	"github.com/godbus/dbus"
	lastore "github.com/linuxdeepin/go-dbus-factory/com.deepin.lastore"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	// lastore agent的interface name
	sessionAgentInterface = "com.deepin.lastore.Agent"
	sessionAgentPath      = "/com/deepin/lastore/agent"
)

// 对应com.deepin.daemon.Network.GetProxy方法的key值
const (
	proxyTypeHttp  = "http"
	proxyTypeHttps = "https"
	proxyTypeFtp   = "ftp"
	proxyTypeSocks = "socks"
)

// 对应系统代理环境变量
const (
	envHttpProxy  = "http_proxy"
	envHttpsProxy = "https_proxy"
	envFtpProxy   = "ftp_proxy"
	envAllProxy   = "all_proxy"
)

// Agent 需要实现GetManualProxy和SendNotify两个接口
type Agent struct {
	sysService *dbusutil.Service
	lastoreObj *Lastore
	sysLastore lastore.Lastore
	mu         sync.Mutex
}

func newAgent(l *Lastore) (*Agent, error) {
	sysBus, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	a := &Agent{
		sysService: sysBus,
	}
	a.lastoreObj = l
	a.sysLastore = lastore.NewLastore(a.sysService.Conn())
	return a, nil
}

func (a *Agent) init() {
	err := a.sysService.Export(sessionAgentPath, a)
	if err != nil {
		logger.Warning(err)
		return
	}
	err = a.sysLastore.Manager().RegisterAgent(0, sessionAgentPath)
	if err != nil {
		logger.Warning(err)
	}
}

func (a *Agent) destroy() {
	err := a.sysLastore.Manager().UnRegisterAgent(0, sessionAgentPath)
	if err != nil {
		logger.Warning(err)
	}
}

func (a *Agent) checkCallerAuth(sender dbus.Sender) bool {
	const rootUid = 0
	uid, err := a.sysService.GetConnUID(string(sender))
	if err != nil {
		return false
	}
	if uid != rootUid {
		logger.Warningf("not allow %v call this method", sender)
		return false
	}
	return true
}
