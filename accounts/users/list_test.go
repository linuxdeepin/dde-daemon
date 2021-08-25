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

func Test_UserInfos(t *testing.T) {
	var testNames = []string{"test1", "test2", "vbox"}
	var testInfos = []struct {
		calLength           int
		checkLength         error
		commentInfo         *CommentInfo
		isHumanUser         bool
		isHumanViaLoginDefs bool
		isHumanViaShell     bool
	}{
		{calLength: 41,
			checkLength:         nil,
			commentInfo:         &CommentInfo{},
			isHumanUser:         true,
			isHumanViaLoginDefs: true,
			isHumanViaShell:     true,
		},
		{calLength: 41,
			checkLength:         nil,
			commentInfo:         &CommentInfo{},
			isHumanUser:         true,
			isHumanViaLoginDefs: true,
			isHumanViaShell:     true,
		},
		{calLength: 28,
			checkLength:         nil,
			commentInfo:         &CommentInfo{},
			isHumanUser:         false,
			isHumanViaLoginDefs: false,
			isHumanViaShell:     false,
		},
	}
	infos, err := getUserInfosFromFile("testdata/passwd")
	assert.Nil(t, err)
	names := infos.GetUserNames()

	assert.Equal(t, testNames, names)

	for i, val := range infos {

		assert.Equal(t, testInfos[i].calLength, val.calLength())
		assert.Equal(t, testInfos[i].checkLength, val.checkLength())
		assert.Equal(t, testInfos[i].commentInfo, val.Comment())
		assert.Equal(t, testInfos[i].isHumanUser, val.isHumanUser("testdata/login.defs"))
		assert.Equal(t, testInfos[i].isHumanViaLoginDefs, val.isHumanViaLoginDefs("testdata/login.defs"))
		assert.Equal(t, testInfos[i].isHumanUser, val.isHumanViaShell())
	}
}
