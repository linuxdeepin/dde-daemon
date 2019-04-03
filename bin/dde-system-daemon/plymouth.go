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
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
)

var plymouthLocker sync.Mutex

func (*Daemon) ScalePlymouth(scale uint32) *dbus.Error {
	plymouthLocker.Lock()
	defer plymouthLocker.Unlock()

	logger.Debug("ScalePlymouth", scale)
	defer logger.Debug("end ScalePlymouth", scale)

	var (
		out []byte
		err error
	)

	// TODO: inhibit poweroff
	switch scale {
	case 1:
		var name = "deepin-logo"
		if isSSD() {
			name = "deepin-ssd-logo"
		}
		out, err = exec.Command("plymouth-set-default-theme", name).CombinedOutput()
	case 2:
		var name = "deepin-hidpi-logo"
		if isSSD() {
			name = "deepin-hidpi-ssd-logo"
		}
		out, err = exec.Command("plymouth-set-default-theme", name).CombinedOutput()
	default:
		return dbusutil.ToError(fmt.Errorf("invalid scale value: %d", scale))
	}

	if err != nil {
		logger.Error("Failed to set plymouth theme:", string(out), err)
		return dbusutil.ToError(err)
	}

	kernel, _ := exec.Command("uname", "-r").CombinedOutput()

	out, err = exec.Command("update-initramfs",
		"-u", "-k", string(kernel[:len(kernel)-1])).CombinedOutput()
	if err != nil {
		logger.Error("Failed to update initramfs:", string(out), err)
		return dbusutil.ToError(err)
	}
	logger.Debug("Plymouth update result:", string(out))
	return nil
}

var _ssd = -1

func isSSD() bool {
	if _ssd != -1 {
		return _ssd == 1
	}

	outputs, err := exec.Command("/bin/lsblk",
		"-P", "-o", "MOUNTPOINT,ROTA").CombinedOutput()
	if err != nil {
		logger.Warning("Failed to check ssd:", string(outputs), err)
		_ssd = -1
		return false
	}

	lines := strings.Split(string(outputs), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "MOUNTPOINT=\"/\"") {
			continue
		}

		// ssd: ROTA="0"
		if strings.Contains(line, "ROTA=\"0\"") {
			_ssd = 1
		} else {
			_ssd = 0
		}
		break
	}
	return _ssd == 1
}
