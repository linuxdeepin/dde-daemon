// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#ifndef __PASSWORD_H__
#define __PASSWORD_H__

char *_mkpasswd(const char *words);

int _lock_shadow_file();
int _unlock_shadow_file();

int _exist_pw_uid(__uid_t uid);

char *_get_pw_name(__uid_t uid);
char *_get_pw_gecos(__uid_t uid);
__uid_t _get_pw_uid(__uid_t uid);
__gid_t _get_pw_gid(__uid_t uid);
char *_get_pw_dir(__uid_t uid);
char *_get_pw_shell(__uid_t uid);

#endif
