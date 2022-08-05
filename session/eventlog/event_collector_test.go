/*
 * Copyright (C) 2019 ~ 2022 Uniontech Software Technology Co.,Ltd
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

package eventlog

import (
	"testing"

	dutils "github.com/linuxdeepin/go-lib/utils"
	"github.com/stretchr/testify/assert"
)

func Test_getUserExpStateFromUserExpPath(t *testing.T) {
	t.Run("Test getUserExpStateFromUserExpPath", func(t *testing.T) {
		userExpPath = "testdata/testuser"
		e := new(EventLog)
		if !dutils.IsFileExist(userExpPath) {
			assert.False(t, e.getUserExpStateFromUserExpPath())
		} else {
			assert.True(t, e.getUserExpStateFromUserExpPath())
		}
	})
}

func Test_getUserExpStateFromDefaultPath(t *testing.T) {
	t.Run("Test getUserExpStateFromDefaultPath", func(t *testing.T) {
		e := new(EventLog)
		defaultExpPath = "testdata/testdefault"
		if !dutils.IsFileExist(defaultExpPath) {
			assert.False(t, e.getUserExpStateFromDefaultPath())
		} else {
			assert.True(t, e.getUserExpStateFromDefaultPath())
		}
	})

}

func Test_setUserExpFileState(t *testing.T) {
	t.Run("Test setUserExpFileState", func(t *testing.T) {
		e := new(EventLog)
		userExpPath = "testdata/testuser"
		assert.NoError(t, e.setUserExpFileState(true))
	})
}
