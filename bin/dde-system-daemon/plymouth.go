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

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"sync"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

var plymouthLocker sync.Mutex

func (d *Daemon) ScalePlymouth(scale uint32) *dbus.Error {
	return dbusutil.ToError(d.scalePlymouth(scale))
}

func (d *Daemon) scalePlymouth(scale uint32) error {
	plymouthLocker.Lock()
	defer plymouthLocker.Unlock()
	defer logger.Debug("end ScalePlymouth", scale)

	edition, err := getEditionName()
	if err != nil {
		return err
	}
	var themeNames map[uint32]string
	if edition == "Community" {
		themeNames = map[uint32]string{
			1: "deepin-ssd-logo",
			2: "deepin-hidpi-ssd-logo",
		}
	} else {
		themeNames = map[uint32]string{
			1: "uos-ssd-logo",
			2: "uos-hidpi-ssd-logo",
		}
	}
	name, ok := themeNames[scale]
	if !ok {
		return fmt.Errorf("invalid scale value: %d", scale)
	}
	out, err := exec.Command("plymouth-set-default-theme", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set plymouth theme: %s, err: %v", string(out), err)
	}

	kernel, err := exec.Command("uname", "-r").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get kernel, err: %v", err)
	}
	out, err = exec.Command("update-initramfs",
		"-u", "-k", string(bytes.TrimSpace(kernel))).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update initramfs: %s, err: %v", string(out), err)
	}

	return nil
}

func getEditionName() (string, error) {
	conf, err := parseInfoFile("/etc/os-version", "=")
	if err != nil {
		return "", err
	}
	value, ok := conf["EditionName"]
	if !ok {
		return "", fmt.Errorf("Can not find the EditionName")
	}
	return value, nil
}

func parseInfoFile(file, delim string) (map[string]string, error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var ret = make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		array := strings.Split(line, delim)
		if len(array) != 2 {
			continue
		}
		ret[strings.TrimSpace(array[0])] = strings.TrimSpace(array[1])
	}
	return ret, nil
}
