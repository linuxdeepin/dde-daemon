// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#ifndef COMMON_H
#define COMMON_H

#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <libintl.h>
#include <locale.h>
#include <pthread.h>
#include <pwd.h>
#include <security/_pam_types.h>
#include <security/pam_ext.h>
#include <security/pam_modules.h>
#include <signal.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <syslog.h>
#include <systemd/sd-bus.h>
#include <termios.h>
#include <unistd.h>
#include <sys/time.h>

#define MASTER_KEY_LEN 16
#define FILE_PATH_BUF_SIZE 128
#define MAX_BUF_SIZE 256
#define LIMITS_BUF_SIZE 1024

#define ENCRYPT_MODE 1
#define DECRYPT_MODE 0

void *dalloc(size_t size);
char *generate_random_str();
char *generate_random_len(unsigned int length);
unsigned char* deepin_wb_encrypt(unsigned char* IN, unsigned char* key, bool flag);
unsigned char* sm4_crypt(unsigned char* IN, unsigned char* key, int mode);
void printState(char *out, unsigned char * in);
void set_debug_flag(int state);

#endif