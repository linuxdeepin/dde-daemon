// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	PromptQuestion uint32 = iota + 1
	PromptSecret
	ErrorMsg
	TextInfo
	Failure
	Success
)

//go:generate dbusutil-gen em -type Manager

type Manager struct {
	service       *dbusutil.Service
	authLocker    sync.Mutex
	authUserTable map[string]chan string // 'pid+user': 'password'

	signals *struct { //nolint
		UserChanged struct {
			username string
		}
	}
}

const (
	dbusServiceName = "org.deepin.dde.LockService1"
	dbusPath        = "/org/deepin/dde/LockService1"
	dbusInterface   = dbusServiceName
)

var _m *Manager

func init() {
	log.SetFlags(log.Lshortfile)
}

func main() {
	service, err := dbusutil.NewSystemService()
	if err != nil {
		log.Fatal("failed to new system service:", err)
	}

	_m = newManager(service)
	err = service.Export(dbusPath, _m)
	if err != nil {
		log.Fatal("failed to export:", err)
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		log.Fatal("failed to request name:", err)
	}

	service.SetAutoQuitHandler(time.Minute*2, func() bool {
		_m.authLocker.Lock()
		canQuit := len(_m.authUserTable) == 0
		_m.authLocker.Unlock()
		return canQuit
	})
	service.Wait()
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func newManager(service *dbusutil.Service) *Manager {
	var m = Manager{
		authUserTable: make(map[string]chan string),
		service:       service,
	}

	return &m
}

func (m *Manager) CurrentUser() (username string, busErr *dbus.Error) {
	username, err := getGreeterUser(greeterUserConfig)
	if err != nil {
		return "", dbusutil.ToError(errors.New("get greeter user failed"))
	}
	return username, nil
}

func (m *Manager) SwitchToUser(username string) *dbus.Error {
	current, _ := getGreeterUser(greeterUserConfig)
	if current == username {
		return nil
	}

	err := setGreeterUser(greeterUserConfig, username)
	if err != nil {
		return dbusutil.ToError(fmt.Errorf("set greeter user failed: %v", err))
	}
	if current != "" {
		err = m.service.Emit(m, "UserChanged", username)
		if err != nil {
			return dbusutil.ToError(err)
		}
	}
	return nil
}
