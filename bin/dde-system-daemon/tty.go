/*
 * Copyright (C) 2020 ~ 2020 Deepin Technology Co., Ltd.
 *
 * Author:     wubowen <wubowen@uniontech.com>
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

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	dbus "github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
)

const clearData = "\033\143\n"

func (*Daemon) ClearTtys() *dbus.Error {
	for _, tty := range getValidTtys() {
		err := clearTtyAux(tty)
		if err != nil {
			return dbusutil.ToError(err)
		}
	}
	return nil
}

func (*Daemon) ClearTty(number uint32) *dbus.Error {
	err := clearTty(number)
	if err != nil {
		logger.Warning(err)
	}
	return dbusutil.ToError(err)
}

func getValidTtys() (res []string) {
	output, err := exec.Command("w", "-h", "-s").Output()
	if err != nil {
		logger.Error(err)
		return nil
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		context := strings.Fields(line)
		if len(context) > 2 && strings.Contains(context[1], "tty") {
			res = append(res, context[1])
		}
	}
	return res
}

func clearTty(number uint32) error {
	file := fmt.Sprintf("tty%d", number)
	//判断是否是有效tty, 无效直接返回"this tty number is not exist"
	ttys := getValidTtys()
	for _, tty := range ttys {
		if file == tty {
			return clearTtyAux(file)
		}
	}
	return errors.New("this tty number is not exist")
}

func clearTtyAux(file string) error {
	f, err := os.OpenFile(filepath.Join("/dev/", file), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		logger.Error("clearTtyAux:", err)
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	_, err = f.WriteString(clearData)
	if err != nil {
		logger.Error("clearTtyAux:", err)
	}
	return err
}
