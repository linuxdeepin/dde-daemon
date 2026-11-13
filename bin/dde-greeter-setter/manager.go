// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"sync"
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/securityloader"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/keyfile"
)

//go:generate dbusutil-gen em -type Manager

const (
	dbusServiceName = "org.deepin.dde.Greeter1"
	dbusPath        = "/org/deepin/dde/Greeter1"
	dbusInterface   = dbusServiceName

	polkitActionUpdateQtTheme = "org.deepin.dde.greeter.update-qt-theme"
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

var greeterThemeMu sync.Mutex

func closeUnixFD(fd dbus.UnixFD) {
	if fd < 0 {
		return
	}
	_ = syscall.Close(int(fd))
}

func (m *Manager) UpdateGreeterQtTheme(sender dbus.Sender, fd dbus.UnixFD) *dbus.Error {
	// The theme file lives in /etc/lightdm and is shared by every user of
	// the machine, so the caller must be authorized before it is touched.
	authorized, err := securityloader.CheckPolkitAuth(m.service.Conn(),
		string(sender), polkitActionUpdateQtTheme)
	if err != nil {
		closeUnixFD(fd)
		logger.Warningf("failed to authorize %v: %v", sender, err)
		return dbusutil.ToError(err)
	}
	if !authorized {
		closeUnixFD(fd)
		logger.Warningf("caller %v is not authorized to update the greeter theme", sender)
		return dbusutil.ToError(errAccessDenied)
	}

	m.service.DelayAutoQuit()

	// The theme file and xsettingsd.conf must be replaced as one logical
	// update. Serialize the complete replacement; otherwise two authorized
	// callers can interleave the fixed .tmp file and leave the two files
	// describing different themes.
	greeterThemeMu.Lock()
	defer greeterThemeMu.Unlock()

	if err := updateGreeterQtTheme(fd); err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	if err := updateXSettingsConfig(); err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	return nil
}

var errAccessDenied = errors.New("access denied")

func updateGreeterQtTheme(fd dbus.UnixFD) error {
	f := os.NewFile(uintptr(fd), "")
	if f == nil {
		return fmt.Errorf("invalid file descriptor %d", fd)
	}
	defer f.Close()

	// 目录创建由 postinst 脚本处理，这里只负责文件写入
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

func updateXSettingsConfig() error {
	return updateXSettingsConfigFiles(themeFile, xsettingsFile)
}

func updateXSettingsConfigFiles(themePath, xsettingsPath string) error {
	kf := keyfile.NewKeyFile()
	if err := kf.LoadFromFile(themePath); err != nil {
		return fmt.Errorf("load greeter theme %q: %w", themePath, err)
	}

	// 目录创建由 postinst 脚本处理，这里只负责文件写入
	xf, err := os.OpenFile(xsettingsPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open xsettings file %q: %w", xsettingsPath, err)
	}
	defer xf.Close()

	// 将theme的配置转换成xsettings的配置
	for themeKey, xsettingsKey := range globalKeyConvertMap {
		value, err := kf.GetString(themeSection, themeKey)
		if err == nil {
			if isInteger(value) {
				if _, err = xf.WriteString(fmt.Sprintf("%s %s\n", xsettingsKey, value)); err != nil {
					return fmt.Errorf("write xsettings key %s: %w", xsettingsKey, err)
				}
			} else if quoteXSettingsValue(value) == "" && value != "" {
				// The value comes from a file the caller controls. A quote or
				// newline inside it would otherwise terminate the string and
				// inject extra xsettingsd keys, so such values are dropped.
				logger.Warningf("skip invalid xsettings value for %s", xsettingsKey)
			} else {
				if _, err = xf.WriteString(fmt.Sprintf("%s %s\n", xsettingsKey, quoteXSettingsValue(value))); err != nil {
					return fmt.Errorf("write xsettings key %s: %w", xsettingsKey, err)
				}
			}
		} else {
			logger.Debugf("greeter theme key %s is absent: %v", themeKey, err)
		}
	}

	// 换算缩放 xsettingsKeyDpi = themeKeyScreenScaleFactors * themeKeyScaleLogicalDpi * 1024
	themeDpiStr, err := kf.GetString(themeSection, themeKeyScaleLogicalDpi)
	if err != nil {
		return fmt.Errorf("read greeter DPI: %w", err)
	}
	exp := regexp.MustCompile(`^([0-9]+),([0-9]+)$`)
	substr := exp.FindStringSubmatch(themeDpiStr)
	if len(substr) < 2 {
		return fmt.Errorf("parse greeter DPI %q", themeDpiStr)
	}
	themeDpi, err := strconv.ParseUint(substr[1], 10, 32)
	if err != nil {
		return fmt.Errorf("parse greeter DPI value %q: %w", substr[1], err)
	}
	scale, err := kf.GetFloat64(themeSection, themeKeyScreenScaleFactors)
	if err != nil {
		return fmt.Errorf("read greeter scale factors: %w", err)
	}
	xDpi := uint64(scale * float64(themeDpi) * 1024)
	if _, err = xf.WriteString(fmt.Sprintf("%s %d\n", xsettingsKeyDpi, xDpi)); err != nil {
		return fmt.Errorf("write xsettings DPI: %w", err)
	}
	return nil
}

// quoteXSettingsValue renders value as an xsettingsd string. Values that
// cannot be rendered safely (quotes, backslashes, control characters) yield
// an empty string and must be skipped by the caller.
func quoteXSettingsValue(value string) string {
	for _, r := range value {
		if r == '"' || r == '\\' || r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return `"` + value + `"`
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func isInteger(str string) bool {
	_, err := strconv.ParseInt(str, 10, 32)
	return err == nil
}
