// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub2

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/utils"
)

const (
	themeDBusPath            = dbusPath + "/Theme"
	themeDBusInterface       = dbusInterface + ".Theme"
	grubBackgroundRuntimeDir = "/run/deepin-grub2"
)

func (*Theme) GetInterfaceName() string {
	return themeDBusInterface
}

func (theme *Theme) SetBackgroundSourceFile(sender dbus.Sender, filename string) *dbus.Error {
	err := checkInvokePermission(theme.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}
	theme.service.DelayAutoQuit()

	logger.Debugf("SetBackgroundSourceFile: %q", filename)
	err = theme.g.checkAuth(sender, polikitActionIdCommon)
	if err != nil {
		return dbusutil.ToError(err)
	}

	filename = utils.DecodeURI(filename)
	cmd := exec.Command(adjustThemeCmd, "-set-background", filename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	theme.emitSignalBackgroundChanged()
	return nil
}

func (theme *Theme) GetBackground(sender dbus.Sender) (background string, busErr *dbus.Error) {
	// 只读操作，无需鉴权
	theme.service.DelayAutoQuit()

	theme.g.PropsMu.RLock()
	themeFile := theme.g.ThemeFile
	theme.g.PropsMu.RUnlock()

	if strings.Contains(themeFile, "/deepin/") {
		_, err := os.Stat(defaultGrubBackground)
		if err != nil {
			if os.IsNotExist(err) {
				background = fallbackGrubBackground
			} else {
				return "", dbusutil.ToError(err)
			}
		} else {
			background = defaultGrubBackground
		}
	} else if strings.Contains(themeFile, "/deepin-fallback/") {
		background = fallbackGrubBackground
	} else {
		return "", nil
	}

	if len(background) == 0 {
		return "", nil
	}

	// source is selected from the fixed packaged default/fallback theme paths above,
	// so it is not controlled by the D-Bus caller.
	theme.backgroundExportMu.Lock()
	defer theme.backgroundExportMu.Unlock()

	destination, err := exportBackground(background, grubBackgroundRuntimeDir)
	if err != nil {
		logger.Warningf("GetBackground: export %q failed: %v", background, err)
		return "", dbusutil.ToError(err)
	}
	return destination, nil
}

func exportBackground(source, destinationDir string) (string, error) {
	sourceFile, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()

	ext := filepath.Ext(source)
	destination := filepath.Join(destinationDir, "background"+ext)
	tempFile, err := os.CreateTemp(destinationDir, ".background-*"+ext)
	if err != nil {
		return "", err
	}
	tempPath := tempFile.Name()
	removeTemp := true
	tempClosed := false
	defer func() {
		if !tempClosed {
			_ = tempFile.Close()
		}
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	// The selected sources are bounded regular theme images; arbitrary paths or
	// unbounded streams cannot reach this exporter through GetBackground.
	if _, err := io.Copy(tempFile, sourceFile); err != nil {
		return "", err
	}
	if err := tempFile.Chmod(0644); err != nil {
		return "", err
	}
	closeErr := tempFile.Close()
	tempClosed = true
	if closeErr != nil {
		return "", closeErr
	}
	if err := os.Rename(tempPath, destination); err != nil {
		return "", err
	}

	removeTemp = false
	return destination, nil
}

func (theme *Theme) emitSignalBackgroundChanged() {
	err := theme.service.Emit(theme, "BackgroundChanged")
	if err != nil {
		logger.Warning(err)
	}
}
