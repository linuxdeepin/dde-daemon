// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub2

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	editAuthDBusPath      = dbusPath + "/EditAuthentication"
	editAuthDBusInterface = dbusInterface + ".EditAuthentication"
)

// pbkdf2HashReg 用于校验 grub 编辑认证密码的 PBKDF2 哈希格式：
// grub.pbkdf2.sha<alg>.<iterations>.<salt>.<hash>
var pbkdf2HashReg = regexp.MustCompile(`^grub\.pbkdf2\.sha\d+\.\d+\.[0-9A-Fa-f]+\.[0-9A-Fa-f]+$`)

func (e *EditAuth) GetInterfaceName() string {
	return editAuthDBusInterface
}

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

	if !pbkdf2HashReg.MatchString(password) {
		return dbusutil.ToError(fmt.Errorf("invalid pbkdf2 hash format"))
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
