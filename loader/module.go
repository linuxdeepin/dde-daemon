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

	"pkg.deepin.io/lib/log"
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
