// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func IsContainItem(items []*MenuItem, item *MenuItem) bool {
	for _, i := range items {
		if i.Text == item.Text && i.IsActive == item.IsActive && i.Id != "" {
			return true
		}
	}
	return false
}

func Test_AppendItem(t *testing.T) {
	menu := NewMenu()
	item0 := NewMenuItem("item 0", nil, true)
	item1 := NewMenuItem("item 1", nil, true)
	item2 := NewMenuItem("item 2", nil, true)
	item3 := NewMenuItem("item 3", nil, true)
	menu.AppendItem(item0, item1, item2)

	assert.True(t, IsContainItem(menu.Items, item0))
	assert.True(t, IsContainItem(menu.Items, item1))
	assert.True(t, IsContainItem(menu.Items, item2))
	assert.False(t, IsContainItem(menu.Items, item3))
}

func Test_GenerateMenuJson(t *testing.T) {
	menu := NewMenu()
	item0 := NewMenuItem("item 0", nil, true)
	item1 := NewMenuItem("item 1", nil, true)
	item2 := NewMenuItem("item 2", nil, true)
	menu.AppendItem(item0, item1, item2)

	menuJSON := menu.GenerateJSON()
	assert.Equal(t, menuJSON, `{"items":[{"itemId":"0","itemText":"item 0","isActive":true,"isCheckable":false,"checked":false,"itemIcon":"","itemIconHover":"","itemIconInactive":"","showCheckMark":false,"itemSubMenu":null},{"itemId":"1","itemText":"item 1","isActive":true,"isCheckable":false,"checked":false,"itemIcon":"","itemIconHover":"","itemIconInactive":"","showCheckMark":false,"itemSubMenu":null},{"itemId":"2","itemText":"item 2","isActive":true,"isCheckable":false,"checked":false,"itemIcon":"","itemIconHover":"","itemIconInactive":"","showCheckMark":false,"itemSubMenu":null}],"checkableMenu":false,"singleCheck":false}`)

	var parseResult interface{}
	err := json.Unmarshal([]byte(menuJSON), &parseResult)
	assert.NoError(t, err)
}
