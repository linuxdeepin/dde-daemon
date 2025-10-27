// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"errors"

	"github.com/linuxdeepin/dde-daemon/common/dconfig"
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

func (m *Manager) setDsgData(key string, value interface{}, dsg *dconfig.DConfig) error {
	if dsg == nil {
		return errors.New("setDsgData dsg is nil")
	}
	err := dsg.SetValue(key, value)
	if err != nil {
		logger.Warningf("setDsgData key : %s. err : %v", key, err)
		return err
	}
	logger.Debugf("setDsgData key : %s , value : %v", key, value)
	return nil
}
