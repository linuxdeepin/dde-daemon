// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package systeminfo

import (
	"github.com/jouyouyun/hardware/dmi"
	"github.com/linuxdeepin/dde-daemon/loader"
	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("daemon/systeminfo")

func init() {
	loader.Register(NewModule())
}

type Module struct {
	m *Manager
	*loader.ModuleBase
}

func (m *Module) GetDependencies() []string {
	return nil
}

func (m *Module) Start() error {
	if m.m != nil {
		return nil
	}
	logger.Debug("system info module start")
	service := loader.GetService()
	m.m = NewManager(service)
	err := service.Export(dbusPath, m.m)
	if err != nil {
		return err
	}
	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}
	go func() {
		//init get memory
		err = m.m.calculateMemoryViaLshw()
		if err != nil {
			logger.Warning(err)
		}
		//get system bit
		systemType := 64
		if "64" != m.m.systemBit() {
			systemType = 32
		}
		//Get CPU MHZ
		currentSpeed, err1 := GetCurrentSpeed(systemType)
		if err1 != nil {
			logger.Warning(err1)
			return
		}
		m.m.setPropCurrentSpeed(currentSpeed)
		info, err := dmi.GetDMI()
		if err != nil {
			logger.Warning(err)
			info = &dmi.DMI{}
		}
		m.m.setPropDMIInfo(info)

	}()
	return nil
}

func (m *Module) Stop() error {
	return nil
}

func NewModule() *Module {
	m := &Module{}
	m.ModuleBase = loader.NewModuleBase("systeminfo", m, logger)
	return m
}
