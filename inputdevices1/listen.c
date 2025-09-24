// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

#include <stdio.h>
#include <stdlib.h>
#include <pthread.h>
#include <setjmp.h>
#include <unistd.h>     // usleep
#include <signal.h>

#include <X11/Xlib.h>
#include <X11/extensions/XInput2.h>

#include "listen.h"
#include "_cgo_export.h"

static int has_xi2();
static void *listen_device_thread(void *user_data);

static pthread_t _thrd;
static volatile int _thrd_exit_flag = 0;

/* 线程本地 jumpbuf，用于处理 X IO 错误 */
static __thread sigjmp_buf io_err_jmp;
static __thread int io_err_jmp_ready = 0;

/* 保存旧的 process-wide IOErrorHandler */
static int (*old_io_error_handler)(Display *) = NULL;

/* fallback 默认 Xlib exit handler */
extern void _XDefaultIOErrorExit(Display *dpy);

static int my_io_error_handler(Display *dpy)
{
    if (io_err_jmp_ready) {
        io_err_jmp_ready = 0;
        siglongjmp(io_err_jmp, 1);
    }
    if (old_io_error_handler && old_io_error_handler != my_io_error_handler) {
        return old_io_error_handler(dpy);
    }
    _XDefaultIOErrorExit(dpy);
    return 0;
}

int start_device_listener()
{
    _thrd_exit_flag = 0;

    int xi_opcode = has_xi2();
    if (xi_opcode == -1) {
        _thrd_exit_flag = 1;
        return -1;
    }

    int *p_xi = malloc(sizeof(int));
    if (!p_xi) {
        fprintf(stderr, "malloc failed\n");
        return -1;
    }
    *p_xi = xi_opcode;

    int ret = pthread_create(&_thrd, NULL, listen_device_thread, (void*)p_xi);
    if (ret != 0) {
        fprintf(stderr, "Create device event listen thread failed\n");
        _thrd_exit_flag = 1;
        free(p_xi);
        return -1;
    }

    return 0;
}

void end_device_listener()
{
    _thrd_exit_flag = 1;
    pthread_join(_thrd, NULL);
}

static int has_xi2()
{
    Display *disp = XOpenDisplay(NULL);
    if (!disp) {
        fprintf(stderr, "Open Display Failed in has_xi2\n");
        return -1;
    }

    int xi_opcode, event, error;
    if (!XQueryExtension(disp, "XInputExtension", &xi_opcode, &event, &error)) {
        fprintf(stderr, "XInput extension not available.\n");
        XCloseDisplay(disp);
        return -1;
    }

    int major = 2, minor = 0;
    int rc = XIQueryVersion(disp, &major, &minor);
    if (rc == BadRequest) {
        fprintf(stderr, "No XI2 Support.\n");
        XCloseDisplay(disp);
        return -1;
    } else if (rc != Success) {
        fprintf(stderr, "Internal Error.\n");
        XCloseDisplay(disp);
        return -1;
    }

    XCloseDisplay(disp);
    return xi_opcode;
}

static void* listen_device_thread(void *user_data)
{
    int xi_opcode = *(int*)user_data;
    free(user_data);

    Display *disp = XOpenDisplay(NULL);
    if (!disp) {
        fprintf(stderr, "listen_device_thread: XOpenDisplay failed\n");
        _thrd_exit_flag = 1;
        return NULL;
    }

    /* 安装线程局部 IOErrorHandler */
    old_io_error_handler = XSetIOErrorHandler(my_io_error_handler);

    if (sigsetjmp(io_err_jmp, 1) != 0) {
        fprintf(stderr, "listen_device_thread: caught X IO error\n");
        io_err_jmp_ready = 0;
        goto cleanup;
    }
    io_err_jmp_ready = 1;

    XIEventMask mask;
    mask.deviceid = XIAllDevices;
    mask.mask_len = XIMaskLen(XI_LASTEVENT);
    mask.mask = calloc(mask.mask_len, sizeof(char));
    if (!mask.mask) {
        fprintf(stderr, "calloc failed for mask\n");
        goto cleanup;
    }
    XISetMask(mask.mask, XI_HierarchyChanged);

    XISelectEvents(disp, DefaultRootWindow(disp), &mask, 1);
    XSync(disp, False);
    free(mask.mask);

    while (!_thrd_exit_flag) {
        while (XPending(disp) > 0) {
            XEvent ev;
            XGenericEventCookie *cookie = (XGenericEventCookie*)&ev.xcookie;

            XNextEvent(disp, &ev);

            if (cookie->type != GenericEvent ||
                cookie->extension != xi_opcode ||
                !XGetEventData(disp, cookie)) {
                continue;
            }

            if (cookie->evtype == XI_HierarchyChanged) {
                XIHierarchyEvent *event = cookie->data;
                if (event->flags & (XIMasterAdded | XISlaveAdded |
                                    XIMasterRemoved | XISlaveRemoved)) {
                    handleDeviceChanged();
                }
            }
            XFreeEventData(disp, cookie);
        }
    }

cleanup:
    io_err_jmp_ready = 0;

    if (disp) {
        XCloseDisplay(disp);
        disp = NULL;
    }

    if (old_io_error_handler) {
        XSetIOErrorHandler(old_io_error_handler);
        old_io_error_handler = NULL;
    }

    _thrd_exit_flag = 1;
    return NULL;
}
