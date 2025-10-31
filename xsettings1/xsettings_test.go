// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"testing"

	C "gopkg.in/check.v1"
)

type testWrapper struct{}

func init() {
	C.Suite(&testWrapper{})
}

func Test(t *testing.T) {
	C.TestingT(t)
}

var (
	xsTestDatas = []byte{0, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0, 0, 0, 15, 0, 78, 101, 116, 47, 68, 111, 117, 98, 108, 101, 67, 108, 105, 99, 107, 0, 0, 0, 0, 0, 5, 0, 0, 0, 1, 0, 13, 0, 78, 101, 116, 47, 84, 104, 101, 109, 101, 78, 97, 109, 101, 0, 0, 0, 0, 0, 0, 0, 6, 0, 0, 0, 68, 101, 101, 112, 105, 110, 0, 0, 2, 0, 15, 0, 78, 101, 116, 47, 83, 99, 104, 101, 109, 97, 67, 111, 108, 111, 114, 0, 0, 0, 0, 0, 0, 0, 128, 0, 255, 0, 100, 0}

	xsTestInfo = xsDataInfo{
		byteOrder:   xsDataOrder,
		serial:      xsDataSerial,
		numSettings: 3,
		items: xsItemInfos{
			{
				header: &xsItemHeader{
					sType:            settingTypeInteger,
					nameLen:          15,
					name:             "Net/DoubleClick",
					lastChangeSerial: 0,
				},
				value: &integerValueInfo{value: 5},
			},
			{
				header: &xsItemHeader{
					sType:            settingTypeString,
					nameLen:          13,
					name:             "Net/ThemeName",
					lastChangeSerial: 0,
				},
				value: &stringValueInfo{
					length: 6,
					value:  "Deepin",
				},
			},
			{
				header: &xsItemHeader{
					sType:            settingTypeColor,
					nameLen:          15,
					name:             "Net/SchemaColor",
					lastChangeSerial: 0,
				},
				value: &colorValueInfo{
					red:   0,
					green: 128,
					blue:  255,
					alpha: 100,
				},
			},
		},
	}
)

func (*testWrapper) TestXSWriter(c *C.C) {
	datas := marshalSettingData(&xsTestInfo)
	for i := 0; i < len(datas); i++ {
		c.Check(datas[i], C.Equals, xsTestDatas[i])
	}
}

func (*testWrapper) TestXSReader(c *C.C) {
	info := unmarshalSettingData(xsTestDatas)
	c.Check(info.byteOrder, C.Equals, xsTestInfo.byteOrder)
	c.Check(info.serial, C.Equals, xsTestInfo.serial)
	c.Check(info.numSettings, C.Equals, xsTestInfo.numSettings)
	for i := uint32(0); i < info.numSettings; i++ {
		c.Check(info.items[i].header.sType, C.Equals,
			xsTestInfo.items[i].header.sType)
		c.Check(info.items[i].header.nameLen, C.Equals,
			xsTestInfo.items[i].header.nameLen)
		c.Check(info.items[i].header.name, C.Equals,
			xsTestInfo.items[i].header.name)
		c.Check(info.items[i].header.lastChangeSerial, C.Equals,
			xsTestInfo.items[i].header.lastChangeSerial)
		switch info.items[i].header.sType {
		case settingTypeInteger:
			v1 := info.items[i].value.(*integerValueInfo)
			v2 := xsTestInfo.items[i].value.(*integerValueInfo)
			c.Check(v1.value, C.Equals, v2.value)
		case settingTypeString:
			v1 := info.items[i].value.(*stringValueInfo)
			v2 := xsTestInfo.items[i].value.(*stringValueInfo)
			c.Check(v1.length, C.Equals, v2.length)
			c.Check(v1.value, C.Equals, v2.value)
		case settingTypeColor:
			v1 := info.items[i].value.(*colorValueInfo)
			v2 := xsTestInfo.items[i].value.(*colorValueInfo)
			c.Check(v1.red, C.Equals, v2.red)
			c.Check(v1.green, C.Equals, v2.green)
			c.Check(v1.blue, C.Equals, v2.blue)
			c.Check(v1.alpha, C.Equals, v2.alpha)
		}
	}
}

func (*testWrapper) TestNewXSItemInteger(c *C.C) {
	var (
		prop        = "Net/DoubleClick"
		value int32 = 5
	)
	info := newXSItemInteger(prop, value)
	header := info.header
	c.Check(header.sType, C.Equals, settingTypeInteger)
	c.Check(header.nameLen, C.Equals, uint16(len(prop)))
	c.Check(header.name, C.Equals, prop)
	v1 := info.value.(*integerValueInfo)
	c.Check(v1.value, C.Equals, value)
}

func (*testWrapper) TestNewXSItemString(c *C.C) {
	var (
		prop  = "Net/ThemeName"
		value = "Deepin"
	)
	info := newXSItemString(prop, value)
	header := info.header
	c.Check(header.sType, C.Equals, settingTypeString)
	c.Check(header.nameLen, C.Equals, uint16(len(prop)))
	c.Check(header.name, C.Equals, prop)
	v1 := info.value.(*stringValueInfo)
	c.Check(v1.length, C.Equals, uint32(len(value)))
	c.Check(v1.value, C.Equals, value)
}

func (*testWrapper) TestNewXSItemColor(c *C.C) {
	var (
		prop  = "Net/SchemaColor"
		value = [4]uint16{255, 0, 128, 100}
	)

	info := newXSItemColor(prop, value)
	header := info.header
	c.Check(header.sType, C.Equals, settingTypeColor)
	c.Check(header.nameLen, C.Equals, uint16(len(prop)))
	c.Check(header.name, C.Equals, prop)
	v1 := info.value.(*colorValueInfo)
	c.Check(v1.red, C.Equals, value[0])
	c.Check(v1.green, C.Equals, value[1])
	c.Check(v1.blue, C.Equals, value[2])
	c.Check(v1.alpha, C.Equals, value[3])
}
