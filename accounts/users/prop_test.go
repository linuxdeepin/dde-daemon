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

package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getGroupInfoWithCache(t *testing.T) {
	groupFileInfo := GroupInfo{Name: "audio", Gid: "29", Users: []string{"pulse", "wen"}}
	groupFileInfoMap, err := getGroupInfoWithCache("testdata/group")

	assert.Nil(t, err)
	val, ok := groupFileInfoMap[groupFileInfo.Name]

	assert.Equal(t, ok, true)

	assert.Equal(t, val, groupFileInfo)

}

func Test_getAdminUserList(t *testing.T) {
	testAdmins := []string{"root", "wen", "test1"}
	admins, err := getAdminUserList("testdata/group", "testdata/sudoers_deepin")
	assert.Nil(t, err)

	assert.ElementsMatch(t, admins, testAdmins)
}
