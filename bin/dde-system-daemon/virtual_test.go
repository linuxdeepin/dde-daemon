/*
 * Copyright (C) 2020 ~ 2020 Deepin Technology Co., Ltd.
 *
 * Author:     wubowen <wubowen@uniontech.com>
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
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var testVirtuals []string = []string{"hvm", "virtual", "vmware", "kvm", "cloud", "invented"}

func TestGetValidSupData(t *testing.T) {
	t.Run("Test file get Valid Support Data check", func(t *testing.T) {
		assert.Equal(t, []string{"invented", "cloud", "kvm", "vmware", "virtual", "bochs", "hvm"}, getValidSupData([]string{"hvm", "bochs", "virtual", "vmware", "kvm", "cloud", "invented", ""}))
	})
}

func Test_isVirtual(t *testing.T) {
	t.Run("Test file isVirtual check", func(t *testing.T) {
		assert.Equal(t, true, isVirtual("/opt/apps/com.huawei.fusionaccessclient/hdpclienthwcloud", testVirtuals))
		assert.Equal(t, true, isVirtual("/opt/appshvm", testVirtuals))
		assert.Equal(t, true, isVirtual("/opt/apps/com.huawei.fusionaccessclient/virtual", testVirtuals))
		assert.Equal(t, true, isVirtual("/opt/apps/vmware", testVirtuals))
		assert.Equal(t, true, isVirtual("/opt/apps/invented", testVirtuals))
		assert.Equal(t, true, isVirtual("/opt/apps/kvm", testVirtuals))
		assert.Equal(t, false, isVirtual("/test/data", testVirtuals))
	})
}
