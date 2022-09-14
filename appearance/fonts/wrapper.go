// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fonts

// #cgo pkg-config: fontconfig
// #cgo CFLAGS: -W -Wall -fstack-protector-all -fPIC
// #include <stdlib.h>
// #include "font_list.h"
import "C"

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unsafe"

	"github.com/linuxdeepin/go-lib/strv"
)

const (
	defaultLang      = "en"
	defaultLangDelim = "|"
	defaultNameDelim = ","
	spaceTypeMono    = "100"
)

var (
	curLang string
	home    = os.Getenv("HOME")
	langReg = regexp.MustCompile("_")
)

var familyBlacklist = strv.Strv([]string{
	// font family names of Deepin Open Symbol Fonts:
	"Symbol",
	"webdings",
	"MT Extra",
	"Wingdings",
	"Wingdings 2",
	"Wingdings 3",
})

// family ex: 'sans', 'serif', 'monospace'
// cRet: `SourceCodePro-Medium.otf: "Source Code Pro" "Medium"`
func fcFontMatch(family string) string {
	cFamily := C.CString(family)
	defer C.free(unsafe.Pointer(cFamily))
	cRet := C.font_match(cFamily)
	defer C.free(unsafe.Pointer(cRet))

	ret := C.GoString(cRet)
	if len(ret) == 0 {
		return ""
	}

	tmp := strings.Split(ret, ":")
	if len(tmp) != 2 {
		return ""
	}

	// return font family id
	name := strings.Split(tmp[1], "\"")[1]
	table := GetFamilyTable()
	key := sumStrHash(name)
	info, ok := table[key]
	if ok {
		return info.Id
	}

	for _, info := range table {
		if info.Name == name {
			return info.Id
		}
	}
	return name
}

func isFcCacheUpdate() bool {
	ret := C.fc_cache_update()
	return (ret == 1)
}

func GetFamilyTable() FamilyHashTable {
	var (
		table FamilyHashTable
		err   error
	)
	if !isFcCacheUpdate() {
		table = make(FamilyHashTable)
		err = loadCacheFromFile(familyHashCacheFile, &table)
		if err == nil {
			return table
		}
		fmt.Println("Failed to load families cache:", err)
	}

	table = fcInfosToFamilyTable()
	err = table.saveToFile()
	if err != nil {
		fmt.Println("Failed to save families cache:", err)
	}
	return table
}

func fcInfosToFamilyTable() FamilyHashTable {
	var table = make(FamilyHashTable)
	var num = C.int(0)
	list := C.list_font_info(&num)
	if num < 1 {
		return nil
	}
	defer C.free_font_info_list(list, num)

	itemSize := unsafe.Sizeof(*list)

	for i := C.int(0); i < num; i++ {
		cItem := (*C.FcInfo)(unsafe.Pointer(uintptr(unsafe.Pointer(list)) + uintptr(i)*itemSize))

		info := fcInfoToFamily(cItem)
		if info == nil {
			continue
		}

		key := sumStrHash(info.Id)
		_, ok := table[key]
		if !ok {
			table[key] = info
		}
	}
	return table
}

func fcInfoToFamily(cInfo *C.FcInfo) *Family {
	// var fullname = C.GoString(cInfo.fullname)
	var familyname = C.GoString(cInfo.family)
	if len(familyname) == 0 {
		return nil
	}
	// names := strings.Split(fullname, defaultNameDelim)
	// nameLang := strings.Split(C.GoString(cInfo.fullnamelang),
	// 	defaultNameDelim)
	families := strings.Split(familyname, defaultNameDelim)
	familyLang := strings.Split(C.GoString(cInfo.familylang),
		defaultNameDelim)
	family := getItemByIndex(indexOf(defaultLang, familyLang), families)
	if familyBlacklist.Contains(family) {
		return nil
	}

	// info.Deletable = isDeletable(info.File)
	langs := strings.Split(C.GoString(cInfo.lang), defaultLangDelim)
	return &Family{
		Id:        family,
		Name:      getItemByIndex(indexOf(getCurLang(), familyLang), families),
		Styles:    strings.Split(C.GoString(cInfo.style), defaultNameDelim),
		Monospace: isMonospace(family, C.GoString(cInfo.spacing)),
		Show:      strv.Strv(langs).Contains(getCurLang()),
	}
}

func isMonospace(name, spacing string) bool {
	if spacing == spaceTypeMono ||
		strings.Contains(strings.ToLower(name), "mono") {
		return true
	}

	return false
}

func getItemByIndex(idx int, list []string) string {
	if len(list) == 0 {
		return ""
	}

	if idx < 0 || len(list) <= idx {
		return list[0]
	}

	return list[idx]
}

func indexOf(item string, list []string) int {
	for i, v := range list {
		if item == v {
			return i
		}
	}
	return -1
}

func getCurLang() string {
	if len(curLang) != 0 {
		return curLang
	}

	locale := os.Getenv("LANGUAGE")
	if len(locale) == 0 {
		locale = os.Getenv("LANG")
	}

	lang := getLangFromLocale(locale)
	if len(lang) == 0 {
		return defaultLang
	}

	curLang = lang
	return lang
}

func getLangFromLocale(locale string) string {
	if len(locale) == 0 {
		return ""
	}

	locale = strings.ToLower(locale)
	if strings.Contains(locale, ".") {
		locale = strings.Split(locale, ".")[0]
	}
	if strings.Contains(locale, ":") {
		locale = strings.Split(locale, ":")[0]
	}
	var lang string
	switch locale {
	case "zh_hk":
		lang = "zh-tw"
	case "zh_cn", "zh_tw", "zh_sg", "ku_tr", "mn_mn", "pap_an", "pap_aw":
		lang = langReg.ReplaceAllString(locale, "-")
	default:
		lang = strings.Split(locale, "_")[0]
	}
	return lang
}
