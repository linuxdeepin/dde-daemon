// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

//#cgo pkg-config: x11
//#cgo CFLAGS: -fstack-protector-strong -D_FORTITY_SOURCE=1 -fPIC
//#include <stdio.h>
//#include <string.h>
//#include <stdlib.h>
//#include <X11/Xlib.h>
//#include <X11/Xatom.h>
//
//char *
//get_xresources()
//{
//    Display *dpy = XOpenDisplay(NULL);
//    if (!dpy) {
//        return NULL;
//    }
//
//    char *res = XResourceManagerString(dpy);
//    if (!res) {
//        XCloseDisplay(dpy);
//        fprintf(stderr, "No xresources data found!\n");
//        return strdup("*customization:\t-color\n");
//    }
//
//    char *ret = strdup(res);
//    XCloseDisplay(dpy);
//    return ret;
//}
//
//int
//set_xresources(char *data, unsigned long length)
//{
//    Display *dpy = XOpenDisplay(NULL);
//    if (!dpy) {
//        return -1;
//    }
//
//    XChangeProperty(dpy, DefaultRootWindow(dpy), XA_RESOURCE_MANAGER, XA_STRING, 8, PropModeReplace,
//                    (const unsigned char*)data, length);
//    XCloseDisplay(dpy);
//    return 0;
//}
import "C"

import (
	"fmt"
	"strings"
	"unsafe"
)

type xresourceInfo struct {
	key   string
	value string
}
type xresourceInfos []*xresourceInfo

func updateXResources(changes xresourceInfos) {
	var infos xresourceInfos
	res := C.get_xresources()
	data := C.GoString(res)
	C.free(unsafe.Pointer(res))
	if len(data) == 0 {
		logger.Debug("--------No xresource found, created")
		infos = append(infos, &xresourceInfo{
			key:   "*customization",
			value: "-color",
		})
		infos = append(infos, changes...)
	} else {
		logger.Debug("------------Info from read:", data)
		infos = unmarshalXResources(data)
		for _, v := range changes {
			logger.Debug("-----updateXResources info:", v.key, v.value)
			infos = infos.UpdateProperty(v.key, v.value)
		}
	}
	data = marshalXResources(infos)
	logger.Debug("[updateXResources] will set to:", data)
	res = C.CString(data)
	defer C.free(unsafe.Pointer(res))
	ret := C.set_xresources(res, C.ulong(len(data)))
	if ret != C.int(0) {
		logger.Error("Failed to set xresource:", data)
	}
}

func (infos xresourceInfos) UpdateProperty(key, value string) xresourceInfos {
	info := infos.Get(key)
	if info == nil {
		infos = append(infos, &xresourceInfo{
			key:   key,
			value: value,
		})
		return infos
	}

	info.value = value
	return infos
}

func (infos xresourceInfos) Get(key string) *xresourceInfo {
	for _, info := range infos {
		if info.key == key {
			return info
		}
	}
	return nil
}

func marshalXResources(infos xresourceInfos) string {
	var data string
	for _, info := range infos {
		data += info.key + ":\t" + info.value + "\n"
	}
	return data
}

func unmarshalXResources(data string) xresourceInfos {
	lines := strings.Split(data, "\n")
	var infos xresourceInfos
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		array := strings.Split(line, ":\t")
		if len(array) != 2 {
			fmt.Println("Array:", array)
			continue
		}

		infos = append(infos, &xresourceInfo{
			key:   array[0],
			value: array[1],
		})
	}
	return infos
}
