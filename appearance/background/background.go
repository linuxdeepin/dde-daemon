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
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"

	"github.com/godbus/dbus"
	daemon "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.daemon"
	"github.com/linuxdeepin/go-lib/imgutil"

	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/strv"
	dutils "github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

var (
	backgroundsCache   Backgrounds
	backgroundsCacheMu sync.Mutex
	fsChanged          bool

	CustomWallpapersConfigDir     string
	customWallpaperDeleteCallback func(file string)
	logger                        *log.Logger
)

const customWallpapersLimit = 10

//0：专业版  1： 政务授权 2： 企业授权
const (
	Professional uint32 = iota
	Government
	Enterprise
	Count
)
var _licenseAuthorizationProperty uint32 = 0

var _wallpapersPathMap = make(map[uint32]string)

func SetLogger(value *log.Logger) {
	logger = value
}

func SetCustomWallpaperDeleteCallback(fn func(file string)) {
	customWallpaperDeleteCallback = fn
}

func init() {
	logger = log.NewLogger("background")
	SetLogger(logger)
	CustomWallpapersConfigDir = filepath.Join(basedir.GetUserConfigDir(),
		"deepin/dde-daemon/appearance/custom-wallpapers")
	err := os.MkdirAll(CustomWallpapersConfigDir, 0755)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}

	//get com.deepin.license Authorization type
	_licenseAuthorizationProperty = getLicenseAuthorizationProperty()

	_wallpapersPathMap[Professional] = "/usr/share/wallpapers/deepin"
	_wallpapersPathMap[Government] = "/usr/share/wallpapers/deepin/deepin-government"
	_wallpapersPathMap[Enterprise] = "/usr/share/wallpapers/deepin/deepin-enterprise"
}

func getLicenseAuthorizationProperty() uint32 {
	conn, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return 0
	}
	var variant dbus.Variant
	err = conn.Object("com.deepin.license", "/com/deepin/license/Info").Call(
		"org.freedesktop.DBus.Properties.Get", 0, "com.deepin.license.Info", "AuthorizationProperty").Store(&variant)
	if err != nil {
		logger.Warning(err)
		return 0
	}
	if variant.Signature().String() != "u" {
		logger.Warning("not excepted value type")
		return 0
	}
	return variant.Value().(uint32)
}

type Background struct {
	Id        string
	Deletable bool
}

type Backgrounds []*Background

func refreshBackground() {
	if logger != nil {
		logger.Debug("refresh background")
	}
	var bgs Backgrounds
	// add custom
	for _, file := range getCustomBgFiles() {
		logger.Debugf("custom: %s", file)
		bgs = append(bgs, &Background{
			Id:        dutils.EncodeURI(file, dutils.SCHEME_FILE),
			Deletable: true,
		})
	}

	// add system, get default systemWallpapers
	for _, file := range getSysBgFiles(_wallpapersPathMap[Professional]) {
		logger.Debugf("system: %s", file)
		bgs = append(bgs, &Background{
			Id:        dutils.EncodeURI(file, dutils.SCHEME_FILE),
			Deletable: false,
		})
	}

	// add system, get  enterprise or government systemWallpapers
	if _licenseAuthorizationProperty > Professional && _licenseAuthorizationProperty < uint32(len(_wallpapersPathMap)) {
		for _, file := range getSysBgFiles(_wallpapersPathMap[_licenseAuthorizationProperty]) {
			logger.Debugf("system: %s", file)
			bgs = append(bgs, &Background{
				Id:        dutils.EncodeURI(file, dutils.SCHEME_FILE),
				Deletable: false,
			})
		}
	}

	backgroundsCache = bgs
	fsChanged = false
}

func ListBackground() Backgrounds {
	backgroundsCacheMu.Lock()
	defer backgroundsCacheMu.Unlock()

	if len(backgroundsCache) == 0 || fsChanged {
		refreshBackground()
	}
	return backgroundsCache
}

func NotifyChanged() {
	backgroundsCacheMu.Lock()
	fsChanged = true
	backgroundsCacheMu.Unlock()
}

var uiSupportedFormats = strv.Strv([]string{"jpeg", "png", "bmp", "tiff", "gif"})

func IsBackgroundFile(file string) bool {
	file = dutils.DecodeURI(file)
	format, err := imgutil.SniffFormat(file)
	if err != nil {
		return false
	}

	if uiSupportedFormats.Contains(format) {
		return true
	}
	return false
}

func (bgs Backgrounds) Get(uri string) *Background {
	uri = dutils.EncodeURI(uri, dutils.SCHEME_FILE)
	for _, info := range bgs {
		if uri == info.Id {
			return info
		}
	}
	return nil
}

func (bgs Backgrounds) ListGet(uris []string) Backgrounds {
	var ret Backgrounds
	for _, uri := range uris {
		v := bgs.Get(uri)
		if v == nil {
			continue
		}
		ret = append(ret, v)
	}
	return ret
}

func (bgs Backgrounds) Delete(uri string) error {
	info := bgs.Get(uri)
	if info == nil {
		return fmt.Errorf("not found '%s'", uri)
	}

	NotifyChanged()
	return info.Delete()
}

func (bgs Backgrounds) Thumbnail(uri string) (string, error) {
	return "", errors.New("not supported")
}

func (info *Background) Delete() error {
	if !info.Deletable {
		return fmt.Errorf("not custom")
	}

	bus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return err
	}

	dm := daemon.NewDaemon(bus)
	cur, err := user.Current()
	if err != nil {
		logger.Warning(err)
		return err
	}

	file := dutils.DecodeURI(info.Id)
	err = dm.DeleteCustomWallPaper(0, cur.Username, file)
	if err != nil {
		logger.Warning(err)
		return err
	}

	if customWallpaperDeleteCallback != nil {
		customWallpaperDeleteCallback(file)
	}
	return err
}

func (info *Background) Thumbnail() (string, error) {
	return "", errors.New("not supported")
}

func Prepare(file string) (string, error) {
	var systemWallpapersDir = []string {
		_wallpapersPathMap[Professional],
	}
	if _licenseAuthorizationProperty > Professional && _licenseAuthorizationProperty < uint32(len(_wallpapersPathMap)) {
		systemWallpapersDir = append(systemWallpapersDir, _wallpapersPathMap[_licenseAuthorizationProperty])
	}

	file = dutils.DecodeURI(file)
	if isFileInDirs(file, systemWallpapersDir) {
		logger.Debug("is system")
		return file, nil
	}

	logger.Debug("is custom")
	return prepare(file)
}
