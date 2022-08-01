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

package background

import (
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sort"

	"github.com/godbus/dbus"
	daemon "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.daemon"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

// ListDirs list all background dirs
func ListDirs() []string {
	var result []string
	result = append(result, _wallpapersPathMap[Professional])
	if _licenseAuthorizationProperty > Professional && _licenseAuthorizationProperty < uint32(len(_wallpapersPathMap)) {
		result = append(result, _wallpapersPathMap[_licenseAuthorizationProperty])
	}
	result = append(result, CustomWallpapersConfigDir)
	return result
}

// 根据时间排序文件
func sortByTime(fileInfoList []os.FileInfo) []os.FileInfo {
	sort.Slice(fileInfoList, func(i, j int) bool {
		fileInfoI := fileInfoList[i]
		fileInfoJ := fileInfoList[j]
		if fileInfoI.ModTime().After(fileInfoJ.ModTime()) {
			return true
		} else if fileInfoI.ModTime().Equal(fileInfoJ.ModTime()) {
			if fileInfoI.Name() < fileInfoJ.Name() {
				return true
			}
		}
		return false
	})

	return fileInfoList
}

func getSysBgFiles(path string) []string {
	var files []string
	if dutils.IsFileExist(path) {
		files = append(files, getBgFilesInDir(path)...)
	}
	return files
}

func getCustomBgFiles() []string {
	bus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return []string{}
	}

	dm := daemon.NewDaemon(bus)
	cur, err := user.Current()
	if err != nil {
		logger.Warning(err)
		return []string{}
	}

	files, err := dm.GetCustomWallPapers(0, cur.Username)
	if err != nil {
		logger.Warning(err)
	}

	return files
}

func getCustomBgFilesInDir(dir string) []string {
	fileInfoList, err := ioutil.ReadDir(dir)
	if err != nil {
		logger.Warning(err)
		return nil
	}

	sortByTime(fileInfoList)

	var wallpapers []string
	for _, info := range fileInfoList {
		// 只处理custom-wallpapers目录下的壁纸图片文件
		if info.IsDir() {
			continue
		}
		path := filepath.Join(dir, info.Name())
		if !IsBackgroundFile(path) {
			continue
		}
		wallpapers = append(wallpapers, path)
	}

	return wallpapers
}

func getBgFilesInDir(dir string) []string {
	fr, err := os.Open(dir)
	if err != nil {
		return []string{}
	}
	defer fr.Close()

	names, err := fr.Readdirnames(0)
	if err != nil {
		return []string{}
	}

	var walls []string
	for _, name := range names {
		path := filepath.Join(dir, name)
		if !IsBackgroundFile(path) {
			continue
		}
		walls = append(walls, path)
	}
	return walls
}

func isFileInDirs(file string, dirs []string) bool {
	for _, dir := range dirs {
		if filepath.Dir(file) == dir {
			return true
		}
	}
	return false
}
