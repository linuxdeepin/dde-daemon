// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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

#ifndef CRYPT_GENSALT_OUTPUT_SIZE
#define CRYPT_GENSALT_OUTPUT_SIZE 192
#endif

typedef struct {
    const char *name;
    const char *prefix;
} PasswordAlgorithm;

static const PasswordAlgorithm SUPPORTED_ALGORITHMS[] = {
    {"sm3",      "$sm3$"},
    {"yescrypt", "$y$"},
    {"sha512",   "$6$"},
    {"sha256",   "$5$"},
    {NULL,       NULL}
};

#define DEFAULT_ALGORITHM 0

static char *try_encrypt_password(const char *words, const char *prefix) {
    char output[CRYPT_GENSALT_OUTPUT_SIZE];
    char *setting;
    char *password;

    setting = crypt_gensalt_rn(prefix, 0, NULL, 0, output, sizeof(output));
    if (setting == NULL || setting[0] == '*') {
        return NULL;
    }

    password = crypt(words, setting);
    if (password == NULL || password[0] == '*') {
        return NULL;
    }

    return password;
}

char *mkpasswd_with_algo(const char *words, const char *algo) {
    char *password;
    int i;
    const PasswordAlgorithm *selected_algo = NULL;

    if (algo != NULL && algo[0] != '\0') {
        for (i = 0; SUPPORTED_ALGORITHMS[i].name != NULL; i++) {
            if (strcmp(algo, SUPPORTED_ALGORITHMS[i].name) == 0) {
                selected_algo = &SUPPORTED_ALGORITHMS[i];
                break;
            }
        }
    }

    if (selected_algo == NULL) {
        selected_algo = &SUPPORTED_ALGORITHMS[DEFAULT_ALGORITHM];
    }

    password = try_encrypt_password(words, selected_algo->prefix);
    if (password != NULL) {
        return password;
    }

    for (i = 0; SUPPORTED_ALGORITHMS[i].name != NULL; i++) {
        if (&SUPPORTED_ALGORITHMS[i] == selected_algo) {
            continue;
        }

        password = try_encrypt_password(words, SUPPORTED_ALGORITHMS[i].prefix);
        if (password != NULL) {
            return password;
        }
    }

    return NULL;
}

int lock_shadow_file() {
    return lckpwdf();
}

int unlock_shadow_file() {
    return ulckpwdf();
}

int exist_pw_uid(__uid_t uid) {
    if (!getpwuid(uid)) {
        return ERROR_NULLPOINTER;
    }
    return ERROR_NOERROR;
}

char *get_pw_name(__uid_t uid) {
    return getpwuid(uid)->pw_name;
}

char *get_pw_gecos(__uid_t uid) {
    return getpwuid(uid)->pw_gecos;
}

__uid_t get_pw_uid(__uid_t uid) {
    return getpwuid(uid)->pw_uid;
}

__gid_t get_pw_gid(__uid_t uid) {
    return getpwuid(uid)->pw_gid;
}

char *get_pw_dir(__uid_t uid) {
    return getpwuid(uid)->pw_dir;
}

char *get_pw_shell(__uid_t uid) {
    return getpwuid(uid)->pw_shell;
}
