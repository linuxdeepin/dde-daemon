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

#include <time.h>
#include <unistd.h>
#include <crypt.h>
#include <shadow.h>
#include <pwd.h> 
#include <grp.h>
#include <string.h>
#include <stdlib.h>

#include "passwd.h"

#define ERROR_NULLPOINTER -1;
#define ERROR_NOERROR 0;

char *
mkpasswd (const char *words)
{
    unsigned long seed[2];
    char salt[] = "$6$........";
    const char *const seedchars =
        "./0123456789ABCDEFGHIJKLMNOPQRST"
        "UVWXYZabcdefghijklmnopqrstuvwxyz";
    char *password;
    int i;

    // Generate a (not very) random seed. You should do it better than this...
    seed[0] = time(NULL);
    seed[1] = getpid() ^ (seed[0] >> 14 & 0x30000);

    // Turn it into printable characters from `seedchars'.
    for (i = 0; i < 8; i++) {
        salt[3 + i] = seedchars[(seed[i / 5] >> (i % 5) * 6) & 0x3f];
    }

    // DES Encrypt
    password = crypt(words, salt);

    return password;
}

int
lock_shadow_file()
{
	return lckpwdf();
}

int
unlock_shadow_file()
{
	return ulckpwdf();
}

int 
exist_pw_uid(__uid_t uid)
{
    if (!getpwuid(uid)) {
        return ERROR_NULLPOINTER;
    }
    return ERROR_NOERROR;
}

char *
get_pw_name(__uid_t uid)
{
    return getpwuid(uid)->pw_name;
}

char *
get_pw_gecos(__uid_t uid)
{
    return getpwuid(uid)->pw_gecos;
}

__uid_t
get_pw_uid(__uid_t uid)
{
    return getpwuid(uid)->pw_uid;
}

__gid_t
get_pw_gid(__uid_t uid)
{
    return getpwuid(uid)->pw_gid;
}

char *
get_pw_dir(__uid_t uid)
{
    return getpwuid(uid)->pw_dir;
}

char *
get_pw_shell(__uid_t uid)
{
    return getpwuid(uid)->pw_shell;
}
