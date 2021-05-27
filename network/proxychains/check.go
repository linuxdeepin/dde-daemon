/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package proxychains

import (
	"net"
	"pkg.deepin.io/lib/keyfile"
	"regexp"
	"strings"
)

var ipv4Reg = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)

func validType(type0 string) bool {
	switch type0 {
	case "http", "socks4", "socks5":
		return true
	default:
		return false
	}
}

func validIPv4(ipStr string) bool {
	if !ipv4Reg.MatchString(ipStr) {
		return false
	}

	ip := net.ParseIP(ipStr)
	return ip != nil
}

func validUser(user string) bool {
	return !strings.ContainsAny(user, "\t ")
}

func validPassword(password string) bool {
	return validUser(password)
}

// dont collect experience message if edition is community
func isCommunity() bool {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile("/etc/os-version")
	// 为避免收集数据的风险，读不到此文件，或者Edition文件不存在也不收集数据
	if err != nil {
		return true
	}
	edition, err := kf.GetString("Version", "EditionName")
	if err != nil {
		return true
	}
	if edition == "Community" {
		return true
	}
	return false
}