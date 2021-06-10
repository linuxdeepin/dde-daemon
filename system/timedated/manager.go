/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package timedated

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	dbus "github.com/godbus/dbus"
	polkit "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.policykit1"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.systemd1"
	timedate1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.timedate1"
	timesync1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.timesync1"

	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/keyfile"
)

//go:generate dbusutil-gen -type Manager manager.go
//go:generate dbusutil-gen em -type Manager

type Manager struct {
	core           timedate1.Timedate
	service        *dbusutil.Service
	PropsMu        sync.RWMutex
	NTPServer      string
	timesyncd      timesync1.Timesync1
	systemd        systemd1.Manager
	setNTPServerMu sync.RWMutex
	signalLoop     *dbusutil.SignalLoop
}

const (
	dbusServiceName = "com.deepin.daemon.Timedated"
	dbusPath        = "/com/deepin/daemon/Timedated"
	dbusInterface   = dbusServiceName

	timedate1ActionId = "org.freedesktop.timedate1.set-time"

	timeSyncCfgFile = "/etc/systemd/timesyncd.conf.d/deepin.conf"

	timesyncdService = "systemd-timesyncd.service"
)

func NewManager(service *dbusutil.Service) (*Manager, error) {
	core := timedate1.NewTimedate(service.Conn())
	m := &Manager{
		core:    core,
		service: service,
	}
	return m, nil
}

func (m *Manager) start() {

	m.signalLoop = dbusutil.NewSignalLoop(m.service.Conn(), 10)
	m.signalLoop.Start()

	m.timesyncd = timesync1.NewTimesync1(m.service.Conn())
	server, err := getNTPServer()
	if err != nil {
		logger.Warning(err)
	}
	m.systemd = systemd1.NewManager(m.service.Conn())
	// 第一次启动时,默认无NTPServer文件.如果时间同步状态是开启的(系统默认开启),将时间同步服务数据同步到timedated中
	if server == "" {
		ntp, err := m.core.NTP().Get(0)
		if err != nil {
			logger.Warning(err)
		}
		if m.isUnitEnable(timesyncdService) && ntp {
			serverName, err := m.timesyncd.ServerName().Get(0)
			if err != nil {
				logger.Warning(err)
			} else {
				server = serverName
			}
		}
	}
	err = m.setNTPServer(server)
	if err != nil {
		logger.Warning(err)
	}
	m.timesyncd.InitSignalExt(m.signalLoop, true)
	err = m.timesyncd.ServerName().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}
		err = m.setNTPServer(server)
		if err != nil {
			logger.Warning(err)
		}
	})
	if err != nil {
		logger.Warning(err)
	}
	m.systemd.InitSignalExt(m.signalLoop, true)

	_, err = m.systemd.ConnectUnitNew(func(id string, unit dbus.ObjectPath) {
		// 监听systemd-timesyncd.service服务的启动,代表了开启了时间同步服务,获取该服务的时间服务器数据,
		// 如果开启NTP后直接读取timesync1的数据,有可能存在服务未启动的情况,该服务无法被dbus-daemon启动.
		if id == timesyncdService {
			if !m.isUnitEnable(timesyncdService) {
				return
			}
			ntp, err := m.core.NTP().Get(0)
			if err != nil {
				logger.Warning(err)
				return
			}
			if !ntp {
				return
			}
			server, err = m.timesyncd.ServerName().Get(dbus.FlagNoAutoStart)
			if err != nil {
				logger.Warning(err)
				return
			}
			if server != "" {
				err = m.setNTPServer(server)
				if err != nil {
					logger.Warning(err)
				}
			}
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) setNTPServer(value string) error {
	m.PropsMu.RLock()
	if m.NTPServer == value {
		m.PropsMu.RUnlock()
		return nil
	}
	m.PropsMu.RUnlock()

	m.setNTPServerMu.Lock()
	defer m.setNTPServerMu.Unlock()
	err := setNTPServer(value)
	if err != nil {
		return err
	}

	m.PropsMu.Lock()
	m.NTPServer = value
	m.PropsMu.Unlock()
	return m.emitPropChangedNTPServer(value)
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) destroy() {
	if m.core == nil {
		return
	}
	m.core = nil
}

func (m *Manager) checkAuthorization(method, msg string, sender dbus.Sender) error {
	isAuthorized, err := doAuthorized(msg, string(sender))
	if err != nil {
		logger.Warning("Has error occurred in doAuthorized:", err)
		return err
	}
	if !isAuthorized {
		logger.Warning("Failed to authorize")
		return fmt.Errorf("[%s] Failed to authorize for %v", method, sender)
	}
	return nil
}

func doAuthorized(msg, sysBusName string) (bool, error) {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return false, err
	}
	authority := polkit.NewAuthority(systemBus)
	subject := polkit.MakeSubject(polkit.SubjectKindSystemBusName)
	subject.SetDetail("name", sysBusName)
	detail := map[string]string{
		"polkit.message": msg,
	}
	ret, err := authority.CheckAuthorization(0, subject, timedate1ActionId,
		detail, polkit.CheckAuthorizationFlagsAllowUserInteraction, "")
	if err != nil {
		return false, err
	}
	return ret.IsAuthorized, nil
}

func setNTPServer(server string) error {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(timeSyncCfgFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	kf.SetString("Time", "NTP", server)

	dir := filepath.Dir(timeSyncCfgFile)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	err = kf.SaveToFile(timeSyncCfgFile)
	return err
}

func getNTPServer() (string, error) {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(timeSyncCfgFile)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	server, _ := kf.GetString("Time", "NTP")
	return server, nil
}

func (m *Manager) isUnitEnable(unit string) bool {
	state, err := m.systemd.GetUnitFileState(0, unit)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return "enabled" == strings.TrimSpace(state)
}
