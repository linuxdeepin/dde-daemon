// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/godbus/dbus/v5"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (d *Daemon) forwardPrepareForSleepSignal(service *dbusutil.Service) error {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	d.loginManager = login1.NewManager(systemBus)
	d.systemSigLoop = dbusutil.NewSignalLoop(systemBus, 10)
	d.systemSigLoop.Start()
	d.loginManager.InitSignalExt(d.systemSigLoop, true)

	_, err = d.loginManager.ConnectPrepareForSleep(func(before bool) {
		logger.Info("login1 PrepareForSleep", before)
		// signal `PrepareForSleep` true -> false
		err = service.Emit(d, "HandleForSleep", before)
		if err != nil {
			logger.Warning("failed to emit HandleForSleep signal")
			return
		}
	})
	if err != nil {
		logger.Warning("failed to ConnectPrepareForSleep")
		return err
	}
	return nil
}
