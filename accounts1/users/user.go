// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package users

/*
#include <unistd.h>
#include <limits.h>

#ifndef LOGIN_NAME_MAX
#define LOGIN_NAME_MAX 256
#endif

long get_login_name_max() {
    long conf = -1;
    conf = sysconf(_SC_LOGIN_NAME_MAX);

    if (conf == -1) {
        conf = LOGIN_NAME_MAX;
    }

    return conf;
}
*/
import "C"

var loginNameMaxSize int

func init() {
    loginNameMaxSize = int(C.get_login_name_max())
}

func LoginNameMaxSize() int {
    return loginNameMaxSize
}
