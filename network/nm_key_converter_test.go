// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package network

import (
	C "gopkg.in/check.v1"
)

func init() {
	C.Suite(&testWrapper{})
}
func (*testWrapper) TestInterfaceToString(c *C.C) {
	data := []struct {
		test   interface{}
		result string
	}{
		{"", ""},
		{"Network Connection", "Network Connection"},
	}
	for _, d := range data {
		c.Check(interfaceToString(d.test), C.Equals, d.result)
	}
}
func (*testWrapper) TestInterfaceToByte(c *C.C) {
	data := []struct {
		test   interface{}
		result byte
	}{
		{byte(0), 0},
		{byte(0x0b), 0x0b},
	}
	for _, d := range data {
		c.Check(interfaceToByte(d.test), C.Equals, d.result)
	}
}
func (*testWrapper) TestInterfaceToInt32(c *C.C) {
	data := []struct {
		test   interface{}
		result int32
	}{
		{int32(0), 0},
		{int32(-2147483648), -2147483648},
		{int32(2147483647), 2147483647},
	}
	for _, d := range data {
		c.Check(interfaceToInt32(d.test), C.Equals, d.result)
	}
}
func (*testWrapper) TestInterfaceToUint32(c *C.C) {
	data := []struct {
		test   interface{}
		result uint32
	}{
		{uint32(0), 0},
		{uint32(4294967295), 4294967295},
	}
	for _, d := range data {
		c.Check(interfaceToUint32(d.test), C.Equals, d.result)
	}
}
func (*testWrapper) TestInterfaceToInt64(c *C.C) {
	data := []struct {
		test   interface{}
		result int64
	}{
		{int64(0), 0},
		{int64(-9223372036854775808), -9223372036854775808},
		{int64(9223372036854775807), 9223372036854775807},
	}
	for _, d := range data {
		c.Check(interfaceToInt64(d.test), C.Equals, d.result)
	}
}
func (*testWrapper) TestInterfaceToUint64(c *C.C) {
	data := []struct {
		test   interface{}
		result uint64
	}{
		{uint64(0), 0},
		{uint64(18446744073709551615), 18446744073709551615},
	}
	for _, d := range data {
		c.Check(interfaceToUint64(d.test), C.Equals, d.result)
	}
}
func (*testWrapper) TestInterfaceToBoolean(c *C.C) {
	data := []struct {
		test   interface{}
		result bool
	}{
		{true, true},
		{false, false},
	}
	for _, d := range data {
		c.Check(interfaceToBoolean(d.test), C.Equals, d.result)
	}
}
func (*testWrapper) TestInterfaceToArrayByte(c *C.C) {
	data := []struct {
		test   interface{}
		result []byte
	}{
		{[]byte{}, []byte{}},
		{[]byte{0}, []byte{0}},
		{[]byte{0, 1, 2, 3}, []byte{0, 1, 2, 3}},
	}
	for _, d := range data {
		c.Check(interfaceToArrayByte(d.test), C.DeepEquals, d.result)
	}
}
func (*testWrapper) TestInterfaceToArrayString(c *C.C) {
	data := []struct {
		test   interface{}
		result []string
	}{
		{[]string{}, []string{}},
		{[]string{"0"}, []string{"0"}},
		{[]string{"0", "1", "2", "3"}, []string{"0", "1", "2", "3"}},
	}
	for _, d := range data {
		c.Check(interfaceToArrayString(d.test), C.DeepEquals, d.result)
	}
}
func (*testWrapper) TestInterfaceToArrayUint32(c *C.C) {
	data := []struct {
		test   interface{}
		result []uint32
	}{
		{[]uint32{}, []uint32{}},
		{[]uint32{0}, []uint32{0}},
		{[]uint32{0, 1111, 2222, 3333}, []uint32{0, 1111, 2222, 3333}},
	}
	for _, d := range data {
		c.Check(interfaceToArrayUint32(d.test), C.DeepEquals, d.result)
	}
}