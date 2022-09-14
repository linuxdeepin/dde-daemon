// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#include <stdio.h>
#include <errno.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/stat.h>
#include <libudev.h>

#include "utils.h"

static int open_restricted(const char *path, int flags, void *user_data);
static void close_restricted(int fd, void *user_data);

static const struct libinput_interface li_ifc = {
    .open_restricted = open_restricted,
    .close_restricted = close_restricted,
};

struct libinput*
open_from_udev(char *seat, void *user_data, int verbose)
{
    struct udev *udev = udev_new();
    if (!udev) {
        fprintf(stderr, "Failed to initialize udev\n");
        return NULL;
    }

    struct libinput *li = libinput_udev_create_context(&li_ifc, user_data, udev);
    if (!li) {
        fprintf(stderr, "Failed to initialize context from udev\n");
        udev_unref(udev);
        return NULL;
    }

    if (verbose) {
        // TODO: add log handler
        libinput_log_set_priority(li, LIBINPUT_LOG_PRIORITY_DEBUG);
    }

    if (!seat) {
        seat = "seat0";
    }
    if (libinput_udev_assign_seat(li, seat)) {
        fprintf(stderr, "Failed to set seat\n");
        libinput_unref(li);
        udev_unref(udev);
        return NULL;
    }
    return li;
}

struct libinput*
open_from_path(char **path, void *user_data, int verbose)
{
    if (!path) {
        fprintf(stderr, "Device path empty\n");
        return NULL;
    }

    struct libinput *li = libinput_path_create_context(&li_ifc, user_data);
    if (!li) {
        fprintf(stderr, "Failed to initialize context\n");
        return NULL;
    }

    if (verbose) {
        // TODO: add log handler
        libinput_log_set_priority(li, LIBINPUT_LOG_PRIORITY_DEBUG);
    }

    int i = 0;
    for (; path[i] != NULL; i++) {
        struct libinput_device *dev = libinput_path_add_device(li, path[i]);
        if (!dev) {
            fprintf(stderr, "Failed to initialize device from %s\n", path[i]);
            libinput_unref(li);
            return NULL;
        }
    }
    return li;
}

static int
open_restricted(const char *path, int flags, void *user_data)
{
    int fd = open(path, flags);
    if (fd < 0) {
        fprintf(stderr, "Failed to open '%s': %s\n", path, strerror(errno));
        return -errno;
    }
    return fd;
}

static void
close_restricted(int fd, void *user_data)
{
    close(fd);
}
