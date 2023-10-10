// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package background

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
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

const (
	Unknown uint32 = iota
	Normal
	Solid
)

const (
	solidPrefix           = "solid::"
	solidWallPaperPath    = "/usr/share/wallpapers/custom-solidwallpapers"
	sysSolidWallPaperPath = "/usr/share/wallpapers/deepin-solidwallpapers"
)

var NotifyFunc func(string, string)

var _licenseAuthorizationProperty uint32 = 0

var _wallpapersPathMap = make(map[uint32]string)

func SetLogger(value *log.Logger) {
	logger = value
}

func SetCustomWallpaperDeleteCallback(fn func(file string)) {
	customWallpaperDeleteCallback = fn
}

func LicenseAuthorizationProperty() uint32 {
	return _licenseAuthorizationProperty
}

func SetLicenseAuthorizationProperty(value uint32) {
	if value != _licenseAuthorizationProperty {
		_licenseAuthorizationProperty = value
	}
}

func UpdateLicenseAuthorizationProperty() {
	refreshBackground(true)
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

	_wallpapersPathMap[Professional] = "/usr/share/wallpapers/deepin"
	_wallpapersPathMap[Government] = "/usr/share/wallpapers/deepin/deepin-government"
	_wallpapersPathMap[Enterprise] = "/usr/share/wallpapers/deepin/deepin-enterprise"
}

type Background struct {
	Id        string
	Deletable bool
}

type Backgrounds []*Background

func refreshBackground(notify bool) {
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
	var systemWallpapersDir = []string{
		_wallpapersPathMap[Professional],
		sysSolidWallPaperPath,
	}
	for _, file := range getSysBgFiles(systemWallpapersDir) {
		logger.Debugf("system: %s", file)
		bgs = append(bgs, &Background{
			Id:        dutils.EncodeURI(file, dutils.SCHEME_FILE),
			Deletable: false,
		})
	}

	logger.Debug("[refreshBackground] _licenseAuthorizationProperty : ", _licenseAuthorizationProperty)
	// add system, get  enterprise or government systemWallpapers
	if _licenseAuthorizationProperty > Professional && _licenseAuthorizationProperty < uint32(len(_wallpapersPathMap)) {
		for _, file := range getSysBgFiles([]string{_wallpapersPathMap[_licenseAuthorizationProperty]}) {
			logger.Debugf("system: %s", file)
			bgs = append(bgs, &Background{
				Id:        dutils.EncodeURI(file, dutils.SCHEME_FILE),
				Deletable: false,
			})
		}
	}
	// 对比差异，发送壁纸新增和删除的信号
	if notify && NotifyFunc != nil {
		diff := diffBackgroundCache(bgs, backgroundsCache)
		// TODO: 不在此处理壁纸添加信号。用户添加壁纸时，只要路径不是/usr/share/wallpaper，即认为是新壁纸，不管底层是否已经存在
		// if diff["added"] != nil {
		// 	logger.Debug("new wallpapper added!", diff["added"])
		// 	NotifyFunc("background-add", strings.Join(diff["added"], ";"))
		// }
		if diff["deleted"] != nil {
			logger.Debug("new wallpapper deleted!", diff["deleted"])
			NotifyFunc("background-delete", strings.Join(diff["deleted"], ";"))
		}
	}
	backgroundsCache = bgs
}

func diffBackgroundCache(newCache Backgrounds, oldCache Backgrounds) map[string][]string {
	tmp := make(map[string]string)
	for _, newBg := range newCache {
		tmp[newBg.Id] = "added"
		for _, oldBg := range oldCache {
			if tmp[oldBg.Id] == "existed" {
				continue
			}
			if newBg.Id == oldBg.Id {
				tmp[oldBg.Id] = "existed"
			}
			if tmp[oldBg.Id] == "" {
				tmp[oldBg.Id] = "deleted"
			}
		}
	}
	diff := make(map[string][]string)
	for file, stat := range tmp {
		diff[stat] = append(diff[stat], file)
	}
	return diff
}

func ListBackground() Backgrounds {
	backgroundsCacheMu.Lock()
	defer backgroundsCacheMu.Unlock()

	if len(backgroundsCache) == 0 {
		refreshBackground(false)
	}
	return backgroundsCache
}

func NotifyChanged() {
	backgroundsCacheMu.Lock()
	refreshBackground(true)
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

	NotifyChanged()
	return err
}

func (info *Background) Thumbnail() (string, error) {
	return "", errors.New("not supported")
}

func Prepare(file string, t uint32) (string, error) {
	var systemWallpapersDir = []string{
		_wallpapersPathMap[Professional],
		sysSolidWallPaperPath,
	}
	if _licenseAuthorizationProperty > Professional && _licenseAuthorizationProperty < uint32(len(_wallpapersPathMap)) {
		systemWallpapersDir = append(systemWallpapersDir, _wallpapersPathMap[_licenseAuthorizationProperty])
	}

	if isFileInDirs(file, systemWallpapersDir) {
		logger.Debug("is system")
		return file, nil
	}

	logger.Debug("is custom")
	return prepare(file, t)

}

func GetWallpaperType(file string) (string, uint32) {
	t := Normal

	if strings.HasPrefix(file, solidPrefix) {
		file = strings.TrimPrefix(file, solidPrefix)
		t = Solid
	}
	if !IsBackgroundFile(file) {
		t = Unknown
	}
	file = dutils.DecodeURI(file)
	if file == "" {
		t = Unknown
	}
	if strings.HasPrefix(file, sysSolidWallPaperPath) ||
		strings.HasPrefix(file, solidWallPaperPath) {
		t = Solid
	}
	return file, t
}
