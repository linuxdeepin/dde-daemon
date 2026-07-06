// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub2

import (
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/utils"
)

const (
	themeDBusPath      = dbusPath + "/Theme"
	themeDBusInterface = dbusInterface + ".Theme"
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

	ext := path.Ext(background)
	backGroundTmpFile, err := os.CreateTemp("/tmp", "dde-grub-background-*"+ext)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	backGroundTmpPath := backGroundTmpFile.Name()
	backGroundTmpFile.Close()

	err = utils.CopyFile(background, backGroundTmpPath)
	if err != nil {
		logger.Warningf("GetBackground: copy %q to %q failed: %v", background, backGroundTmpPath, err)
		_ = os.Remove(backGroundTmpPath)
		return "", dbusutil.ToError(err)
	}

	err = os.Chmod(backGroundTmpPath, 0644)
	if err != nil {
		logger.Warningf("GetBackground: chmod %q 0644 failed: %v", backGroundTmpPath, err)
		_ = os.Remove(backGroundTmpPath)
		return "", dbusutil.ToError(err)
	}
	logger.Debugf("Copy file %s to %s", background, backGroundTmpPath)

	// 已知问题：返回的临时文件由调用方使用，本服务不清理。每次调用产生一个
	// 唯一命名的新文件，重复调用会在 /tmp 中累积（依赖重启清空 tmpfs）。
	return backGroundTmpPath, nil
}

func (theme *Theme) emitSignalBackgroundChanged() {
	err := theme.service.Emit(theme, "BackgroundChanged")
	if err != nil {
		logger.Warning(err)
	}
}
