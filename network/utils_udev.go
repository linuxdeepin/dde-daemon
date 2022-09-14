// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package network

// #cgo pkg-config: libudev
// #cgo CFLAGS: -W -Wall -fstack-protector-all -fPIC
// #include <stdlib.h>
// #include "utils_udev.h"
import "C"
import (
	"strings"
	"unsafe"
)

var deviceDescIgnoredWords = []string{
	"Semiconductor",
	"Components",
	"Corporation",
	"Communications",
	"Company",
	"Corp.",
	"Corp",
	"Co.",
	"Inc.",
	"Inc",
	"Incorporated",
	"Ltd.",
	"Limited.",
	"Intel?",
	"chipset",
	"adapter",
	"[hex]",
	"NDIS",
	"Module",
	"Technology",
	"(Motherboard)",
	"Fast",
}
var deviceDescIgnoredPhrases = []string{
	"Multiprotocol MAC/baseband processor",
	"Wireless LAN Controller",
	"Wireless LAN Adapter",
	"Wireless Adapter",
	"Network Connection",
	"Wireless Cardbus Adapter",
	"Wireless CardBus Adapter",
	"54 Mbps Wireless PC Card",
	"Wireless PC Card",
	"Wireless PC",
	"PC Card with XJACK(r) Antenna",
	"Wireless cardbus",
	"Wireless LAN PC Card",
	"Technology Group Ltd.",
	"Communication S.p.A.",
	"Business Mobile Networks BV",
	"Mobile Broadband Minicard Composite Device",
	"Mobile Communications AB",
	"(PC-Suite Mode)",
	"PCI Express",
	"Ethernet Controller",
	"Ethernet Adapter",
	"(Industrial Computer Source / ICS Advent)",
}

func udevGetDeviceDesc(syspath string) (desc string, ok bool) {
	vendor := fixupDeviceDesc(udevGetDeviceVendor(syspath))
	product := fixupDeviceDesc(udevGetDeviceProduct(syspath))
	if len(vendor) == 0 && len(product) == 0 {
		return "", false
	}

	// If all of the fixed up vendor string is found in product,
	// ignore the vendor.
	if strings.Contains(vendor, product) {
		desc = vendor
	} else {
		desc = vendor + " " + product
	}
	return desc, true
}

func udevGetDeviceVendor(syspath string) (vendor string) {
	cSyspath := C.CString(syspath)
	defer C.free(unsafe.Pointer(cSyspath))

	cVendor := C.get_device_vendor(cSyspath)
	defer C.free(unsafe.Pointer(cVendor))
	vendor = C.GoString(cVendor)
	return
}

func udevGetDeviceProduct(syspath string) (product string) {
	cSyspath := C.CString(syspath)
	defer C.free(unsafe.Pointer(cSyspath))

	cVendor := C.get_device_product(cSyspath)
	defer C.free(unsafe.Pointer(cVendor))
	product = C.GoString(cVendor)
	return
}

func udevIsUsbDevice(syspath string) bool {
	cSyspath := C.CString(syspath)
	defer C.free(unsafe.Pointer(cSyspath))

	ret := C.is_usb_device(cSyspath)
	return ret == 0
}

// fixupDeviceDesc attempt to shorten description by ignoring certain
// phrases and individual words, such as "Corporation", "Inc".
func fixupDeviceDesc(desc string) (fixedDesc string) {
	desc = strings.Replace(desc, "_", " ", -1)
	desc = strings.Replace(desc, ",", " ", -1)

	for _, phrase := range deviceDescIgnoredPhrases {
		desc = strings.Replace(desc, phrase, "", -1)
	}

	words := strings.Split(desc, " ")
	for _, w := range words {
		if len(w) > 0 && !isStringInArray(w, deviceDescIgnoredWords) {
			if len(fixedDesc) == 0 {
				fixedDesc = w
			} else {
				fixedDesc = fixedDesc + " " + w
			}
		}
	}
	return
}
