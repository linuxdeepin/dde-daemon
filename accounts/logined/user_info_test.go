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

package logined

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testSessionInfo1 = &SessionInfo{Uid: 1, Desktop: "deepin", Display: ":0", sessionPath: "/org/freedesktop/login1/session/self_1"}
	testSessionInfo2 = &SessionInfo{Uid: 1, Desktop: "deepin", Display: ":0", sessionPath: "/org/freedesktop/login1/session/self"}
	testSessionInfo3 = &SessionInfo{Uid: 1, Desktop: "uos", Display: ":0", sessionPath: "/org/freedesktop/login1/session/self"}
	testSessionInfo4 = &SessionInfo{Uid: 1, Desktop: "uos1", Display: ":0", sessionPath: "/org/freedesktop/login1/session/self"}
	testSessionInfo5 = &SessionInfo{Uid: 1, Desktop: "uos1", Display: ":0", sessionPath: "/org/freedesktop/login1/session/self"}
	sessionInfos     SessionInfos
)

func Test_Add(t *testing.T) {

	sessionInfos, err := sessionInfos.Add(testSessionInfo1)
	assert.Equal(t, err, true)

	sessionInfos, err = sessionInfos.Add(testSessionInfo2)
	assert.Equal(t, err, true)

	sessionInfos, err = sessionInfos.Add(testSessionInfo3)
	assert.Equal(t, err, true)

	assert.Equal(t, len(sessionInfos), 2)

	sessionInfos, err = sessionInfos.Add(testSessionInfo3)
	assert.Equal(t, err, false)

	assert.Equal(t, len(sessionInfos), 2)

	sessionInfos, err = sessionInfos.Delete(testSessionInfo4.sessionPath)
	assert.Equal(t, err, true)
	assert.Equal(t, len(sessionInfos), 1)

}
func Test_Equal(t *testing.T) {

	assert.Equal(t, testSessionInfo1.Equal(testSessionInfo5), false)
	assert.Equal(t, testSessionInfo1.Equal(testSessionInfo3), false)
}
