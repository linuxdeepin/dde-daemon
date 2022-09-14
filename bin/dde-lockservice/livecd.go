// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

//#include <stdlib.h>
//#include <string.h>
//#include <shadow.h>
//#include <crypt.h>
//#cgo LDFLAGS: -lcrypt
//#cgo CFLAGS: -W -Wall -fstack-protector-all -fPIC
//int is_livecd(const char *username)\
//{\
//    if (strcmp("deepin", username) != 0) {\
//        return 0;\
//    }\
//    struct spwd *data = getspnam(username);\
//    if (data == NULL || strlen(data->sp_pwdp) == 0) {\
//        return 0;\
//    }\
//    if (strcmp(crypt("", data->sp_pwdp), data->sp_pwdp) != 0) {\
//        return 0;\
//    }\
//    return 1;\
//}
import "C"
import "unsafe"

func isInLiveCD(username string) bool {
	cName := C.CString(username)
	ret := C.is_livecd(cName)
	C.free(unsafe.Pointer(cName))
	return (int(ret) == 1)
}
