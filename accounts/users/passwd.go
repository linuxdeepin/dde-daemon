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

package users

/*
#cgo CFLAGS: -Wall -g
#cgo LDFLAGS: -lcrypt

#include <stdlib.h>
#include <shadow.h>
#include "passwd.h"
*/
import "C"

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

var (
	wLocker sync.Mutex
)

func EncodePasswd(words string) string {
	cwords := C.CString(words)
	defer C.free(unsafe.Pointer(cwords))

	return C.GoString(C.mkpasswd(cwords))
}

func ExistPwUid(uid uint32) int {
	return int(C.exist_pw_uid(C.uint(uid)))
}

func GetPwName(uid uint32) string {
	return C.GoString(C.get_pw_name(C.uint(uid)))
}

func GetPwGecos(uid uint32) string {
	var pwGecos string
	pwGecos = C.GoString(C.get_pw_gecos(C.uint(uid)))

	return strings.Split(pwGecos, ",")[0]
}

func GetPwUid(uid uint32) string {
	return strconv.FormatUint(uint64(C.get_pw_uid(C.uint(uid))), 10)
}

func GetPwGid(uid uint32) string {
	return strconv.FormatUint(uint64(C.get_pw_gid(C.uint(uid))), 10)
}

func GetPwDir(uid uint32) string {
	return C.GoString(C.get_pw_dir(C.uint(uid)))
}

func GetPwShell(uid uint32) string {
	return C.GoString(C.get_pw_shell(C.uint(uid)))
}

type originShadowInfo struct {
	Name       string
	LastChange int
	MaxDays    int
	ShadowPwdp string // password status
}

func getSpwd(username string) (*originShadowInfo, error) {
	cname := C.CString(username)
	defer C.free(unsafe.Pointer(cname))
	spwd := C.getspnam(cname)
	if spwd == nil {
		return &originShadowInfo{}, fmt.Errorf("no such user name: %v", username)
	}
	sInfo := originShadowInfo{
		Name:       C.GoString(spwd.sp_namp),
		LastChange: int(spwd.sp_lstchg),
		MaxDays:    int(spwd.sp_max),
		ShadowPwdp: C.GoString(spwd.sp_pwdp),
	}
	return &sInfo, nil
}

// password: has been crypt
func updatePasswd(password, username string) error {
	status := C.lock_shadow_file()
	if status != 0 {
		return fmt.Errorf("Lock shadow file failed")
	}
	defer C.unlock_shadow_file()

	content, err := ioutil.ReadFile(userFileShadow)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	var datas []string
	var found bool
	for _, line := range lines {
		if len(line) == 0 {
			datas = append(datas, line)
			continue
		}

		items := strings.Split(line, ":")
		if items[0] != username {
			datas = append(datas, line)
			continue
		}

		found = true
		if items[1] == password {
			return nil
		}

		var tmp string
		for i, v := range items {
			if i != 0 {
				tmp += ":"
			}

			if i == 1 {
				tmp += password
				continue
			}

			tmp += v
		}

		datas = append(datas, tmp)
	}

	if !found {
		return fmt.Errorf("The username not exist.")
	}

	err = writeStrvToFile(datas, userFileShadow, math.MaxUint32)
	if err != nil {
		return nil
	}

	usr, _ := user.Lookup("root")
	grp, _ := user.LookupGroup("shadow")
	uid, _ := strconv.Atoi(usr.Uid)
	gid, _ := strconv.Atoi(grp.Gid)
	err = os.Chown(userFileShadow, uid, gid)

	return err
}

func writeStrvToFile(datas []string, file string, mode os.FileMode) error {
	var content string
	for i, v := range datas {
		if i != 0 {
			content += "\n"
		}

		content += v
	}

	f, err := os.Create(file + ".bak~")
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	wLocker.Lock()
	defer wLocker.Unlock()
	_, err = f.WriteString(content)
	if err != nil {
		return err
	}

	err = f.Sync()
	if err != nil {
		return err
	}

	info, err := os.Stat(file)
	if err == nil && mode == math.MaxUint32 {
		mode = info.Mode()
	}

	os.Rename(file+".bak~", file)
	os.Chmod(file, mode)

	return nil
}
