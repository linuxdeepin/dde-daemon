// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"path"
	"time"

	"github.com/linuxdeepin/go-lib/graphic"
	"github.com/linuxdeepin/dde-api/themes"
	"github.com/linuxdeepin/dde-api/thumbnails/cursor"
	"github.com/linuxdeepin/dde-api/thumbnails/gtk"
	"github.com/linuxdeepin/dde-api/thumbnails/icon"
	"github.com/linuxdeepin/dde-api/thumbnails/images"
	"github.com/linuxdeepin/dde-daemon/appearance/background"
)

const (
	thumbBgDir = "/var/cache/appearance/thumbnail/background"

	defaultWidth  = 128
	defaultHeight = 72
)

func genAllThumbnails(force bool) []string {
	var ret []string
	ret = append(ret, genGtkThumbnails(force)...)
	ret = append(ret, genIconThumbnails(force)...)
	ret = append(ret, genCursorThumbnails(force)...)
	ret = append(ret, genBgThumbnails(force)...)
	return ret
}

func genGtkThumbnails(force bool) []string {
	var ret []string
	list := themes.ListGtkTheme()
	for _, v := range list {
		thumb, err := gtk.ThumbnailForTheme(path.Join(v, "index.theme"),
			getThumbBg(), defaultWidth, defaultHeight, force)
		if err != nil {
			fmt.Printf("Gen '%s' thumbnail failed: %v\n", v, err)
			continue
		}
		ret = append(ret, thumb)
	}
	return ret
}

func genIconThumbnails(force bool) []string {
	var ret []string
	list := themes.ListIconTheme()
	for _, v := range list {
		thumb, err := icon.ThumbnailForTheme(path.Join(v, "index.theme"),
			getThumbBg(), defaultWidth, defaultHeight, force)
		if err != nil {
			fmt.Printf("Gen '%s' thumbnail failed: %v\n", v, err)
			continue
		}
		ret = append(ret, thumb)
	}
	return ret
}

func genCursorThumbnails(force bool) []string {
	var ret []string
	list := themes.ListCursorTheme()
	for _, v := range list {
		thumb, err := cursor.ThumbnailForTheme(path.Join(v, "cursor.theme"),
			getThumbBg(), defaultWidth, defaultHeight, force)
		if err != nil {
			fmt.Printf("Gen '%s' thumbnail failed: %v\n", v, err)
			continue
		}
		ret = append(ret, thumb)
	}
	return ret
}

func genBgThumbnails(force bool) []string {
	var ret []string
	infos := background.ListBackground()
	for _, info := range infos {
		thumb, err := images.ThumbnailForTheme(info.Id,
			defaultWidth, defaultHeight, force)
		if err != nil {
			fmt.Printf("Gen '%s' thumbnail failed: %v\n", info.Id, err)
			continue
		}
		ret = append(ret, thumb)
	}
	return ret
}

func getThumbBg() string {
	var imgs = getImagesInDir()
	if len(imgs) == 0 {
		return ""
	}

	rand.Seed(time.Now().UnixNano())
	// #nosec G404
	idx := rand.Intn(len(imgs))
	return imgs[idx]
}

func getImagesInDir() []string {
	finfos, err := ioutil.ReadDir(thumbBgDir)
	if err != nil {
		return nil
	}

	var imgs []string
	for _, finfo := range finfos {
		tmp := path.Join(thumbBgDir, finfo.Name())
		if !graphic.IsSupportedImage(tmp) {
			continue
		}
		imgs = append(imgs, tmp)
	}
	return imgs
}
