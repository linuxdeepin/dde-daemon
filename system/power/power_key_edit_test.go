/*
 * Copyright (C) 2022 ~ 2022 Deepin Technology Co., Ltd.
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

package power

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_interfaceToArrayString(t *testing.T) {
	data := []struct {
		test   interface{}
		result []string
	}{
		{[]interface{}{}, []string{}},
		{[]interface{}{"0"}, []string{"0"}},
		{[]interface{}{"0", "1"}, []string{"0", "1"}},
		{[]interface{}{"0", "1", "2"}, []string{"0", "1", "2"}},
		{[]interface{}{"0", "1", "2", "3"}, []string{"0", "1", "2", "3"}},
	}
	for _, d := range data {
		trans := interfaceToArrayString(d.test)
		arr := make([]string, len(trans))
		for i, v := range trans {
			arr[i] = v.(string)
		}

		assert.Equal(t, arr, d.result)
	}
}

func Test_interfaceToString(t *testing.T) {
	data := []struct {
		test   interface{}
		result string
	}{
		{"", ""},
		{"test", "test"},
		{"System Power Test", "System Power Test"},
	}
	for _, d := range data {
		assert.Equal(t, interfaceToString(d.test), d.result)
	}
}

func Test_interfaceToBool(t *testing.T) {
	data := []struct {
		test   interface{}
		result bool
	}{
		{true, true},
		{false, false},
	}
	for _, d := range data {
		assert.Equal(t, interfaceToBool(d.test), d.result)
	}
}