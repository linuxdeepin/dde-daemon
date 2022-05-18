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

package checkers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_CheckUsername(t *testing.T) {
	type checkRet struct {
		name string
		code ErrorCode
	}

	var infos = []checkRet{
		{"", ErrCodeEmpty},
		{"a1111111111111111111111111111111", 0},
		{"music", 0},
		{"music-player", 0},
		{"music_player", 0},
		{"MusicPlayer", 0},
		{"Music-_-Player", 0},
		{"0MusicPlayer", 0},
		{"-MusicPlayer", ErrCodeFirstCharInvalid},
		{"_MusicPlayer", ErrCodeFirstCharInvalid},
		{"a11111111111111111111111111111111", ErrCodeLen},
		{"a1", ErrCodeLen},
		{"root", ErrCodeSystemUsed},
		{"a123*&", ErrCodeInvalidChar},
	}

	for _, v := range infos {
		tmp := CheckUsernameValid(v.name)
		if v.code == 0 {
			assert.Equal(t, tmp, (*ErrorInfo)(nil))
		} else {
			assert.Equal(t, tmp.Code, v.code)
		}
	}
}

func Test_GetUsernames(t *testing.T) {
	var datas = []struct {
		name string
		ret  bool
	}{
		{
			name: "test1",
			ret:  true,
		},
		{
			name: "test2",
			ret:  true,
		},
		{
			name: "test3",
			ret:  false,
		},
	}

	names, err := getAllUsername("testdata/passwd")
	assert.Equal(t, err, nil)
	assert.Equal(t, len(names), 2)

	for _, data := range datas {
		assert.Equal(t, isStrInArray(data.name, names), data.ret)
		assert.Equal(t, isStrInArray(data.name, names), data.ret)
		assert.Equal(t, isStrInArray(data.name, names), data.ret)
	}
}

func Test_CheckPasswordValid(t *testing.T) {
	type passwordCheckPair struct {
		str     string
		errCode passwordErrorCode
		Prompt  string
		isOK    bool
	}

	passwordStrErrList := []passwordCheckPair{
		{"", passwordErrCodeShort, "Please enter a password not less than 8 characters", false},
		{"aa", passwordErrCodeShort, "Please enter a password not less than 8 characters", false},
		{"aA1?", passwordErrCodeShort, "Please enter a password not less than 8 characters", false},
		{"aaaaaaaa", passwordErrCodeSimple, "The password must contain English letters (case-sensitive), numbers or special symbols (~!@#$%^&*()[]{}\\|/?,.<>)", false},
		{"aaaaAAAA", passwordErrCodeSimple, "The password must contain English letters (case-sensitive), numbers or special symbols (~!@#$%^&*()[]{}\\|/?,.<>)", false},
		{"aaaaAA12", passwordErrCodeSimple, "The password must contain English letters (case-sensitive), numbers or special symbols (~!@#$%^&*()[]{}\\|/?,.<>)", false},
		{"aaaaaa1?", passwordErrCodeSimple, "The password must contain English letters (case-sensitive), numbers or special symbols (~!@#$%^&*()[]{}\\|/?,.<>)", false},
		{"AAAAAA1?", passwordErrCodeSimple, "The password must contain English letters (case-sensitive), numbers or special symbols (~!@#$%^&*()[]{}\\|/?,.<>)", false},
		{"aaaaA12?", passwordOK, "", true},
	}

	releaseType := "Server"
	for _, v := range passwordStrErrList {
		errCode := CheckPasswordValid(releaseType, v.str)
		assert.Equal(t, errCode, v.errCode)
		assert.Equal(t, errCode.IsOk(), v.isOK)
		assert.Equal(t, errCode.Prompt(), v.Prompt)
	}

	releaseType = "Desktop"
	for _, v := range passwordStrErrList {
		errCode := CheckPasswordValid(releaseType, v.str)
		assert.Equal(t, errCode, passwordOK)
		assert.True(t, errCode.IsOk())
		assert.Equal(t, errCode.Prompt(), "")
	}
}
