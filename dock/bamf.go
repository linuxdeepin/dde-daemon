/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package dock

import (
	"github.com/godbus/dbus"
	bamf "github.com/linuxdeepin/go-dbus-factory/session/org.ayatana.bamf"
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
