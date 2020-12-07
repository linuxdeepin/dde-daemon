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

package loader

import (
	"fmt"
	"sync"
	"time"

	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
)

type EnableFlag int

const (
	EnableFlagNone EnableFlag = 1 << iota
	EnableFlagIgnoreMissingModule
	EnableFlagForceStart
)

func (flags EnableFlag) HasFlag(flag EnableFlag) bool {
	return flags&flag != 0
}

const (
	ErrorNoDependencies int = iota
	ErrorCircleDependencies
	ErrorMissingModule
	ErrorInternalError
	ErrorConflict
)

type EnableError struct {
	ModuleName string
	Code       int
	detail     string
}

func (e *EnableError) Error() string {
	switch e.Code {
	case ErrorNoDependencies:
		return fmt.Sprintf("%s's dependencies is not meet, %s is need", e.ModuleName, e.detail)
	case ErrorCircleDependencies:
		return "dependency circle"
		// return fmt.Sprintf("%s and %s dependency each other.", e.ModuleName, e.detail)
	case ErrorMissingModule:
		return fmt.Sprintf("%s is missing", e.ModuleName)
	case ErrorInternalError:
		return fmt.Sprintf("%s started failed: %s", e.ModuleName, e.detail)
	case ErrorConflict:
		return fmt.Sprintf("tring to enable disabled module(%s)", e.ModuleName)
	}
	panic("EnableError: Unknown Error, Should not be reached")
}

type Loader struct {
	modules Modules
	log     *log.Logger
	lock    sync.Mutex
	service *dbusutil.Service
}

func (l *Loader) SetLogLevel(pri log.Priority) {
	l.log.SetLogLevel(pri)

	l.lock.Lock()
	defer l.lock.Unlock()

	for _, module := range l.modules {
		module.SetLogLevel(pri)
	}
}

func (l *Loader) AddModule(m Module) {
	l.lock.Lock()
	defer l.lock.Unlock()

	tmp := l.modules.Get(m.Name())
	if tmp != nil {
		l.log.Debug("Register", m.Name(), "is already registered")
		return
	}

	l.log.Debug("Register module:", m.Name())
	l.modules = append(l.modules, m)
}

func (l *Loader) DeleteModule(name string) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.modules, _ = l.modules.Delete(name)
}

func (l *Loader) List() []Module {
	var modules Modules

	l.lock.Lock()
	modules = append(modules, l.modules...)
	l.lock.Unlock()

	return modules
}

func (l *Loader) GetModule(name string) Module {
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.modules.Get(name)
}

func (l *Loader) WaitDependencies(module Module) {
	for _, dependencyName := range module.GetDependencies() {
		l.modules.Get(dependencyName).WaitEnable()
	}
}

func (l *Loader) EnableModules(enablingModules []string, disableModules []string, flag EnableFlag) error {
	l.lock.Lock()
	defer l.lock.Unlock()

	// build a dag
	startTime := time.Now()
	builder := NewDAGBuilder(l, enablingModules, disableModules, flag)
	dag, err := builder.Execute()
	if err != nil {
		return err
	}
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	l.log.Infof("build dag done, cost %s", duration)

	// perform a topo sort
	nodes, ok := dag.TopologicalDag()
	if !ok {
		return &EnableError{Code: ErrorCircleDependencies}
	}
	endTime = time.Now()
	duration = endTime.Sub(startTime)
	l.log.Infof("topo sort done, cost add up to %s", duration)

	// enable modules
	for _, node := range nodes {
		if node == nil {
			continue
		}
		module := l.modules.Get(node.ID)
		go func() {
			l.log.Info("enable module", module.Name())
			startTime := time.Now()

			// wait for its dependency
			l.WaitDependencies(module)
			l.log.Info("module", module.Name(), "wait done")
			err := module.Enable(true)
			endTime := time.Now()
			duration := endTime.Sub(startTime)
			if err != nil {
				l.log.Fatalf("enable module %s failed: %s, cost %s", module.Name(), err, duration)
			} else {
				l.log.Infof("enable module %s done cost %s", module.Name(), duration)
			}
		}()
	}

	for _, n := range nodes {
		m := l.modules.Get(n.ID)
		m.WaitEnable()
	}

	endTime = time.Now()
	duration = endTime.Sub(startTime)
	l.log.Infof("enable modules done, cost add up to %s", duration)
	return nil
}
