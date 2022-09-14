// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package background

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/godbus/dbus"
	daemon "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.daemon"
	"github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/graphic"
	"github.com/linuxdeepin/go-lib/imgutil"
	"github.com/linuxdeepin/go-lib/strv"
	dutils "github.com/linuxdeepin/go-lib/utils"
	"github.com/nfnt/resize"
)

func sumFileMd5(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func updateModTime(file string) {
	now := time.Now()
	err := os.Chtimes(file, now, now)
	if err != nil {
		logger.Warning("failed to update cache file modify time:", err)
	}
}

func resizeImage(filename, cacheDir string) (outFilename, ext string, isResized bool) {
	img, err := imgutil.Load(filename)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to load image file %q: %v\n", filename, err)
		outFilename = filename
		return
	}

	const (
		stdWidth  = 3840
		stdHeight = 2400
	)

	imgWidth := img.Bounds().Dx()
	imgHeight := img.Bounds().Dy()

	if imgWidth <= stdWidth && imgHeight <= stdHeight {
		// no need to resize
		outFilename = filename
		return
	}

	ext = "jpg"
	format := graphic.FormatJpeg
	_, err = os.Stat("/usr/share/wallpapers/deepin/desktop.bmp")
	if err == nil {
		ext = "bmp"
		format = graphic.FormatBmp
	}

	fh, err := ioutil.TempFile(cacheDir, "tmp-")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "failed to create temp file:", err)
		outFilename = filename
		return
	}

	// tmp-###
	outFilename = fh.Name()
	err = fh.Close()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "failed to close temp file:", err)
	}

	if float64(imgWidth)/float64(imgHeight) > float64(stdWidth)/float64(stdHeight) {
		// use std height
		imgWidth = 0
		imgHeight = stdHeight
	} else {
		// use std width
		imgWidth = stdWidth
		imgHeight = 0
	}

	img = resize.Resize(uint(imgWidth), uint(imgHeight), img, resize.Lanczos3)
	err = graphic.SaveImage(outFilename, img, format)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to save image file %q: %v\n",
			outFilename, err)
		outFilename = filename
		return
	}

	isResized = true
	return
}

func prepare(filename string) (string, error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}

	file, _, _ := resizeImage(filename, CustomWallpapersConfigDir)

	dm := daemon.NewDaemon(bus)
	cur, err := user.Current()
	if err != nil {
		return "", err
	}

	NotifyChanged()

	return dm.SaveCustomWallPaper(0, cur.Username, file)
}

func shrinkCache(cacheFileBaseName string) {
	gs := gio.NewSettings("com.deepin.dde.appearance")
	defer gs.Unref()

	workspaceBackgrounds := gs.GetStrv("background-uris")
	var notDeleteFiles strv.Strv
	notDeleteFiles = append(notDeleteFiles, cacheFileBaseName)
	for _, uri := range workspaceBackgrounds {
		wbFile := dutils.DecodeURI(uri)
		if strings.HasPrefix(wbFile, CustomWallpapersConfigDir) {
			// is custom wallpaper
			basename := filepath.Base(wbFile)
			if basename != cacheFileBaseName {
				notDeleteFiles = append(notDeleteFiles, basename)
			}
		}
	}
	deleteOld(notDeleteFiles)
}

func deleteOld(notDeleteFiles strv.Strv) {
	fileInfos, _ := ioutil.ReadDir(CustomWallpapersConfigDir)
	count := len(fileInfos) - customWallpapersLimit
	if count <= 0 {
		return
	}
	logger.Debugf("need delete %d file(s)", count)

	sort.Sort(byModTime(fileInfos))
	for _, fileInfo := range fileInfos {
		if count == 0 {
			break
		}

		// traverse from old to new
		fileBaseName := fileInfo.Name()
		if !notDeleteFiles.Contains(fileBaseName) {
			logger.Debug("delete", fileBaseName)
			fullPath := filepath.Join(CustomWallpapersConfigDir, fileBaseName)
			err := os.Remove(fullPath)
			if os.IsNotExist(err) {
				err = nil
			}

			if err == nil {
				count--

				if customWallpaperDeleteCallback != nil {
					customWallpaperDeleteCallback(fullPath)
				}
			} else {
				logger.Warning(err)
			}
		}
	}
}

type byModTime []os.FileInfo

func (a byModTime) Len() int      { return len(a) }
func (a byModTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byModTime) Less(i, j int) bool {
	return a[i].ModTime().Unix() < a[j].ModTime().Unix()
}
