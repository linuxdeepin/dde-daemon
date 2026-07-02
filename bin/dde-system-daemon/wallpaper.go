// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	_ "image/jpeg"
	_ "image/png"

	dbus "github.com/godbus/dbus/v5"
	polkit "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.policykit1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	dutils "github.com/linuxdeepin/go-lib/utils"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

const maxCount = 20
const maxWallpaperSize = 32 * 1024 * 1024
const wallPaperDir = "/var/cache/wallpapers/custom-wallpapers/"
const solidWallPaperPath = "/var/cache/wallpapers/custom-solidwallpapers/"
const polkitActionUserAdministration = "org.deepin.dde.accounts.user-administration"

var wallPaperDirs = []string{
	wallPaperDir,
	solidWallPaperPath,
}

const (
	wallpaperDel = iota
	wallpaperAdd
)

func checkPath(path string, dirs []string) string {
	for _, dir := range dirs {
		// 确保目录路径以分隔符结尾，防止 "user1" 匹配 "user10"
		prefix := dir + string(filepath.Separator)
		if path == dir || strings.HasPrefix(path, prefix) {
			return dir
		}
	}
	return ""
}

func GetUserDirs(username string) (dirs []string, err error) {
	dirs = make([]string, 0)
	for _, wallPaperDir := range wallPaperDirs {
		dir := filepath.Join(wallPaperDir, username)
		dir, err := filepath.Abs(dir)

		if err != nil {
			continue
		}

		if !strings.HasPrefix(dir, wallPaperDir) {
			continue
		}

		info, err := os.Stat(dir)

		if err != nil && !os.IsExist(err) {
			os.MkdirAll(dir, 0755)
		}

		if info != nil && !info.IsDir() {
			return nil, errors.New("UsernName is not a dir")
		}
		dirs = append(dirs, dir)
	}
	return dirs, nil
}

func RemoveOverflowWallPapers(username string, max int) (error, []string) {
	dirs, err := GetUserDirs(username)
	if err != nil {
		logger.Warning(err)
		return err, nil
	}
	var removed []string
	for _, dir := range dirs {
		fileinfos, err := os.ReadDir(dir)
		if err != nil {
			logger.Warning(err)
			continue
		}

		logger.Debugf("is count %d <= %d ?", len(fileinfos), max)
		if len(fileinfos) <= max {
			continue
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
			file := filepath.Join(dir, fileinfos[i].Name())
			removed = append(removed, file)
			err = os.Remove(file)
			if err != nil {
				logger.Warning(err)
			}
		}
	}
	return nil, removed
}

func DeleteWallPaper(username string, file string) error {
	dirs, err := GetUserDirs(username)
	if err != nil {
		return err
	}

	path, err := filepath.Abs(file)
	if err != nil {
		return err
	}

	if checkPath(path, dirs) == "" {
		return fmt.Errorf("%s is not in %v", file, dirs)
	}

	return os.Remove(path)
}

var wallpaperMutex sync.Mutex

func checkAuth(actionId string, sysBusName string) error {
	ret, err := checkAuthByPolkit(actionId, sysBusName)
	if err != nil {
		return err
	}
	if !ret.IsAuthorized {
		inf, err := getDetailsKey(ret.Details, "polkit.dismissed")
		if err == nil {
			if dismiss, ok := inf.(string); ok {
				if dismiss != "" {
					return errors.New("")
				}
			}
		}
		return fmt.Errorf("Policykit authentication failed")
	}
	return nil
}

func checkAuthByPolkit(actionId string, sysBusName string) (ret polkit.AuthorizationResult, err error) {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return
	}
	authority := polkit.NewAuthority(systemBus)
	subject := polkit.MakeSubject(polkit.SubjectKindSystemBusName)
	subject.SetDetail("name", sysBusName)

	ret, err = authority.CheckAuthorization(0, subject,
		actionId, nil,
		polkit.CheckAuthorizationFlagsAllowUserInteraction, "")
	if err != nil {
		logger.Warningf("call check auth failed, err: %v", err)
		return
	}
	logger.Debugf("call check auth success, ret: %v", ret)
	return
}

func getDetailsKey(details map[string]dbus.Variant, key string) (interface{}, error) {
	result, ok := details[key]
	if !ok {
		return nil, errors.New("key dont exist in details")
	}
	if dutils.IsInterfaceNil(result) {
		return nil, errors.New("result is nil")
	}
	return result.Value(), nil
}

func (d *Daemon) isSelf(sender dbus.Sender, username string) error {
	uid, err := d.service.GetConnUID(string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	user, err := user.LookupId(strconv.Itoa(int(uid)))
	if err != nil {
		return dbusutil.ToError(err)
	}
	if user.Username != username {
		err = fmt.Errorf("%s not allowed to delete %s custom wallpaper", user.Username, username)
		return dbusutil.ToError(err)
	}
	return nil
}

func (d *Daemon) checkAuth(sender dbus.Sender) error {
	return checkAuth(polkitActionUserAdministration, string(sender))
}

func readWallpaperSourceFromFD(fd dbus.UnixFD) ([]byte, error) {
	if fd < 0 {
		return nil, errors.New("invalid wallpaper fd")
	}

	f := os.NewFile(uintptr(fd), "wallpaper")
	if f == nil {
		return nil, errors.New("invalid wallpaper fd")
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("wallpaper source is not a regular file")
	}
	if info.Size() > maxWallpaperSize {
		return nil, fmt.Errorf("file size %d > %d", info.Size(), maxWallpaperSize)
	}

	src, err := io.ReadAll(io.NewSectionReader(f, 0, maxWallpaperSize+1))
	if err != nil {
		return nil, err
	}
	if len(src) > maxWallpaperSize {
		return nil, fmt.Errorf("file size > %d", maxWallpaperSize)
	}
	return src, nil
}

func wallpaperExtFromContent(src []byte) (string, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil {
		return "", fmt.Errorf("invalid wallpaper image: %w", err)
	}

	switch format {
	case "jpeg":
		return ".jpg", nil
	case "png":
		return ".png", nil
	case "bmp":
		return ".bmp", nil
	case "tiff":
		return ".tiff", nil
	default:
		return "", fmt.Errorf("unsupported wallpaper format: %s", format)
	}
}

func (d *Daemon) SaveCustomWallPaper(sender dbus.Sender, username string, fd dbus.UnixFD, isSolid bool) (string, *dbus.Error) {
	var err error
	wallpaperMutex.Lock()
	defer wallpaperMutex.Unlock()

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

	dirs, err := GetUserDirs(username)
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}

	src, err := readWallpaperSourceFromFD(fd)
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}
	ext, err := wallpaperExtFromContent(src)
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}
	md5sum := fmt.Sprintf("%x", md5.Sum(src))

	var prefix string
	if isSolid {
		prefix = solidWallPaperPath
	} else {
		prefix = wallPaperDir
	}
	// 设置壁纸路径
	path := func() string {
		for _, dir := range dirs {
			if strings.HasPrefix(dir, prefix) {
				return dir
			}
		}
		return ""
	}()
	if path == "" {
		err = fmt.Errorf("unknown path: %s", prefix)
		return "", dbusutil.ToError(err)
	}
	destFile := filepath.Join(path, md5sum)
	destFile = destFile + ext
	if dutils.IsFileExist(destFile) {
		return destFile, nil
	}

	err = os.WriteFile(destFile, src, 0644)
	if err != nil {
		logger.Warning(err)
		return "", dbusutil.ToError(err)
	}
	err = d.service.Emit(d, "WallpaperChanged", username, uint32(wallpaperAdd), []string{destFile})
	if err != nil {
		logger.Warning("failed to emit WallpaperChanged signal:", err)
	}
	customWallpaperMaximum := func() int {
		if d.dsSystem != nil {
			v, err := d.dsSystem.Value(0, dsKeyCustomWallpaperMaximum)
			if err != nil {
				logger.Warning(err)
			} else {
				if data, ok := v.Value().(int64); ok {
					return int(data)
				}
			}
		}
		return maxCount
	}

	err, removed := RemoveOverflowWallPapers(username, customWallpaperMaximum())
	if err != nil {
		logger.Warning(err)
	}
	if len(removed) > 0 {
		err = d.service.Emit(d, "WallpaperChanged", username, uint32(wallpaperDel), removed)
		if err != nil {
			logger.Warning("failed to emit WallpaperChanged signal:", err)
		}
	}
	return destFile, dbusutil.ToError(err)
}

func (d *Daemon) DeleteCustomWallPaper(sender dbus.Sender, username string, file string) *dbus.Error {
	err := d.isSelf(sender, username)
	if err != nil {
		logger.Warning(err)

		err = d.checkAuth(sender)
		if err != nil {
			return dbusutil.ToError(err)
		} else {
			return dbusutil.ToError(DeleteWallPaper(username, file))
		}
	}
	err = DeleteWallPaper(username, file)
	if err == nil {
		err = d.service.Emit(d, "WallpaperChanged", username, uint32(wallpaperDel), []string{file})
		if err != nil {
			logger.Warning("failed to emit WallpaperChanged signal:", err)
		}
	}
	return dbusutil.ToError(err)
}

func (*Daemon) GetCustomWallPapers(username string) ([]string, *dbus.Error) {
	dirs, err := GetUserDirs(username)
	if err != nil {
		logger.Warning(err)
		return []string{}, dbusutil.ToError(err)
	}
	files := []string{}

	for _, dir := range dirs {

		fileinfos, err := os.ReadDir(dir)
		if err != nil {
			logger.Warning(err)
			return []string{}, dbusutil.ToError(err)
		}

		for _, fileinfo := range fileinfos {
			files = append(files, filepath.Join(dir, fileinfo.Name()))
		}
	}

	return files, nil
}
