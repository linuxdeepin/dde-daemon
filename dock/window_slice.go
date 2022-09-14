// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	x "github.com/linuxdeepin/go-x11-client"
)

type windowSlice []x.Window

func (a windowSlice) Len() int           { return len(a) }
func (a windowSlice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a windowSlice) Less(i, j int) bool { return uint32(a[i]) < uint32(a[j]) }

func (winSlice windowSlice) Contains(win x.Window) bool {
	for _, window := range winSlice {
		if window == win {
			return true
		}
	}
	return false
}

// from a to b
// return [add, remove]
func diffSortedWindowSlice(a, b windowSlice) (add, remove windowSlice) {
	ia := 0
	ib := 0
	lenA := len(a)
	lenB := len(b)

	for ia < lenA && ib < lenB {
		va := uint32(a[ia])
		vb := uint32(b[ib])
		if va == vb {
			ia++
			ib++
		} else if va < vb {
			// remove
			remove = append(remove, a[ia])
			ia++
		} else {
			// va > vb
			// add
			add = append(add, b[ib])
			ib++
		}
	}

	for ia < lenA {
		remove = append(remove, a[ia])
		ia++
	}

	for ib < lenB {
		add = append(add, b[ib])
		ib++
	}
	return
}
