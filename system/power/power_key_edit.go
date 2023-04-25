// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"github.com/linuxdeepin/go-lib/utils"
)

func interfaceToArrayString(v interface{}) (d []interface{}) {
	if utils.IsInterfaceNil(v) {
		return
	}

	d, ok := v.([]interface{})
	if !ok {
		logger.Errorf("interfaceToArrayString() failed: %#v", v)
		return
	}
	return
}

func interfaceToString(v interface{}) (d string) {
	if utils.IsInterfaceNil(v) {
		return
	}
	d, ok := v.(string)
	if !ok {
		logger.Errorf("interfaceToString() failed: %#v", v)
		return
	}
	return
}

func interfaceToBool(v interface{}) (d bool) {
	if utils.IsInterfaceNil(v) {
		return
	}
	d, ok := v.(bool)
	if !ok {
		logger.Errorf("interfaceToBool() failed: %#v", v)
		return
	}
	return
}
