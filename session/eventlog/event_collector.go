// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package eventlog

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/godbus/dbus"
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
)

type EventLog struct {
	service *dbusutil.Service

	writeEventLogFn writeEventLogFunc
	Enabled         bool
	propMu          sync.Mutex
	fileMu          sync.Mutex
}

func newEventLog(service *dbusutil.Service, fn writeEventLogFunc) *EventLog {
	m := &EventLog{
		service:         service,
		writeEventLogFn: fn,
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
	if !dutils.IsFileExist(userExpPath) {
		logger.Debugf("%s not exist,should get state in %s", userExpPath, defaultExpPath)
		state = e.getUserExpStateFromDefaultPath()
		// 如果从安装器文件获取到状态，则将数据同步回user
		err := e.setUserExpFileState(state)
		if err != nil {
			logger.Warning(err)
		}
	} else {
		state = e.getUserExpStateFromUserExpPath()
	}
	e.setPropEnabled(state)
}

func (e *EventLog) getUserExpStateFromUserExpPath() bool {
	e.fileMu.Lock()
	defer e.fileMu.Unlock()
	content, err := ioutil.ReadFile(userExpPath)
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
	if !dutils.IsFileExist(userExpPath) {
		err := os.MkdirAll(filepath.Dir(userExpPath), 0777)
		if err != nil {
			return err
		}
	}
	sys := &SysCfg{
		UserExp: state,
	}
	content, err := proto.Marshal(sys)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(userExpPath, content, 0666)
}
