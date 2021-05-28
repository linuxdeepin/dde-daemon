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

#ifndef __PASSWORD_H__
#define __PASSWORD_H__

char *mkpasswd(const char *words);

int lock_shadow_file();
int unlock_shadow_file();

int exist_pw_uid(__uid_t uid);

char *
get_pw_name(__uid_t uid);
char *
get_pw_gecos(__uid_t uid);
__uid_t
get_pw_uid(__uid_t uid);
__gid_t
get_pw_gid(__uid_t uid);
char *
get_pw_dir(__uid_t uid);
char *
get_pw_shell(__uid_t uid);

#endif
