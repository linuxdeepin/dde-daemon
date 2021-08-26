/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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

func Test_calcCheckSum(t *testing.T) {
	var data []byte
	calcCheckSum(data)

	data = []byte{
		0x1, 0x2, 0x3,
	}
	sum := calcCheckSum(data)
	assert.Equal(t, uint16(0xfefa), sum)

	data = []byte{
		0x1, 0x2, 0x3, 0x4,
	}
	sum = calcCheckSum(data)
	assert.Equal(t, uint16(0xfbf9), sum)
}

func Test_getSequenceNum(t *testing.T) {
	num := getSequenceNum()
	assert.Equal(t, uint16(1), num)
}

func Test_unmarshalIPHeader(t *testing.T) {
	var datas []byte
	header, err := unmarshalIPHeader(datas)
	assert.Nil(t, header)
	assert.NotNil(t, err)

	datas = []byte{
		0xF1, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10,
		0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10,
		0x10,
	}

	header, err = unmarshalIPHeader(datas)
	assert.Equal(t, uint8(0x10), header.Flag)
	assert.Nil(t, err)
}
