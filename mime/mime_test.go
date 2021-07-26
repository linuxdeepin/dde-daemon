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

package mime

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppInfos(t *testing.T) {
	var infos = AppInfos{
		&AppInfo{
			Id:   "gvim.desktop",
			Name: "gvim",
			Exec: "gvim",
		},
		&AppInfo{
			Id:   "firefox.desktop",
			Name: "Firefox",
			Exec: "firefox",
		}}
	assert.Equal(t, len(infos.Delete("gvim.desktop")), 1)
	assert.Equal(t, len(infos.Delete("vim.desktop")), 2)
}

func TestUnmarshal(t *testing.T) {
	table, err := unmarshal("testdata/data.json")
	assert.NoError(t, err)
	assert.Equal(t, len(table.Apps), 2)

	assert.ElementsMatch(t, table.Apps[0].AppId, []string{"org.gnome.Nautilus.desktop"})
	assert.Equal(t, table.Apps[0].AppType, "file-manager")
	assert.ElementsMatch(t, table.Apps[0].Types, []string{
		"inode/directory",
		"application/x-gnome-saved-search",
	})

	assert.ElementsMatch(t, table.Apps[1].AppId, []string{"org.gnome.gedit.desktop"})
	assert.Equal(t, table.Apps[1].AppType, "editor")
	assert.ElementsMatch(t, table.Apps[1].Types, []string{
		"text/plain",
	})
}

func TestIsStrInList(t *testing.T) {
	var list = []string{"abc", "abs"}
	assert.Equal(t, isStrInList("abs", list), true)
	assert.Equal(t, isStrInList("abd", list), false)
}

func TestUserAppInfo(t *testing.T) {
	var infos = userAppInfos{
		{
			DesktopId: "test-web.desktop",
			SupportedMime: []string{
				"application/test.xml",
				"application/test.html",
			},
		},
		{
			DesktopId: "test-doc.desktop",
			SupportedMime: []string{
				"application/test.doc",
				"application/test.xls",
			},
		},
	}
	var file = "testdata/tmp_user_mime.json"
	var manager = &userAppManager{
		appInfos: infos,
		filename: file,
	}
	assert.Equal(t, manager.Get("application/test.xml")[0].DesktopId, "test-web.desktop")
	assert.Nil(t, manager.Get("application/test.ppt"))
	assert.Equal(t, manager.Add([]string{"application/test.xml"}, "test-web.desktop"), false)
	assert.Equal(t, manager.Add([]string{"application/test.ppt"}, "test-doc.desktop"), true)
	assert.Equal(t, manager.Get("application/test.ppt")[0].DesktopId, "test-doc.desktop")
	assert.Nil(t, manager.Delete("test-web.desktop"))
	assert.NotNil(t, manager.Delete("test-xxx.desktop"))
	assert.Nil(t, manager.Get("application/test.xml"))
	assert.Nil(t, manager.Write())
	tmp, err := newUserAppManager(file)
	assert.NoError(t, err)
	assert.Nil(t, tmp.Get("application/test.xml"))
	assert.Equal(t, tmp.Get("application/test.ppt")[0].DesktopId, "test-doc.desktop")
	os.Remove(file)
}
