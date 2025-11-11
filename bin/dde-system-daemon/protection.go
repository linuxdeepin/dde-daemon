// SPDX-FileCopyrightText: 2018 - 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/common/systemdunit"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	EnableROProtection  = "org.deepin.dde.daemon.enable-readonly-protection"
	DisableROProtection = "org.deepin.dde.daemon.disable-readonly-protection"
)

func (d *Daemon) SetReadOnlyProtection(sender dbus.Sender, enable bool) *dbus.Error {
	// 检查 polkit 权限
	var id string
	if enable {
		id = EnableROProtection
	} else {
		id = DisableROProtection
	}

	err := checkAuth(id, string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}

	sysBus, err := dbus.SystemBus()
	if err != nil {
		return dbusutil.ToError(err)
	}

	var writable string
	if enable {
		writable = "disable"
	} else {
		writable = "enable"
	}

	// transient unit 必须是唯一的，不能与已加载的 unit 同名
	unitName := "dde-system-ro-protect.service"

	unitInfo := systemdunit.TransientUnit{
		Dbus:        sysBus,
		UnitName:    unitName,
		Type:        "oneshot",
		Description: "set read-only protect for system",
		Commands:    []string{"deepin-immutable-writable", writable, "-y"},
	}
	err = unitInfo.StartTransientUnit()
	if err != nil {
		return dbusutil.ToError(err)
	}
	if !unitInfo.WaitforFinish(d.systemSigLoop) {
		return dbusutil.ToError(fmt.Errorf("failed to set system read-only(%v)", enable))
	}
	return nil
}
