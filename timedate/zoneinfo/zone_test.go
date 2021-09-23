/*
 * Copyright (C) 2013 ~ 2018 Deepin Technology Co., Ltd.
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

package zoneinfo

import (
	"os"
	"path"
	"testing"

	C "gopkg.in/check.v1"
	dutils "pkg.deepin.io/lib/utils"
)

type testWrapper struct{}

func init() {
	C.Suite(&testWrapper{})
}

func Test(t *testing.T) {
	C.TestingT(t)
}

func (*testWrapper) TestGetZoneList(c *C.C) {
	var ret = []string{
		"Europe/Andorra",
		"Asia/Dubai",
		"Asia/Kabul",
		"Europe/Tirane",
		"Asia/Yerevan",
	}

	list, err := getZoneListFromFile("testdata/zone1970.tab")
	c.Check(err, C.Equals, nil)
	for i := range list {
		c.Check(list[i], C.Equals, ret[i])
	}
}

func (*testWrapper) TestZoneValid(c *C.C) {
	zoneFile := path.Join(defaultZoneDir, "Asia/Shanghai")
	if !dutils.IsFileExist(zoneFile) {
		c.Skip("file not exist")
	}

	var infos = []struct {
		zone  string
		valid bool
		err   error
	}{
		{
			zone:  "Asia/Shanghai",
			valid: true,
			err:   nil,
		},
		//{
		//zone:  "Asia/Beijing",
		//valid: true,
		//},
		{
			zone:  "Asia/xxxx",
			valid: false,
			err:   nil,
		},
	}

	for _, info := range infos {
		valid, err := IsZoneValid(info.zone)
		c.Check(valid, C.Equals, info.valid)
		c.Check(err, C.Equals, info.err)
	}
}

var zoneInfos = []ZoneInfo{
	{
		"Europe/Andorra",
		"Andorra",
		3600,
		DSTInfo{1585443600, 1603587599, 7200},
	},
	{
		"Asia/Dubai",
		"Dubai",
		14400,
		DSTInfo{0, 0, 0},
	},
	{
		"Asia/Kabul",
		"Kabul",
		16200,
		DSTInfo{0, 0, 0},
	},
	{
		"Europe/Tirane",
		"Tirane",
		3600,
		DSTInfo{1585443600, 1603587599, 7200},
	},
	{
		"Asia/Yerevan",
		"Yerevan",
		14400,
		DSTInfo{0, 0, 0},
	},
}

func (*testWrapper) TestGetDSTTime(c *C.C) {
	lang := os.Getenv("LANGUAGE")
	_ = os.Setenv("LANGUAGE", "en_US")
	defer func() {
		_ = os.Setenv("LANGUAGE", lang)
	}()

	for _, info := range zoneInfos {
		first, second, ok := getDSTTime(info.Name, 2020) // 固定计算2020年夏令时相关时间
		c.Check(first, C.Equals, info.DST.Enter)
		c.Check(second, C.Equals, info.DST.Leave)
		if first == 0 || second == 0 {
			c.Check(ok, C.Equals, false)
		} else {
			c.Check(ok, C.Equals, true)
		}
	}
}

func (*testWrapper) TestGetRawUSec(c *C.C) {
	lang := os.Getenv("LANGUAGE")
	_ = os.Setenv("LANGUAGE", "en_US")
	defer func() {
		_ = os.Setenv("LANGUAGE", lang)
	}()

	for _, info := range zoneInfos {
		enter := getRawUSec(info.Name, info.DST.Enter)
		c.Check(enter+1, C.Equals, info.DST.Enter)
	}
}

func (*testWrapper) TestGetOffsetByUSec(c *C.C) {
	lang := os.Getenv("LANGUAGE")
	_ = os.Setenv("LANGUAGE", "en_US")
	defer func() {
		_ = os.Setenv("LANGUAGE", lang)
	}()

	for _, info := range zoneInfos {
		offset := getOffsetByUSec(info.Name, info.DST.Enter)
		if info.DST.Enter != 0 {
			c.Check(offset, C.Equals, info.DST.Offset)
		} else {
			c.Check(offset, C.Equals, info.Offset)
		}

	}
}

func (*testWrapper) TestGetZoneInfo(c *C.C) {
	lang := os.Getenv("LANGUAGE")
	_ = os.Setenv("LANGUAGE", "en_US")
	defer func() {
		_ = os.Setenv("LANGUAGE", lang)
	}()
	for _, info := range zoneInfos {
		dstInfo := newDSTInfo(info.Name)
		if dstInfo != nil {
			info.DST.Enter = dstInfo.Enter
			info.DST.Leave = dstInfo.Leave
			info.DST.Offset = dstInfo.Offset
		} else {
			info.Offset = getOffsetByUSec(info.Name, 0)
		}
		zoneInfo, err := GetZoneInfo(info.Name)
		c.Check(err, C.Equals, nil)
		c.Check(*zoneInfo, C.Equals, info)
	}
}
