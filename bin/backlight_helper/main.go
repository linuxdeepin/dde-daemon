/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-daemon/bin/backlight_helper/ddcci"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

//go:generate dbusutil-gen em -type Manager

const (
	dbusServiceName = "com.deepin.daemon.helper.Backlight"
	dbusPath        = "/com/deepin/daemon/helper/Backlight"
	dbusInterface   = "com.deepin.daemon.helper.Backlight"
)

const (
	DisplayBacklight byte = iota + 1
	KeyboardBacklight
)

type Manager struct {
	service *dbusutil.Service
}

var logger = log.NewLogger("backlight_helper")

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) SetBrightness(type0 byte, name string, value int32) *dbus.Error {
	m.service.DelayAutoQuit()
	filename, err := getBrightnessFilename(type0, name)
	if err != nil {
		return dbusutil.ToError(err)
	}

	fh, err := os.OpenFile(filename, os.O_WRONLY, 0666)
	if err != nil {
		return dbusutil.ToError(err)
	}
	defer fh.Close()

	_, err = fh.WriteString(strconv.Itoa(int(value)))
	if err != nil {
		return dbusutil.ToError(err)
	}

	return nil
}

func getBrightnessFilename(type0 byte, name string) (string, error) {
	// check type0
	var subsystem string
	switch type0 {
	case DisplayBacklight:
		subsystem = "backlight"
	case KeyboardBacklight:
		subsystem = "leds"
	default:
		return "", fmt.Errorf("invalid type %d", type0)
	}

	// check name
	if strings.ContainsRune(name, '/') || name == "" ||
		name == "." || name == ".." {
		return "", fmt.Errorf("invalid name %q", name)
	}

	return filepath.Join("/sys/class", subsystem, name, "brightness"), nil
}

func main() {
	service, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Fatal("failed to new system service:", err)
	}

	m := &Manager{
		service: service,
	}
	err = service.Export(dbusPath, m)
	if err != nil {
		logger.Fatal("failed to export:", err)
	}

	ddcciManager, err := ddcci.NewManager(service)
	if err != nil {
		logger.Warning(err)
	} else {
		err = service.Export(ddcci.DbusPath, ddcciManager)
		if err != nil {
			logger.Warning("failed to export:", err)
		}
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		logger.Fatal("failed to request name:", err)
	}

	service.SetAutoQuitHandler(time.Second*30, nil)
	service.Wait()
}
