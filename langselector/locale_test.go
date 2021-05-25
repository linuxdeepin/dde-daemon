/*
 * Copyright (C) 2013 ~ 2018 Deepin Technology Co., Ltd.
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

package langselector

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GenerateLocaleEnvFile(t *testing.T) {
	example := `LANG=en_US.UTF-8
LANGUAGE=en_US
LC_TIME="zh_CN.UTF-8"
`
	assert.Equal(t, string(generateLocaleEnvFile("en_US.UTF-8",
		"testdata/pam_environment")), example)
}

func Test_GetLocale(t *testing.T) {
	l, err := getLocaleFromFile("testdata/pam_environment")
	assert.Nil(t, err)
	assert.Equal(t, l, "zh_CN.UTF-8")

	l = getCurrentUserLocale()
	assert.NotEqual(t, len(l), 0)
}

func Test_WriteUserLocale(t *testing.T) {
	assert.Nil(t, writeLocaleEnvFile("zh_CN.UTF-8", "testdata/pam_environment", "testdata/pam"))
	os.RemoveAll("testdata/pam")
}
