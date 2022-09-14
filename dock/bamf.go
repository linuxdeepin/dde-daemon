// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"github.com/godbus/dbus"
	bamf "github.com/linuxdeepin/go-dbus-factory/org.ayatana.bamf"
	x "github.com/linuxdeepin/go-x11-client"
)

func getDesktopFromWindowByBamf(win x.Window) (string, error) {
	bus, err := dbus.SessionBus()
	if err != nil {
		return "", err
	}
	matcher := bamf.NewMatcher(bus)
	applicationObjPathStr, err := matcher.ApplicationForXid(0, uint32(win))
	if err != nil {
		return "", err
	}
	applicationObjPath := dbus.ObjectPath(applicationObjPathStr)
	if !applicationObjPath.IsValid() {
		return "", nil
	}
	application, err := bamf.NewApplication(bus, applicationObjPath)
	if err != nil {
		return "", err
	}
	desktopFile, err := application.Application().DesktopFile(0)
	if err != nil {
		return "", err
	}
	return desktopFile, nil
}
