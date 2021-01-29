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
	"io"
	"os"
	"regexp"
	"strconv"

	"github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/keyfile"
)

//go:generate dbusutil-gen em -type Manager

const (
	dbusServiceName = "com.deepin.daemon.Greeter"
	dbusPath        = "/com/deepin/daemon/Greeter"
	dbusInterface   = dbusServiceName
)

const (
	themeFile                  = "/etc/lightdm/deepin/qt-theme.ini"
	themeSection               = "Theme"
	themeKeyIconThemeName      = "IconThemeName"
	themeKeyFontSize           = "FontSize"
	themeKeyFont               = "Font"
	themeKeyMonoFont           = "MonoFont"
	themeKeyScreenScaleFactors = "ScreenScaleFactors"
	themeKeyScaleLogicalDpi    = "ScaleLogicalDpi"
)

const (
	xsettingsFile             = "/etc/lightdm/deepin/xsettingsd.conf"
	xsettingsKeyIconThemeName = "Net/IconThemeName"
	xsettingsKeyFontSize      = "Qt/FontPointSize"
	xsettingsKeyFont          = "Qt/FontName"
	xsettingsKeyMonoFont      = "Qt/MonoFontName"
	xsettingsKeyDpi           = "Xft/DPI"
)

// theme的key
var globalKeyConvertMap = map[string]string{
	themeKeyIconThemeName: xsettingsKeyIconThemeName,
	themeKeyFontSize:      xsettingsKeyFontSize,
	themeKeyFont:          xsettingsKeyFont,
	themeKeyMonoFont:      xsettingsKeyMonoFont,
}

type Manager struct {
	service *dbusutil.Service
}

func (m *Manager) UpdateGreeterQtTheme(fd dbus.UnixFD) *dbus.Error {
	m.service.DelayAutoQuit()
	err := updateGreeterQtTheme(fd)
	if err != nil {
		logger.Warning(err)
	}
	updateXSettingsConfig()
	return dbusutil.ToError(err)
}

func updateGreeterQtTheme(fd dbus.UnixFD) error {
	f := os.NewFile(uintptr(fd), "")
	defer f.Close()
	err := os.MkdirAll("/etc/lightdm/deepin", 0755)
	if err != nil {
		return err
	}
	const themeFileTemp = themeFile + ".tmp"
	dest, err := os.Create(themeFileTemp)
	if err != nil {
		return err
	}
	// limit file size: 100KB
	src := io.LimitReader(f, 1024*100)
	_, err = io.Copy(dest, src)
	if err != nil {
		closeErr := dest.Close()
		if closeErr != nil {
			logger.Warning(closeErr)
		}
		return err
	}

	err = dest.Close()
	if err != nil {
		return err
	}

	err = os.Rename(themeFileTemp, themeFile)
	return err
}

func updateXSettingsConfig() {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(themeFile)
	if err != nil {
		logger.Warning(err)
		return
	}

	xf, err := os.OpenFile(xsettingsFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		logger.Warning(err)
		return
	}
	defer xf.Close()

	// 将theme的配置转换成xsettings的配置
	for themeKey, xsettingsKey := range globalKeyConvertMap {
		value, err := kf.GetString(themeSection, themeKey)
		if err == nil {
			if isInteger(value) {
				_, err = xf.WriteString(fmt.Sprintf("%s %s\n", xsettingsKey, value))
			} else {
				_, err = xf.WriteString(fmt.Sprintf("%s \"%s\"\n", xsettingsKey, value))
			}
		}

		if err != nil {
			logger.Warning(err)
		}
	}

	// 换算缩放 xsettingsKeyDpi = themeKeyScreenScaleFactors * themeKeyScaleLogicalDpi * 1024
	themeDpiStr, err := kf.GetString(themeSection, themeKeyScaleLogicalDpi)
	if err != nil {
		logger.Warningf("cannot get DPI: %v", err)
		return
	}
	exp, err := regexp.Compile(`^([0-9]+),([0-9]+)$`)
	if err != nil {
		logger.Warning(err)
		return
	}
	substr := exp.FindStringSubmatch(themeDpiStr)
	if len(substr) < 2 {
		logger.Warning("cannot parse theme dpi")
		return
	}
	themeDpi, err := strconv.ParseUint(substr[1], 10, 32)
	if err != nil {
		logger.Warning(err)
		return
	}
	scale, err := kf.GetFloat64(themeSection, themeKeyScreenScaleFactors)
	xDpi := uint64(scale * float64(themeDpi) * 1024)
	_, err = xf.WriteString(fmt.Sprintf("%s %d\n", xsettingsKeyDpi, xDpi))
	if err != nil {
		logger.Warning(err)
	}
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func isInteger(str string) bool {
	_, err := strconv.ParseInt(str, 10, 32)
	return err == nil
}
