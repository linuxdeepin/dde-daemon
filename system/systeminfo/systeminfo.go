// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package systeminfo

import (
	"strings"

	"github.com/jouyouyun/hardware/dmi"
	"github.com/linuxdeepin/go-lib/log"

	"github.com/linuxdeepin/dde-daemon/loader"
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
		// init get memory
		err = m.m.calculateMemoryViaLshw()
		if err != nil {
			logger.Warning(err)
		}
		// get system bit
		systemType := 64
		if "64" != m.m.systemBit() {
			systemType = 32
		}
		// Get CPU MHZ
		currentSpeed, err1 := GetCurrentSpeed(systemType)
		if err1 != nil {
			logger.Warning(err1)
			return
		}
		m.m.setPropCurrentSpeed(currentSpeed)
		info, err := dmi.GetDMI()
		if err != nil {
			logger.Warning(err)
		} else {
			if info != nil {
				m.m.setPropDMIInfo(*info)
			}
		}
		// get system display driver info
		classContent, err := getLshwData("display")
		if err != nil {
			logger.Warning(err)
		} else {
			var res []string
			for _, item := range classContent {
				res = append(res, item.Configuration.Driver)
			}
			m.m.setPropDisplayDriver(strings.Join(res, "&&"))
		}
		// get system video driver info
		classContent, err = getLshwData("video")
		if err != nil {
			logger.Warning(err)
		} else {
			var res []string
			for _, item := range classContent {
				res = append(res, item.Configuration.Driver)
			}
			m.m.setPropVideoDriver(strings.Join(res, "&&"))
		}
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
