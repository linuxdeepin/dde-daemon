// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/godbus/dbus/v5"
)

func startBacklightHelperAsync(conn *dbus.Conn) {
	go func() {
		obj := conn.Object("org.deepin.dde.BacklightHelper1", "/org/deepin/dde/BacklightHelper1")
		err := obj.Call("org.freedesktop.DBus.Peer.Ping", 0).Err

		if err != nil {
			logger.Warning(err)
		}
	}()
}
