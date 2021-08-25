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
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"pkg.deepin.io/lib/log"
	"pkg.deepin.io/lib/strv"
	dutils "pkg.deepin.io/lib/utils"
)

func init() {
	SetLogger(log.NewLogger("daemon/appearance"))

}

func Test_Scanner(t *testing.T) {
	assert.ElementsMatch(t, getBgFilesInDir("testdata/Theme1/wallpapers"),
		[]string{
			"testdata/Theme1/wallpapers/desktop.jpg",
		})
	assert.Equal(t, getBgFilesInDir("testdata/Theme3/wallpapers"), []string{})
}

func Test_FileInDirs(t *testing.T) {
	var dirs = []string{
		"/tmp/backgrounds",
		"/tmp/wallpapers",
	}

	assert.Equal(t, isFileInDirs("/tmp/backgrounds/1.jpg", dirs),
		true)
	assert.Equal(t, isFileInDirs("/tmp/wallpapers/1.jpg", dirs),
		true)
	assert.Equal(t, isFileInDirs("/tmp/background/1.jpg", dirs),
		false)
}

func Test_GetBgFiles(t *testing.T) {
	files := getSysBgFiles()
	t.Log(files)
}

func Test_getCustomBgFilesInDir(t *testing.T) {

	assert.ElementsMatch(t, getCustomBgFilesInDir("testdata/Theme1/wallpapers"),
		[]string{
			"testdata/Theme1/wallpapers/desktop.jpg",
		})
}

func Test_ListDirs(t *testing.T) {

	varTest := []string{"/usr/share/custom-wallpapers/deepin", "/usr/share/wallpapers/deepin"}

	CustomWallpapersConfigDir = "/usr/share/custom-wallpapers/deepin"
	val := ListDirs()

	assert.ElementsMatch(t, varTest, val)

}

func Test_IsBackgroundFile(t *testing.T) {
	testFilePath1 := "file:///usr/share/backgrounds/gnome/adwaita-timed.xml"
	testFilePath2 := "testdata/Theme1/wallpapers/desktop.jpg"
	assert.Equal(t, IsBackgroundFile(testFilePath1), false)

	assert.Equal(t, IsBackgroundFile(testFilePath2), true)

	NotifyChanged()

	assert.Equal(t, fsChanged, true)

}
func DeleteCallack(file string) {

}
func Test_SetCustomWallpaperDeleteCallback(t *testing.T) {
	SetCustomWallpaperDeleteCallback(DeleteCallack)

}

func Test_Backgrounds(t *testing.T) {

	cmd1 := exec.Command("cp", "testdata/Theme1/wallpapers/desktop.jpg", "testdata/Theme1/wallpapers/desktop0.jpg")
	cmd1.Run()
	defer func() {
		cmd2 := exec.Command("rm", "testdata/Theme1/wallpapers/desktop0.jpg")
		cmd2.Run()
	}()

	var bgs Backgrounds
	for _, file := range getCustomBgFilesInDir("testdata/Theme1/wallpapers") {
		bgs = append(bgs, &Background{
			Id:        dutils.EncodeURI(file, dutils.SCHEME_FILE),
			Deletable: true,
		})
	}
	assert.Equal(t, len(bgs), 2)

	val, _ := bgs.Thumbnail("testdata/Theme1/wallpapers/desktop.jpg")
	assert.Equal(t, val, "")

	val, _ = bgs[0].Thumbnail()
	assert.Equal(t, val, "")

	assert.Equal(t, len(bgs.ListGet([]string{"fake/Theme1/wallpapers/desktop"})), 0)

}

func Test_sumFileMd5(t *testing.T) {

	md5, _ := sumFileMd5("testdata/Theme1/wallpapers/desktop.jpg")
	assert.Equal(t, md5, "fafa2baf6dba60d3e47b8c7fe4eea9e9")

	updateModTime("testdata/Theme1/wallpapers/desktop.jpg")
}

func Test_deleteOld(t *testing.T) {

	for i := 0; i < 10; i++ {

		cmd1 := exec.Command("cp", "testdata/Theme1/wallpapers/desktop.jpg", "testdata/Theme1/wallpapers/desktop"+strconv.Itoa(i)+".jpg")
		cmd1.Run()
	}
	defer func() {
		for i := 0; i < 10; i++ {
			// 保留desktop.jpg文件
			cmd1 := exec.Command("rm", "testdata/Theme1/wallpapers/desktop"+strconv.Itoa(i)+".jpg")
			cmd1.Run()
		}
	}()
	var notDeleteFiles strv.Strv = []string{"desktop.jpg"}
	CustomWallpapersConfigDir = "testdata/Theme1/wallpapers/"
	assert.Equal(t, len(getBgFilesInDir("testdata/Theme1/wallpapers")), 11)
	deleteOld(notDeleteFiles)
	assert.Equal(t, len(getBgFilesInDir("testdata/Theme1/wallpapers")), 10)

}

func Test_ListBackground(t *testing.T) {

	var testInfos = []*Background{
		{
			Id:        "file://testdata/Theme1/wallpapers/desktop.jpg",
			Deletable: true,
		},
	}

	CustomWallpapersConfigDir = "testdata/Theme1/wallpapers/"
	systemWallpapersDir = []string{
		"testdata/Theme2/wallpapers/",
	}
	bgs := ListBackground()

	assert.ElementsMatch(t, testInfos, bgs)
}
