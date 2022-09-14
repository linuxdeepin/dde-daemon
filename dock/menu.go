// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"encoding/json"
	"errors"
	"strconv"
)

/*
json sample
{
   "checkableMenu" : false,
   "items" : [
      {
         "itemText" : "item 1",
         "isActive" : true,
         "itemSubMenu" : nil,
         "itemId" : "2",
         "itemIconInactive" : "",
         "checked" : false,
         "itemIconHover" : "",
         "itemIcon" : "",
         "showCheckMark" : false,
         "isCheckable" : false
      },
   ],
   "singleCheck" : false
}
*/

type MenuItem struct {
	Id            string `json:"itemId"`
	Text          string `json:"itemText"`
	IsActive      bool   `json:"isActive"`
	IsCheckable   bool   `json:"isCheckable"`
	Checked       bool   `json:"checked"`
	Icon          string `json:"itemIcon"`
	IconHover     string `json:"itemIconHover"`
	IconInactive  string `json:"itemIconInactive"`
	ShowCheckMark bool   `json:"showCheckMark"`
	SubMenu       *Menu  `json:"itemSubMenu"`

	hint   int
	action func(uint32)
}

const menuItemHintShowAllWindows = 1

func NewMenuItem(name string, action func(uint32), enable bool) *MenuItem {
	return &MenuItem{
		Text:     name,
		IsActive: enable,
		action:   action,
	}
}

type Menu struct {
	Items         []*MenuItem `json:"items"`
	CheckableMenu bool        `json:"checkableMenu"`
	SingleCheck   bool        `json:"singleCheck"`

	itemCount int64
}

func NewMenu() *Menu {
	return &Menu{}
}

func (m *Menu) allocateId() string {
	idStr := strconv.FormatInt(m.itemCount, 10)
	m.itemCount++
	return idStr
}

func (m *Menu) AppendItem(items ...*MenuItem) {
	for _, item := range items {
		if item.Text != "" {
			item.Id = m.allocateId()
			m.Items = append(m.Items, item)
		}
	}
}

func (m *Menu) HandleAction(id string, timestamp uint32) error {
	for _, item := range m.Items {
		if id == item.Id {
			item.action(timestamp)
			return nil
		}
	}
	return errors.New("invalid item id")
}

func (m *Menu) GenerateJSON() string {
	bytes, err := json.Marshal(m)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return string(bytes)
}
