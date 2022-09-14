// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package bluetooth

import (
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
	btcommon "github.com/linuxdeepin/dde-daemon/common/bluetooth"
	"github.com/linuxdeepin/dde-daemon/loader"
)

type daemon struct {
	*loader.ModuleBase
}

func newBluetoothDaemon(logger *log.Logger) *daemon {
	var d = new(daemon)
	d.ModuleBase = loader.NewModuleBase("bluetooth", d, logger)
	return d
}

func (*daemon) GetDependencies() []string {
	return []string{"audio"}
}

var globalBluetooth *Bluetooth
var globalAgent *agent

func (d *daemon) Start() error {
	if globalBluetooth != nil {
		return nil
	}

	service := loader.GetService()
	globalBluetooth = newBluetooth(service)

	err := service.Export(dbusPath, globalBluetooth)
	if err != nil {
		logger.Warning("failed to export bluetooth:", err)
		globalBluetooth = nil
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	sysService, err := dbusutil.NewSystemService()
	if err != nil {
		return err
	}

	globalAgent = newAgent(sysService)
	globalAgent.b = globalBluetooth
	globalBluetooth.agent = globalAgent

	err = sysService.Export(btcommon.SessionAgentPath, globalAgent)
	if err != nil {
		logger.Warning("failed to export agent:", err)
		return err
	}

	obexAgent := newObexAgent(service, globalBluetooth)
	err = service.Export(obexAgentDBusPath, obexAgent)
	if err != nil {
		logger.Warning("failed to export obex agent:", err)
		return err
	}
	globalBluetooth.obexAgent = obexAgent

	err = initNotifications()
	if err != nil {
		return err
	}
	// initialize bluetooth after dbus interface installed
	go globalBluetooth.init()
	return nil
}

func (*daemon) Stop() error {
	if globalBluetooth == nil {
		return nil
	}

	service := loader.GetService()
	err := service.ReleaseName(dbusServiceName)
	if err != nil {
		logger.Warning(err)
	}

	globalBluetooth.destroy()
	globalBluetooth = nil
	return nil
}
