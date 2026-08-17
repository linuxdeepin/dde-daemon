// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os/exec"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/securityloader"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

// TODO: 临时方案，hwe一些机型内核wifi有问题，需要停止wifip2p扫描，待内核修改后去掉
func stopNetworkDisaplay() {
	err := exec.Command("killall", "deepin-network-display-daemon").Run()
	if err != nil {
		logger.Warning("Failed to stop network disaplay")
	}
}

func (d *Daemon) forwardPrepareForSleepSignal(service *dbusutil.Service) error {
	d.loginManager.InitSignalExt(d.systemSigLoop, true)

	_, err := d.loginManager.ConnectPrepareForSleep(func(before bool) {
		logger.Info("login1 PrepareForSleep", before)
		// signal `PrepareForSleep` true -> false
		if before {
			stopNetworkDisaplay()
		}
		err := service.Emit(d, "HandleForSleep", before)
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

func (d *Daemon) SetAllowCaller(sender dbus.Sender, uniqueName string) *dbus.Error {
	return dbusutil.ToError(d.allowCallers.AddCaller(securityloader.DaemonScope, sender, uniqueName))
}
