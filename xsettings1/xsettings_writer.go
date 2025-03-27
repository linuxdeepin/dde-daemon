// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"bytes"
	"encoding/binary"
	"io"
)

func (info *xsDataInfo) modifyProperty(setting xsSetting) xsItemInfos {
	var (
		tmp   xsItemInfos
		items = info.items
	)

	for _, item := range items {
		if item.header.name == setting.prop {
			var ptr = &item
			item.header.lastChangeSerial++
			ptr.changePropValue(setting.value)
		}
		tmp = append(tmp, item)
	}

	return tmp
}

func (item *xsItemInfo) changePropValue(value interface{}) {
	switch item.header.sType {
	case settingTypeInteger:
		item.changeValueInteger(value.(int32))
	case settingTypeString:
		item.changeValueString(value.(string))
	case settingTypeColor:
		item.changeValueColor(value.([4]uint16))
	}
}

func (item *xsItemInfo) changeValueInteger(value int32) {
	v, ok := item.value.(*integerValueInfo)
	if !ok || v.value == value {
		return
	}

	v.value = value
}

func (item *xsItemInfo) changeValueString(value string) {
	v, ok := item.value.(*stringValueInfo)
	if !ok || v.value == value {
		return
	}

	v.length = uint32(len(value))
	v.value = value
}

func (item *xsItemInfo) changeValueColor(value [4]uint16) {
	v, ok := item.value.(*colorValueInfo)
	if !ok || (v.red == value[0] && v.green == value[1] &&
		v.blue == value[2] && v.alpha == value[3]) {
		return
	}

	v.red = value[0]
	v.green = value[1]
	v.blue = value[2]
	v.alpha = value[3]
}

func marshalSettingData(info *xsDataInfo) []byte {
	var buf = new(bytes.Buffer)

	writeInteger(buf, &info.byteOrder)
	writeSkip(buf, 3)
	writeInteger(buf, &info.serial)
	writeInteger(buf, &info.numSettings)
	for _, item := range info.items {
		writeXSItemInfo(buf, &item)
	}

	return buf.Bytes()
}

func newXSItemInteger(prop string, v int32) *xsItemInfo {
	var item = xsItemInfo{
		header: newXSItemHeader(prop),
		value: &integerValueInfo{
			value: v,
		},
	}

	item.header.sType = settingTypeInteger
	return &item
}

func newXSItemString(prop string, v string) *xsItemInfo {
	var item = xsItemInfo{
		header: newXSItemHeader(prop),
	}
	item.header.sType = settingTypeString

	var value = stringValueInfo{
		length: uint32(len(v)),
		value:  v,
	}

	item.value = &value
	return &item
}

func newXSItemColor(prop string, v [4]uint16) *xsItemInfo {
	var item = xsItemInfo{
		header: newXSItemHeader(prop),
	}
	item.header.sType = settingTypeColor

	var value = colorValueInfo{
		red:   v[0],
		green: v[1],
		blue:  v[2],
		alpha: v[3],
	}

	item.value = &value
	return &item
}

func newXSItemHeader(prop string) *xsItemHeader {
	var header = xsItemHeader{
		nameLen:          uint16(len(prop)),
		name:             prop,
		lastChangeSerial: 1,
	}
	return &header
}

func writeSkip(writer io.Writer, num int) {
	var buf = make([]byte, num)
	err := binary.Write(writer, defaultByteOrder, buf)
	if err != nil {
		logger.Warning(err)
	}
}

func writeInteger(writer io.Writer, v interface{}) {
	err := binary.Write(writer, defaultByteOrder, v)
	if err != nil {
		logger.Warning(err)
	}
}

func writeString(writer io.Writer, v string) {
	err := binary.Write(writer, defaultByteOrder, []byte(v))
	if err != nil {
		logger.Warning(err)
	}
}

func writeXSItemInfo(writer io.Writer, item *xsItemInfo) {
	writeXSInfoHeader(writer, item.header)

	switch item.header.sType {
	case settingTypeInteger:
		writeXSValueInteger(writer, item.value.(*integerValueInfo))
	case settingTypeString:
		writeXSValueString(writer, item.value.(*stringValueInfo))
	case settingTypeColor:
		writeXSValueColor(writer, item.value.(*colorValueInfo))
	}
}

func writeXSInfoHeader(writer io.Writer, header *xsItemHeader) {
	writeInteger(writer, &header.sType)
	writeSkip(writer, 1)
	writeInteger(writer, &header.nameLen)
	writeString(writer, header.name)
	writeSkip(writer, pad(int(header.nameLen)))
	writeInteger(writer, &header.lastChangeSerial)
}

func writeXSValueInteger(writer io.Writer, v *integerValueInfo) {
	writeInteger(writer, &v.value)
}

func writeXSValueString(writer io.Writer, v *stringValueInfo) {
	writeInteger(writer, &v.length)
	writeString(writer, v.value)
	writeSkip(writer, pad(int(v.length)))
}

func writeXSValueColor(writer io.Writer, v *colorValueInfo) {
	writeInteger(writer, &v.red)
	writeInteger(writer, &v.green)
	writeInteger(writer, &v.blue)
	writeInteger(writer, &v.alpha)
}
