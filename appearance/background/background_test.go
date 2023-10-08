// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package background

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LicenseAuthorizationProperty(t *testing.T) {
	t.Log(LicenseAuthorizationProperty())
}

func Test_SetLicenseAuthorizationProperty(t *testing.T) {
	SetLicenseAuthorizationProperty(30)
	t.Log(_licenseAuthorizationProperty)
	assert.Equal(t, _licenseAuthorizationProperty, uint32(30))
}

func Test_Scanner(t *testing.T) {
	assert.ElementsMatch(t, getBgFilesInDir("testdata/Theme1/wallpapers"),
		[]string{
			"testdata/Theme1/wallpapers/desktop.jpg",
		})
	assert.Nil(t, getBgFilesInDir("testdata/Theme2/wallpapers"))
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
	files := getSysBgFiles([]string{"/usr/share/wallpapers/deepin"})
	t.Log(files)
}
