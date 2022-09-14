// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package loader

import (
	"fmt"
	"sync"

	"github.com/linuxdeepin/go-lib/log"
)

type Module interface {
	Name() string
	IsEnable() bool
	Enable(bool) error
	GetDependencies() []string
	SetLogLevel(log.Priority)
	LogLevel() log.Priority
	WaitEnable() // TODO: should this function return when modules enable failed?
	ModuleImpl
}

type Modules map[string]Module

type ModuleImpl interface {
	Start() error // please keep Start sync, please return err, err log will be done by loader
	Stop() error
}

type ModuleBase struct {
	impl    ModuleImpl
	enabled bool
	name    string
	log     *log.Logger
	wg      sync.WaitGroup
}

func NewModuleBase(name string, impl ModuleImpl, logger *log.Logger) *ModuleBase {
	m := &ModuleBase{
		name: name,
		impl: impl,
		log:  logger,
	}

	// 此为等待「enabled」的 WaitGroup，故在 enable 完成之前，需要一直为等待状态。
	// 其他依赖当前模块的模块启动时，可能还没有调用过当前模块的 Enable，所以不能放在 Enable 中。
	m.wg.Add(1)

	return m
}

func (d *ModuleBase) doEnable(enable bool) error {
	if d.impl != nil {
		fn := d.impl.Stop
		if enable {
			fn = d.impl.Start
		}

		if err := fn(); err != nil {
			return err
		}

		if enable {
			d.wg.Done()
		}
	}
	d.enabled = enable
	return nil
}

func (d *ModuleBase) Enable(enable bool) error {
	if d.enabled == enable {
		return fmt.Errorf("%s daemon is already started", d.name)
	}
	return d.doEnable(enable)
}

func (d *ModuleBase) IsEnable() bool {
	return d.enabled
}

func (d *ModuleBase) WaitEnable() {
	d.wg.Wait()
}

func (d *ModuleBase) Name() string {
	return d.name
}

func (d *ModuleBase) SetLogLevel(pri log.Priority) {
	d.log.SetLogLevel(pri)
}

func (d *ModuleBase) LogLevel() log.Priority {
	return d.log.GetLogLevel()
}
