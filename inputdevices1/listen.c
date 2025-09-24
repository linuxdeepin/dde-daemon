// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#include <stdio.h>
#include <stdlib.h>
#include <pthread.h>

#include <X11/Xlib.h>
#include <X11/extensions/XInput2.h>

#include "listen.h"
#include "_cgo_export.h"

static int has_xi2();
static void *listen_device_thread(void *user_data);

static Display *_disp = NULL;
static pthread_t _thrd;
static int _thrd_exit_flag = 0;

int
start_device_listener()
{
    _thrd_exit_flag = 0;

    int xi_opcode = has_xi2();
    if (xi_opcode == -1) {
        _thrd_exit_flag = 1;
        return -1;
    }

    pthread_attr_t attr;
    pthread_attr_init(&attr);
    // 设置为 joinable，方便主线程等待退出
    pthread_attr_setdetachstate(&attr, PTHREAD_CREATE_JOINABLE);
    int ret = pthread_create(&_thrd, &attr,
                             listen_device_thread, (void*)&xi_opcode);
    pthread_attr_destroy(&attr);

    if (ret != 0 ) {
        fprintf(stderr, "Create device event listen thread failed\n");
        _thrd_exit_flag = 1;
        return -1;
    }

    // 主线程等待子线程主动退出
    pthread_join(_thrd, NULL);

    return 0;
}

void
end_device_listener()
{
    // 设置退出标志，通知子线程退出
    _thrd_exit_flag = 1;
}

static int
has_xi2()
{
    Display *disp = XOpenDisplay(0);
    if (!disp) {
        fprintf(stderr, "Open Display Failed in has_xi2\n");
        return -1;
    }

    int xi_opcode, event, error;
    if (!XQueryExtension(disp, "XInputExtension",
                         &xi_opcode, &event, &error)) {
        fprintf(stderr, "XInput extension not available.\n");
        XCloseDisplay(disp);
        return -1;
    }

    // We support XI 2.0
    int major = 2;
    int minor = 0;

    int rc =XIQueryVersion(disp, &major, &minor);
    if ( rc == BadRequest) {
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

static void*
listen_device_thread(void *user_data)
{
    /*int xi_opcode = *(int*)user_data;*/
    XIEventMask mask;

    mask.deviceid = XIAllDevices;
    mask.mask_len = XIMaskLen(XI_LASTEVENT);
    mask.mask = calloc(mask.mask_len, sizeof(char));

    XISetMask(mask.mask, XI_HierarchyChanged);

    _disp = XOpenDisplay(0);
    if (!_disp) {
        _thrd_exit_flag = 1;
        pthread_exit(NULL);
        return NULL;
    }

    XISelectEvents(_disp, DefaultRootWindow(_disp), &mask, 1);
    XSync(_disp, False);

    free(mask.mask);

    while (!_thrd_exit_flag) {
        XEvent ev;
        XGenericEventCookie *cookie = (XGenericEventCookie*)&ev.xcookie;
        // 使用 XPending 检查事件队列，避免阻塞在 XNextEvent
        if (XPending(_disp) > 0) {
            XNextEvent(_disp, (XEvent*)&ev);

            if (cookie->type != GenericEvent ||
                !XGetEventData(_disp, cookie)) {
                continue;
            }

            if (cookie->evtype == XI_HierarchyChanged) {
                XIHierarchyEvent *event = cookie->data;
                if (event->flags & XIMasterAdded ||
                    event->flags & XISlaveAdded ) {
                    handleDeviceChanged();
                } else if (event->flags & XIMasterRemoved ||
                           event->flags &XISlaveRemoved ) {
                    handleDeviceChanged();
                }
            }
            XFreeEventData(_disp, cookie);
        } else {
            // 没有事件时休眠，避免 CPU 占用
            struct timespec ts = {0, 10000000}; // 10ms
            nanosleep(&ts, NULL);
        }
    }

    _thrd_exit_flag = 1;
    XCloseDisplay(_disp);
    pthread_exit(NULL);

    return NULL;
}
