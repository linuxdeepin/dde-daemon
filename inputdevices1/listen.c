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
static void my_io_error_exit_handler(Display *dpy, void *user_data);

static Display *_disp = NULL;
static pthread_t _thrd;
static int _thrd_exit_flag = 0;

static void my_io_error_exit_handler(Display *dpy, void *user_data)
{
    fprintf(stderr, "X IO Error: Connection to X server lost, exiting thread gracefully\n");
    _thrd_exit_flag = 1;
    
    // 清理 Display 连接（虽然已经断开，但释放资源）
    if (_disp) {
        XCloseDisplay(_disp);
        _disp = NULL;
    }
    
    // XSetIOErrorExitHandler 的处理器通常不会返回到调用点
    // 所以我们需要在这里直接退出线程
    pthread_exit(NULL);
}

int
start_device_listener()
{
    _thrd_exit_flag = 0;

    int xi_opcode = has_xi2();
    if (xi_opcode == -1) {
        _thrd_exit_flag = 1;
        return -1;
    }

    int ret = pthread_create(&_thrd, NULL, listen_device_thread, (void*)&xi_opcode);
    if (ret != 0 ) {
        fprintf(stderr, "Create device event listen thread failed\n");
        _thrd_exit_flag = 1;
        return -1;
    }

    return 0;
}

void
end_device_listener()
{
    _thrd_exit_flag = 1;
    
    // 取消阻塞在 XNextEvent 的线程
    pthread_cancel(_thrd);
    pthread_join(_thrd, NULL);
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

    /* 设置 IO 错误退出处理器，防止程序崩溃 */
    XSetIOErrorExitHandler(_disp, my_io_error_exit_handler, NULL);

    XISelectEvents(_disp, DefaultRootWindow(_disp), &mask, 1);
    XSync(_disp, False);

    free(mask.mask);
    
    while (!_thrd_exit_flag) {
        XEvent ev;
        XGenericEventCookie *cookie = (XGenericEventCookie*)&ev.xcookie;
        XNextEvent(_disp, (XEvent*)&ev);
        
        if (cookie->type != GenericEvent ||
            /*cookie->extension != xi_opcode ||*/
            !XGetEventData(_disp, cookie)) {
            continue;
        }

        if (cookie->evtype == XI_HierarchyChanged) {
            XIHierarchyEvent *event = cookie->data;
            if (event->flags & XIMasterAdded ||
                event->flags & XISlaveAdded ) {
                /* int deviceid = event->info->deviceid; */
                /*printf("Device Added: %d\n", deviceid);*/
                handleDeviceChanged();
            } else if (event->flags & XIMasterRemoved ||
                       event->flags &XISlaveRemoved ) {
                /* int deviceid = event->info->deviceid; */
                /*printf("Device Removed: %d\n", deviceid);*/
                handleDeviceChanged();
            }
        }
        XFreeEventData(_disp, cookie);
    }

    _thrd_exit_flag = 1;
    XCloseDisplay(_disp);
    pthread_exit(NULL);

    return NULL;
}
