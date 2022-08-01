/*
 * Copyright (C) 2013 ~ 2018 Deepin Technology Co., Ltd.
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

package users

import (
	"fmt"
	"os/exec"
	"strconv"

	"github.com/linuxdeepin/go-lib/keyfile"
)

func isStrInArray(str string, array []string) bool {
	for _, v := range array {
		if v == str {
			return true
		}
	}

	return false
}

func doAction(cmd string, args []string) error {
	fmt.Println("run cmd", cmd)
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		fmt.Printf("[doAction] exec '%s' failed: %s, %v\n", cmd, string(out), err)
	}
	return err
}

func strToInt(str string, defaultVal int) int {
	if str == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(str)
	if err != nil {
		val = defaultVal
	}
	return val
}

func systemType() string {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile("/etc/deepin-version")
	if err != nil {
		fmt.Println("load version file failed, err: ", err)
		return ""
	}
	typ, err := kf.GetString("Release", "Type")
	if err != nil {
		fmt.Println("get version type failed, err: ", err)
		return ""
	}
	return typ
}
