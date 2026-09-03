// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub2

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	themeDBusPath            = dbusPath + "/Theme"
	themeDBusInterface       = dbusInterface + ".Theme"
	grubBackgroundRuntimeDir = "/run/deepin-grub2"

	// maxBackgroundSourceSize 限制通过 dbus fd 传入的壁纸源文件大小，避免资源耗尽
	maxBackgroundSourceSize = 32 * 1024 * 1024
)

func (*Theme) GetInterfaceName() string {
	return themeDBusInterface
}

func (theme *Theme) SetBackgroundSourceFile(sender dbus.Sender, fd dbus.UnixFD) *dbus.Error {
	err := checkInvokePermission(theme.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}
	theme.service.DelayAutoQuit()

	logger.Debugf("SetBackgroundSourceFile: fd %d", fd)
	err = theme.g.checkAuth(sender, polikitActionIdCommon)
	if err != nil {
		return dbusutil.ToError(err)
	}

	tempPath, err := writeBackgroundSourceToTemp(fd)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	defer os.Remove(tempPath)

	cmd := exec.Command(adjustThemeCmd, "-set-background", tempPath)
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

// writeBackgroundSourceToTemp 将 dbus fd 中的壁纸源文件内容写入运行时目录下的临时文件，
// 并返回临时文件路径。调用方使用完后应删除返回的文件；fd 在读取后被关闭。
func writeBackgroundSourceToTemp(fd dbus.UnixFD) (string, error) {
	if fd < 0 {
		return "", errors.New("invalid background source fd")
	}
	f := os.NewFile(uintptr(fd), "background-source")
	if f == nil {
		return "", errors.New("invalid background source fd")
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("background source is not a regular file")
	}
	if info.Size() > maxBackgroundSourceSize {
		return "", fmt.Errorf("file size %d > %d", info.Size(), maxBackgroundSourceSize)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	tempFile, err := os.CreateTemp(grubBackgroundRuntimeDir, ".background-source-*")
	if err != nil {
		return "", err
	}
	tempPath := tempFile.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	n, err := io.Copy(tempFile, io.LimitReader(f, maxBackgroundSourceSize+1))
	if err != nil {
		_ = tempFile.Close()
		return "", err
	}
	if n > maxBackgroundSourceSize {
		_ = tempFile.Close()
		return "", fmt.Errorf("file size %d > %d", n, maxBackgroundSourceSize)
	}
	if err := tempFile.Close(); err != nil {
		return "", err
	}
	removeTemp = false
	return tempPath, nil
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
