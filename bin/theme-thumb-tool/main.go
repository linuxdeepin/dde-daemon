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

package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	dutils "pkg.deepin.io/lib/utils"
)

const (
	TypeAll        = "all"
	TypeGtk        = "gtk"
	TypeIcon       = "icon"
	TypeCursor     = "cursor"
	TypeBackground = "background"

	forceFlagUsage = "Force generate thumbnails"
	destDirUsage   = "Thumbnails output directory"
)

var _forceFlag bool
var _destDir string

func init() {
	flag.BoolVar(&_forceFlag, "force", false, forceFlagUsage)
	flag.BoolVar(&_forceFlag, "f", false, forceFlagUsage)
	flag.StringVar(&_destDir, "output", "", destDirUsage)
	flag.StringVar(&_destDir, "o", "", destDirUsage)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	thumbType := flag.Arg(0)
	var thumbFiles []string
	switch thumbType {
	case TypeAll:
		thumbFiles = genAllThumbnails(_forceFlag)
	case TypeGtk:
		thumbFiles = genGtkThumbnails(_forceFlag)
	case TypeIcon:
		thumbFiles = genIconThumbnails(_forceFlag)
	case TypeCursor:
		thumbFiles = genCursorThumbnails(_forceFlag)
	case TypeBackground:
		thumbFiles = genBgThumbnails(_forceFlag)
	default:
		usage()
	}
	moveThumbFiles(thumbFiles)
}

func usage() {
	fmt.Println("Desc:")
	fmt.Println("\ttheme-thumb-tool - gtk/icon/cursor/background thumbnail batch generator")
	fmt.Println("Usage:")
	fmt.Println("\ttheme-thumb-tool [Option] [Type]")
	fmt.Println("Option:")
	fmt.Println("\t-f --force: force to generate thumbnail regardless of file exist")
	fmt.Println("\t-o --output: thumbnails output directory")
	fmt.Println("Type:")
	fmt.Println("\tall: generate all of the following types thumbnails")
	fmt.Println("\tgtk: generate all gtk theme thumbnails")
	fmt.Println("\ticon: generate all icon theme thumbnails")
	fmt.Println("\tcursor: generate all cursor theme thumbnails")
	fmt.Println("\tbackground: generate all background thumbnails")

	os.Exit(0)
}

func moveThumbFiles(files []string) {
	if len(_destDir) == 0 {
		return
	}

	err := os.MkdirAll(_destDir, 0755)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "create %q failed: %v\n", _destDir, err)
		return
	}
	for _, file := range files {
		dest := path.Join(_destDir, path.Base(file))
		if !_forceFlag && dutils.IsFileExist(dest) {
			continue
		}
		err = dutils.CopyFile(file, dest)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "copy file %q to %q failed: %v\n", file, dest, err)
			continue
		}
		err = os.Remove(file)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "delete file %q failed: %v\n", file, err)
		}
	}
}
