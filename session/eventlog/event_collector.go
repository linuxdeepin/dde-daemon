// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package eventlog

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/keyfile"
	dutils "github.com/linuxdeepin/go-lib/utils"
	"google.golang.org/protobuf/proto"
)

type BaseCollector interface {
	Init(service *dbusutil.Service, fn writeEventLogFunc) error
	Collect() error
	Stop() error
}

const (
	dbusServiceName = "com.deepin.daemon.EventLog"
	dbusPath        = "/com/deepin/daemon/EventLog"
	dbusInterface   = dbusServiceName
)

var (
	userExpPath    = "/var/public/deepin-user-experience/user"
	defaultExpPath = "/etc/deepin/deepin-user-experience"
	varTmpExpPath  = "/var/tmp/deepin/deepin-user-experience/state/"
)

const (
	enableUserExp  = "switch=on"
	disableUserExp = "switch=off"
)

type EventLog struct {
	service *dbusutil.Service

	writeEventLogFn          writeEventLogFunc
	Enabled                  bool
	propMu                   sync.Mutex
	fileMu                   sync.Mutex
	currentUserVarTmpExpPath string
}

func newEventLog(service *dbusutil.Service, fn writeEventLogFunc) *EventLog {
	m := &EventLog{
		service:                  service,
		writeEventLogFn:          fn,
		currentUserVarTmpExpPath: path.Join(varTmpExpPath, fmt.Sprintf("%v/info", os.Getuid())),
	}
	return m
}

func (e *EventLog) start() error {
	e.syncUserExpState()
	return nil
}

func (e *EventLog) GetInterfaceName() string {
	return dbusInterface
}

func (e *EventLog) Enable(enable bool) *dbus.Error {
	e.propMu.Lock()
	defer e.propMu.Unlock()
	err := e.setUserExpFileState(enable)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	e.setPropEnabled(enable)
	return nil
}

func (e *EventLog) syncUserExpState() {
	var state bool
	if dutils.IsFileExist(e.currentUserVarTmpExpPath) {
		// 从1054标准路径获取
		state = e.getUserExpStateFromVarTmpExpPath()
	} else if dutils.IsFileExist(userExpPath) {
		// 从历史版本获取
		state = e.getUserExpStateFromUserExpPath()
		err := e.setUserExpFileState(state)
		if err != nil {
			logger.Warning(err)
		}
	} else {
		// 从安装器文件获取
		state = e.getUserExpStateFromDefaultPath()
		err := e.setUserExpFileState(state)
		if err != nil {
			logger.Warning(err)
		}
	}
	e.setPropEnabled(state)
}

func (e *EventLog) getUserExpStateFromUserExpPath() bool {
	e.fileMu.Lock()
	defer e.fileMu.Unlock()
	content, err := os.ReadFile(userExpPath)
	if err != nil {
		logger.Warning(err)
		return false
	}
	if len(content) == 0 {
		// 如果user的内容是空的，则默认为false，并将状态写回user
		e.fileMu.Unlock()
		err = e.setUserExpFileState(false)
		if err != nil {
			logger.Warning(err)
		}
		e.fileMu.Lock()
		return false
	}
	cfg := &SysCfg{}
	err = proto.Unmarshal(content, cfg)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return cfg.UserExp
}

func (e *EventLog) getUserExpStateFromDefaultPath() bool {
	const (
		expSection = "ExperiencePlan"
		expKey     = "ExperienceState"
	)
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(defaultExpPath)
	if err != nil {
		logger.Warning(err)
		return false
	} else {
		state, err := kf.GetBool(expSection, expKey)
		if err != nil {
			logger.Warning(err)
			return false
		}
		return state
	}
}

func (e *EventLog) setUserExpFileState(state bool) error {
	e.fileMu.Lock()
	defer e.fileMu.Unlock()
	if !dutils.IsFileExist(e.currentUserVarTmpExpPath) {
		err := os.MkdirAll(filepath.Dir(e.currentUserVarTmpExpPath), 0777)
		if err != nil {
			return err
		}
	}
	var content []byte
	if state {
		content = []byte(enableUserExp)
	} else {
		content = []byte(disableUserExp)
	}
	return os.WriteFile(e.currentUserVarTmpExpPath, content, 0666)
}

func (e *EventLog) getUserExpStateFromVarTmpExpPath() bool {
	e.fileMu.Lock()
	defer e.fileMu.Unlock()
	content, err := os.ReadFile(e.currentUserVarTmpExpPath)
	if err != nil {
		logger.Warning(err)
		return false
	}
	if len(content) == 0 {
		// 如果info的内容是空的，则默认为false，并将状态写回user
		e.fileMu.Unlock()
		err = e.setUserExpFileState(false)
		if err != nil {
			logger.Warning(err)
		}
		e.fileMu.Lock()
		return false
	}

	return string(content) == enableUserExp
}
