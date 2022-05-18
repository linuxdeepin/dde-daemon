/*
 * Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:     liaohanqin <liaohanqin@uniontech.com>
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

package grub2

import (
	"fmt"
	"strings"
	"unicode"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	editAuthDBusPathV20      = dbusPathV20 + "/EditAuthentication"
	editAuthDBusInterfaceV20 = dbusInterfaceV20 + ".EditAuthentication"

	editAuthDBusPathV23      = dbusPathV23 + "/EditAuthentication"
	editAuthDBusInterfaceV23 = dbusInterfaceV23 + ".EditAuthentication"
)

func (e *EditAuth) Enable(sender dbus.Sender, username, password string) *dbus.Error {
	e.service.DelayAutoQuit()

	err := e.g.checkAuth(sender, polikitActionIdCommon)
	if err != nil {
		return dbusutil.ToError(err)
	}

	if len(username) == 0 ||
		len(password) == 0 ||
		!e.reg.MatchString(username) ||
		strings.IndexFunc(password, unicode.IsSpace) >= 0 {
		return dbusutil.ToError(fmt.Errorf("username or password invalid"))
	}

	err = e.setGrubEditShellAuth(username, password)
	if err != nil {
		return dbusutil.ToError(err)
	}

	return nil
}

func (e *EditAuth) Disable(sender dbus.Sender, username string) *dbus.Error {
	e.service.DelayAutoQuit()

	err := e.g.checkAuth(sender, polikitActionIdCommon)
	if err != nil {
		return dbusutil.ToError(err)
	}

	err = e.disableGrubEditShellAuth(username)
	if err != nil {
		return dbusutil.ToError(err)
	}

	return nil
}
