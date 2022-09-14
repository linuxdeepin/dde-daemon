// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package image_effect

import (
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/dde-daemon/loader"
)

func init() {
	loader.Register(newModule())
}

type Module struct {
	*loader.ModuleBase
	ie *ImageEffect
}

func (m *Module) GetDependencies() []string {
	return nil
}

func (m *Module) Start() error {
	if m.ie != nil {
		return nil
	}

	var err error
	m.ie, err = start()
	if err != nil {
		return err
	}

	return nil
}

func (m *Module) Stop() error {
	// TODO
	return nil
}

const moduleName = "image_effect"

var logger = log.NewLogger("daemon/" + moduleName)

func newModule() *Module {
	m := &Module{}
	m.ModuleBase = loader.NewModuleBase(moduleName, m, logger)
	return m
}

func start() (*ImageEffect, error) {
	logger.Debug("module image_effect start")
	ie := newImageEffect()
	service := loader.GetService()
	ie.service = service
	err := service.Export(dbusPath, ie)
	if err != nil {
		return nil, err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return nil, err
	}

	return ie, nil
}
