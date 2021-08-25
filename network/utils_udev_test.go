/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
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

package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_udevGetDeviceVendor(t *testing.T) {
	tests := []struct {
		arg1 string
		want string
	}{
		{"", ""},
	}

	for _, e := range tests {
		get := udevGetDeviceVendor(e.arg1)
		assert.Equal(t, e.want, get)
	}
}

func Test_udevGetDeviceProduct(t *testing.T) {
	tests := []struct {
		arg1 string
		want string
	}{
		{"", ""},
	}

	for _, e := range tests {
		get := udevGetDeviceProduct(e.arg1)
		assert.Equal(t, e.want, get)
	}
}

func Test_udevGetDeviceDesc(t *testing.T) {
	tests := []struct {
		arg1 string
		want string
		ok   bool
	}{
		{"", "", false},
	}

	for _, e := range tests {
		get, ok := udevGetDeviceDesc(e.arg1)
		assert.Equal(t, e.ok, ok)
		if ok {
			assert.Equal(t, e.want, get)
		}
	}
}

func Test_udevIsUsbDevice(t *testing.T) {
	tests := []struct {
		arg1 string
		want bool
	}{
		{"", false},
	}

	for _, e := range tests {
		get := udevIsUsbDevice(e.arg1)
		assert.Equal(t, e.want, get)
	}
}

func Test_fixupDeviceDesc(t *testing.T) {
	tests := []struct {
		arg1 string
		want string
	}{
		{"Intel Corporation Ethernet Connection (12) I219-V",
			"Intel Ethernet Connection (12) I219-V"},
		{"Shenzhen Longsys Electronics Co., Ltd. Device 1160_200",
			"Shenzhen Longsys Electronics Device 1160 200"},
	}

	for _, e := range tests {
		get := fixupDeviceDesc(e.arg1)
		assert.Equal(t, e.want, get)
	}
}
