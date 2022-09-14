// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var deskotpFilePathTestMap = map[string]string{
	"/usr/share/applications/deepin-screenshot.desktop":                               "/S@deepin-screenshot",
	"/usr/local/share/applications/wps-office-et.desktop":                             "/L@wps-office-et",
	"/home/tp/.config/dock/scratch/docked:w:42f9e4a33162e38b2febbad0d9e39a3f.desktop": "/D@docked:w:42f9e4a33162e38b2febbad0d9e39a3f",
	"/home/tp/.local/share/applications/webtorrent-desktop.desktop":                   "/H@webtorrent-desktop",
}

func init() {
	homeDir = "/home/tp/"
	scratchDir = homeDir + ".config/dock/scratch/"
	initPathDirCodeMap()
}

func Test_addDesktopExt(t *testing.T) {
	assert.Equal(t, addDesktopExt("0ad"), "0ad.desktop")
	assert.Equal(t, addDesktopExt("0ad.desktop"), "0ad.desktop")
	assert.Equal(t, addDesktopExt("0ad.desktop-x"), "0ad.desktop-x.desktop")
}

func Test_trimDesktopExt(t *testing.T) {
	assert.Equal(t, trimDesktopExt("deepin-movie"), "deepin-movie")
	assert.Equal(t, trimDesktopExt("deepin-movie.desktop"), "deepin-movie")
	assert.Equal(t, trimDesktopExt("deepin-movie.desktop-x"), "deepin-movie.desktop-x")
}

func Test_zipDesktopPath(t *testing.T) {
	for path, zipped := range deskotpFilePathTestMap {
		assert.Equal(t, zipped, zipDesktopPath(path))
	}
}

func Test_unzipDesktopPath(t *testing.T) {
	for path, zipped := range deskotpFilePathTestMap {
		assert.Equal(t, path, unzipDesktopPath(zipped))
	}
}

func Test_getDesktopIdByFilePath(t *testing.T) {
	path := "/usr/share/applications/deepin-screenshot.desktop"
	desktopId := getDesktopIdByFilePath(path)
	assert.Equal(t, desktopId, "deepin-screenshot.desktop")

	path = "/usr/share/applications/kde4/krita.desktop"
	desktopId = getDesktopIdByFilePath(path)
	assert.Equal(t, desktopId, "kde4-krita.desktop")

	path = "/home/tp/.local/share/applications/telegramdesktop.desktop"
	desktopId = getDesktopIdByFilePath(path)
	assert.Equal(t, desktopId, "telegramdesktop.desktop")

	path = "/home/tp/.local/share/applications/dirfortest/dir2/space test.desktop"
	desktopId = getDesktopIdByFilePath(path)
	assert.Equal(t, desktopId, "dirfortest-dir2-space test.desktop")
}

func Test_addDirTrailingSlash(t *testing.T) {
	dir := "/usr/shareapplication"
	dir2 := addDirTrailingSlash(dir)
	assert.Equal(t, dir2, dir+"/")

	dir3 := addDirTrailingSlash(dir2)
	assert.Equal(t, dir3, dir2)
}
