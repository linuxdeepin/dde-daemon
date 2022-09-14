// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	x "github.com/linuxdeepin/go-x11-client"
)

type ExportWindowInfo struct {
	Title string
	Flash bool
}

type windowInfosType map[x.Window]ExportWindowInfo

func newWindowInfos() windowInfosType {
	return make(windowInfosType)
}

func (a windowInfosType) Equal(b windowInfosType) bool {
	if len(a) != len(b) {
		return false
	}
	for keyA, valA := range a {
		valB, okB := b[keyA]
		if okB {
			if valA != valB {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
