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

package accounts

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"pkg.deepin.io/dde/daemon/accounts/users"
	"pkg.deepin.io/lib/dbusutil"
	dutils "pkg.deepin.io/lib/utils"
)

/*
  #cgo CFLAGS: -Wall -g
  #cgo LDFLAGS: -lcrypt

  #include <stdlib.h>
  #include <pwd.h>
  #include <grp.h>
*/
import "C"

type UserInfo struct {
	Name    string
	Uid     uint32
	Gid     uint32
	comment string
	Home    string
	Shell   string
}

// 根据对应账号的UID从password database中获取用户信息
func GetUserInfoByUID(uid uint32) (UserInfo, error) {
	pwent := C.getpwuid(C.uint(uid))
	if pwent == nil {
		return UserInfo{}, fmt.Errorf("Invalid uuid: %v", uid)
	}

	info := UserInfo{
		Name:  C.GoString(pwent.pw_name),
		Uid:   uint32(pwent.pw_uid),
		Gid:   uint32(pwent.pw_gid),
		Home:  C.GoString(pwent.pw_dir),
		Shell: C.GoString(pwent.pw_shell),
	}

	return info, nil
}

// 判断用户是否是网络账户，域账户信息只能由web端设置，本地没有保存
func IsDomainUserID(uid string) bool {
	id, _ := strconv.Atoi(uid)

	domainUserGroups, err := GetUserGroupsByUID(uint32(id))
	if err != nil {
		logger.Errorf("get domain user groups failed: %v", err)
		return false
	}

	for _, v := range domainUserGroups {
		if strings.Contains(v, "domain") {
			return true
		}
	}

	return false
}

// 获取当前账号的Group通过当前用户的uid
func GetUserGroupsByUID(uid uint32) ([]string, error) {
	var result []string

	pwent := C.getpwuid(C.uint(uid))
	if pwent == nil {
		return nil, fmt.Errorf("Invalid uuid: %d", uid)
	}

	gid := C.uint(pwent.pw_gid)
	groupPtr := C.getgrgid(gid)
	if groupPtr == nil {
		return nil, fmt.Errorf("Invalid gid: %d", gid)
	}

	result = append(result, C.GoString(groupPtr.gr_name))
	sort.Strings(result)

	return result, nil
}

func NewDomainUser(uid uint32, service *dbusutil.Service) (*User, error) {
	userInfo, err := GetUserInfoByUID(uid)

	if err != nil {
		return nil, err
	}

	var u = &User{
		service:            service,
		UserName:           userInfo.Name,
		Uid:                strconv.FormatUint(uint64(userInfo.Uid), 10),
		Gid:                strconv.FormatUint(uint64(userInfo.Gid), 10),
		HomeDir:            userInfo.Home,
		Shell:              userInfo.Shell,
		AutomaticLogin:     false,
		NoPasswdLogin:      false,
		Locked:             false,
		PasswordStatus:     users.PasswordStatusUsable,
		MaxPasswordAge:     30,
		PasswordLastChange: 18737,
	}

	u.AccountType = users.UserTypeNetwork
	u.Groups, err = GetUserGroupsByUID(userInfo.Uid)
	if err != nil {
		return nil, err
	}

	u.CreatedTime, err = u.getCreatedTime()
	if err != nil {
		logger.Warning("Failed to get created time:", err)
	}

	updateConfigPath(userInfo.Name)

	xSession, _ := users.GetDefaultXSession()
	u.XSession = xSession

	u.SystemAccount = false
	u.Layout = getDefaultLayout()
	u.Locale = getDefaultLocale()
	u.IconFile = defaultUserIcon
	defaultUserBackground := getDefaultUserBackground()
	u.DesktopBackgrounds = []string{defaultUserBackground}
	u.GreeterBackground = defaultUserBackground
	u.Use24HourFormat = defaultUse24HourFormat
	u.UUID = dutils.GenUuid()
	err = u.writeUserConfig()
	if err != nil {
		logger.Warning(err)
	}

	u.checkLeftSpace()
	return u, err
}
