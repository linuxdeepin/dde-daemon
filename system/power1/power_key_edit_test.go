// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
