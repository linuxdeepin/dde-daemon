// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	dbus "github.com/godbus/dbus/v5"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

// genGaussianBlur pre-generates a blur image for the greeter background
// by calling the dde-wallpaper-cache ImageBlur1 D-Bus service.
func genGaussianBlur(file string) {
	file = dutils.DecodeURI(file)
	go func() {
		sysBus, err := dbus.SystemBus()
		if err != nil {
			logger.Warning("genGaussianBlur: failed to get system bus:", err)
			return
		}
		obj := sysBus.Object("org.deepin.dde.ImageBlur1", "/org/deepin/dde/ImageBlur1")
		var blurPath string
		err = obj.Call("org.deepin.dde.ImageBlur1.Get", 0, file).Store(&blurPath)
		if err != nil {
			logger.Warning("genGaussianBlur: D-Bus call failed:", err)
		}
	}()
}
