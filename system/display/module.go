// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display

import (
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/dde-daemon/loader"
)

type module struct {
	*loader.ModuleBase
}

func (m *module) GetDependencies() []string {
	return nil
}

func (m *module) Start() error {
	logger.Debug("module display start")
	service := loader.GetService()

	d := newDisplay(service)
	err := service.Export(dbusPath, d)
	if err != nil {
		return err
	}
	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	return nil
}

func (m *module) Stop() error {
	return nil
}

func newDisplayModule(logger *log.Logger) *module {
	m := new(module)
	m.ModuleBase = loader.NewModuleBase("display", m, logger)
	return m
}

var logger = log.NewLogger("daemon/display")

func init() {
	loader.Register(newDisplayModule(logger))
}
