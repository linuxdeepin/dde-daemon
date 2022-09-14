// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package timedate

import (
	C "gopkg.in/check.v1"
	"testing"
)

type testWrapper struct{}

func init() {
	C.Suite(&testWrapper{})
}

func Test(t *testing.T) {
	C.TestingT(t)
}

var list = []string{
	"home",
	"hello",
	"world",
	"goodbye",
}

func (*testWrapper) TestItemInList(c *C.C) {
	var infos = []struct {
		name  string
		exist bool
	}{
		{
			name:  "hello",
			exist: true,
		},
		{
			name:  "goodbye",
			exist: true,
		},
		{
			name:  "helloxxx",
			exist: false,
		},
	}

	for _, info := range infos {
		c.Check(isItemInList(info.name, list), C.Equals, info.exist)
	}
}

func (*testWrapper) TestAddItem(c *C.C) {
	var infos = []struct {
		name   string
		length int
		added  bool
	}{
		{
			name:   "hello",
			length: 4,
			added:  false,
		},
		{
			name:   "helloxxx",
			length: 5,
			added:  true,
		},
	}

	for _, info := range infos {
		tList, added := addItemToList(info.name, list)
		c.Check(len(tList), C.Equals, info.length)
		c.Check(added, C.Equals, info.added)
		c.Check(isItemInList(info.name, tList), C.Equals, true)
	}
}

func (*testWrapper) TestDeleteItem(c *C.C) {
	var infos = []struct {
		name    string
		length  int
		deleted bool
	}{
		{
			name:    "hello",
			length:  3,
			deleted: true,
		},
		{
			name:    "helloxxx",
			length:  4,
			deleted: false,
		},
	}

	for _, info := range infos {
		tList, deleted := deleteItemFromList(info.name, list)
		c.Check(len(tList), C.Equals, info.length)
		c.Check(deleted, C.Equals, info.deleted)
		c.Check(isItemInList(info.name, tList), C.Equals, false)
	}
}

func (*testWrapper) TestFilterNilStr(c *C.C) {
	var infos = []struct {
		list   []string
		hasNil bool
		ret    []string
	}{
		{
			list:   []string{"abs", "apt", "", "pacman"},
			hasNil: true,
			ret:    []string{"abs", "apt", "pacman"},
		},
		{
			list:   []string{"c", "go", "python"},
			hasNil: false,
			ret:    []string{"c", "go", "python"},
		},
	}

	for _, info := range infos {
		list, hasNil := filterNilString(info.list)
		c.Check(hasNil, C.Equals, info.hasNil)
		c.Check(len(list), C.Equals, len(info.ret))
	}
}
