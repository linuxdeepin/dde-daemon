// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

const maxCount = 5
const maxSize = 32 * 1024 * 1024
const wallPaperDir = "/usr/share/wallpapers/custom-wallpapers/"

func GetUserDir(username string) (string, error) {
	dir := filepath.Join(wallPaperDir, username)
	dir, err := filepath.Abs(dir)

	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(dir, wallPaperDir) {
		return "", fmt.Errorf("UserName is not in %s", wallPaperDir)
	}

	info, err := os.Stat(dir)

	if err != nil && !os.IsExist(err) {
		os.MkdirAll(dir, 0755)
	}

	if info != nil && !info.IsDir() {
		return "", errors.New("UsernName is not a dir")
	}

	return dir, nil
}

func RemoveOverflowWallPapers(username string, max int) error {
	dir, err := GetUserDir(username)
	if err != nil {
		logger.Warning(err)
		return err
	}

	fileinfos, err := os.ReadDir(dir)
	if err != nil {
		logger.Warning(err)
		return err
	}

	logger.Debugf("is count %d <= %d ?", len(fileinfos), max)
	if len(fileinfos) <= max {
		return nil
	}

	sort.Slice(fileinfos, func(i, j int) bool {
		infoI, err := fileinfos[i].Info()
		if err != nil {
			logger.Warning(err)
			return false
		}
		infoJ, err := fileinfos[j].Info()
		if err != nil {
			logger.Warning(err)
			return false
		}
		return infoI.ModTime().Before(infoJ.ModTime())
	})
	for i := 0; i < len(fileinfos)-max; i++ {
		err = os.Remove(filepath.Join(dir, fileinfos[i].Name()))
		if err != nil {
			logger.Warning(err)
		}
	}
	return nil
}

func DeleteWallPaper(username string, file string) error {
	dir, err := GetUserDir(username)
	if err != nil {
		return err
	}

	path, err := filepath.Abs(file)
	if err != nil {
		return err
	}

	if !filepath.HasPrefix(path, dir) {
		return fmt.Errorf("%s is not in %s", file, dir)
	}

	return os.Remove(path)
}

func (d *Daemon) SaveCustomWallPaper(sender dbus.Sender, username string, file string) (string, *dbus.Error) {
	info, err := os.Stat(file)
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}

	if info.Size() > maxSize {
		err = fmt.Errorf("file size %d > %d", info.Size(), maxSize)
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}

	dir, err := GetUserDir(username)
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}

	if filepath.HasPrefix(file, dir) {
		return file, nil
	}

	uid, err := d.service.GetConnUID(string(sender))
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	user, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	if user.Username != username && uid != 0 {
		err = fmt.Errorf("%s not allowed to set %s wallpaper", user.Username, username)
		return "", dbusutil.ToError(err)
	}
	err = exec.Command("runuser", []string{
		"-u",
		username,
		"--",
		"head",
		"-c",
		"0",
		file,
	}...).Run()
	if err != nil {
		err = fmt.Errorf("permission denied, %s is not allowed to read this file:%s", username, file)
		return "", dbusutil.ToError(err)
	}
	md5sum, _ := dutils.SumFileMd5(file)

	destFile := filepath.Join(dir, md5sum)
	destFile = destFile + filepath.Ext(file)
	src, err := os.Open(file)
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}
	defer src.Close()

	dest, err := os.Create(destFile)
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}
	defer dest.Close()

	_, err = io.Copy(dest, src)
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}

	err = RemoveOverflowWallPapers(username, maxCount)
	if err != nil {
		logger.Warning(err)
	}

	return destFile, dbusutil.ToError(err)
}

func (*Daemon) DeleteCustomWallPaper(username string, file string) *dbus.Error {
	return dbusutil.ToError(DeleteWallPaper(username, file))
}

func (*Daemon) GetCustomWallPapers(username string) ([]string, *dbus.Error) {
	dir, err := GetUserDir(username)
	if err != nil {
		logger.Warning(err)
		return []string{}, dbusutil.ToError(err)
	}

	fileinfos, err := os.ReadDir(dir)
	if err != nil {
		logger.Warning(err)
		return []string{}, dbusutil.ToError(err)
	}

	files := []string{}
	for _, fileinfo := range fileinfos {
		files = append(files, filepath.Join(wallPaperDir, username, fileinfo.Name()))
	}

	return files, nil
}
