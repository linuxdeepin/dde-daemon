// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	settingTypeInteger uint8 = iota
	settingTypeString
	settingTypeColor
)

var (
	defaultByteOrder = binary.LittleEndian
)

type stringValueInfo struct {
	length uint32
	value  string
}

type integerValueInfo struct {
	value int32
}

type colorValueInfo struct {
	red   uint16
	green uint16
	blue  uint16
	//If the setting does not need the alpha field,
	//it should be set to 65535.
	alpha uint16
}

type xsItemHeader struct {
	sType            uint8  // setting type
	nameLen          uint16 // name length
	name             string
	lastChangeSerial uint32
}

type xsItemInfo struct {
	header *xsItemHeader
	value  interface{}
}

type xsItemInfos []xsItemInfo

type xsDataInfo struct {
	byteOrder   uint8
	serial      uint32
	numSettings uint32

	items xsItemInfos
}

func (infos xsItemInfos) listProps() string {
	var content = "["
	for i, info := range infos {
		if i != 0 {
			content += ","
		}
		content += fmt.Sprintf("%q", info.header.name)
	}
	return content + "]"
}

func (info *xsDataInfo) getPropItem(prop string) *xsItemInfo {
	for _, item := range info.items {
		if prop == item.header.name {
			return &item
		}
	}

	return nil
}

func unmarshalSettingData(data []byte) *xsDataInfo {
	var info xsDataInfo
	if len(data) == 0 {
		info.byteOrder = xsDataOrder
		info.numSettings = 0
		info.serial = xsDataSerial
		return &info
	}

	var reader = bytes.NewReader(data)

	readInteger(reader, &info.byteOrder)
	readSkip(reader, 3)
	readInteger(reader, &info.serial)
	readInteger(reader, &info.numSettings)
	for i := 0; i < int(info.numSettings); i++ {
		var item = xsItemInfo{
			header: &xsItemHeader{},
		}
		readXSItemInfo(reader, &item)
		info.items = append(info.items, item)
	}

	return &info
}

func readSkip(reader io.Reader, num int) {
	var buf = make([]byte, num)
	err := binary.Read(reader, defaultByteOrder, &buf)
	if err != nil {
		logger.Warning(err)
	}
}

func readInteger(reader io.Reader, v interface{}) {
	err := binary.Read(reader, defaultByteOrder, v)
	if err != nil {
		logger.Warning(err)
	}
}

func readString(reader io.Reader, v *string, length int) {
	var buf = make([]byte, length)
	err := binary.Read(reader, defaultByteOrder, &buf)
	if err != nil {
		logger.Warning(err)
	}
	*v = string(buf)
}

func readXSItemInfo(reader io.Reader, item *xsItemInfo) {
	readXSItemHeader(reader, item.header)

	switch item.header.sType {
	case settingTypeInteger:
		var v = integerValueInfo{}
		readXSValueInteger(reader, &v)
		item.value = &v
	case settingTypeString:
		var v = stringValueInfo{}
		readXSValueString(reader, &v)
		item.value = &v
	case settingTypeColor:
		var v = colorValueInfo{}
		readXSValueColor(reader, &v)
		item.value = &v
	}
}

func readXSItemHeader(reader io.Reader, header *xsItemHeader) {
	readInteger(reader, &header.sType)
	readSkip(reader, 1)
	readInteger(reader, &header.nameLen)
	readString(reader, &header.name, int(header.nameLen))
	readSkip(reader, pad(int(header.nameLen)))
	readInteger(reader, &header.lastChangeSerial)
}

func readXSValueInteger(reader io.Reader, v *integerValueInfo) {
	readInteger(reader, &v.value)
}

func readXSValueString(reader io.Reader, v *stringValueInfo) {
	readInteger(reader, &v.length)
	readString(reader, &v.value, int(v.length))
	readSkip(reader, pad(int(v.length)))
}

func readXSValueColor(reader io.Reader, v *colorValueInfo) {
	readInteger(reader, &v.red)
	readInteger(reader, &v.green)
	readInteger(reader, &v.blue)
	readInteger(reader, &v.alpha)
}
