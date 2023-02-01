// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
	ConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

//go:generate dbusutil-gen em -type Manager

const (
	dbusServiceName = "org.deepin.dde.BacklightHelper1"
	dbusPath        = "/org/deepin/dde/BacklightHelper1"
	dbusInterface   = "org.deepin.dde.BacklightHelper1"
)

const (
	DisplayBacklight byte = iota + 1
	KeyboardBacklight
)

type Manager struct {
	service *dbusutil.Service

	// 亮度调节方式，策略组配置
	gsSupportDdcci bool
	// helper是否常驻，策略组配置
	gsBackendHold bool

	configManagerPath dbus.ObjectPath
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

func (m *Manager) getBacklightGs(name string) bool {
	systemConnObj := m.service.Conn().Object("org.desktopspec.ConfigManager", m.configManagerPath)
	var value bool
	err := systemConnObj.Call("org.desktopspec.ConfigManager.Manager.value", 0, name).Store(&value)
	if err != nil {
		logger.Warning(err)
		return false
	}
	return value
}

// 检查配置是否支持亮度调节方式, 后续如果有其他的方式可以继续添加补充
func (m *Manager) CheckCfgSupport(name string) (bool, *dbus.Error) {
	switch name {
	case "ddcci":
		if m.gsSupportDdcci {
			return true, nil
		}
	}
	return false, nil
}

func main() {
	m := &Manager{}
	service, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Fatal("failed to new system service:", err)
	}
	m.service = service

	err = service.Export(dbusPath, m)
	if err != nil {
		logger.Fatal("failed to export:", err)
	}
	err = service.RequestName(dbusServiceName)
	if err != nil {
		logger.Fatal("failed to request name:", err)
	}
	if !m.gsBackendHold {
		service.SetAutoQuitHandler(time.Second*30, nil)
	}
	service.Wait()
}
