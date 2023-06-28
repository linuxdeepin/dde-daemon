// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fonts

import (
	"crypto/md5"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

type Family struct {
	Id   string
	Name string

	Styles []string

	Monospace bool
	Show      bool
}

type FamilyHashTable map[string]*Family

const (
	fallbackStandard  = "Noto Sans"
	fallbackMonospace = "Noto Mono"

	xsettingsSchema = "com.deepin.xsettings"
	gsKeyFontName   = "gtk-font-name"
)

var (
	locker    sync.Mutex
	xsSetting = gio.NewSettings(xsettingsSchema)

	DeepinFontConfig = path.Join(basedir.GetUserConfigDir(), "fontconfig", "conf.d", "99-deepin.conf")
)

func Reset() error {
	err := removeAll(DeepinFontConfig)
	if err != nil {
		return err
    }
	xsSetting.Reset(gsKeyFontName)
	return nil
}

func IsFontFamily(value string) bool {
	if isVirtualFont(value) {
		return true
	}

	info := GetFamilyTable().GetFamily(value)
	return info != nil
}

func IsFontSizeValid(size float64) bool {
	if size >= 7.0 && size <= 22.0 {
		return true
	}
	return false
}

func SetFamily(standard, monospace string, size float64) error {
	locker.Lock()
	defer locker.Unlock()

	if isVirtualFont(standard) {
		standard = fcFontMatch(standard)
	}
	if isVirtualFont(monospace) {
		monospace = fcFontMatch(monospace)
	}

	table := GetFamilyTable()
	standInfo := table.GetFamily(standard)
	if standInfo == nil {
		return fmt.Errorf("Invalid standard id '%s'", standard)
	}
	// standard += " " + standInfo.preferredStyle()
	monoInfo := table.GetFamily(monospace)
	if monoInfo == nil {
		return fmt.Errorf("Invalid monospace id '%s'", monospace)
	}
	// monospace += " " + monoInfo.preferredStyle()

	// fc-match can not real time update
	/*
		curStand := fcFontMatch("sans-serif")
		curMono := fcFontMatch("monospace")
		if (standInfo.Id == curStand || standInfo.Name == curStand) &&
			(monoInfo.Id == curMono || monoInfo.Name == curMono) {
			return nil
		}
	*/

	err := writeFontConfig(configContent(standInfo.Id, monoInfo.Id), DeepinFontConfig)
	if err != nil {
		return err
	}
	return setFontByXSettings(standard, size)
}

func GetFontSize() float64 {
	return getFontSize(xsSetting)
}

func (table FamilyHashTable) ListMonospace() []string {
	var ids []string
	for _, info := range table {
		if !info.Monospace {
			continue
		}
		ids = append(ids, info.Id)
	}
	sort.Strings(ids)
	return ids
}

func (table FamilyHashTable) ListStandard() []string {
	var ids []string
	for _, info := range table {
		if info.Monospace || !info.Show {
			continue
		}
		ids = append(ids, info.Id)
	}
	sort.Strings(ids)
	return ids
}

func (table FamilyHashTable) Get(key string) *Family {
	info := table[key]
	return info
}

func (table FamilyHashTable) GetFamily(id string) *Family {
	info, ok := table[sumStrHash(id)]
	if !ok {
		return nil
	}
	return info
}

func (table FamilyHashTable) GetFamilies(ids []string) []*Family {
	var infos []*Family
	for _, id := range ids {
		info, ok := table[sumStrHash(id)]
		if !ok {
			continue
		}
		infos = append(infos, info)
	}
	return infos
}

func setFontByXSettings(name string, size float64) error {
	if size == -1 {
		size = getFontSize(xsSetting)
	}
	v := fmt.Sprintf("%s %v", name, size)
	if v == xsSetting.GetString(gsKeyFontName) {
		return nil
	}

	xsSetting.SetString(gsKeyFontName, v)
	return nil
}

func getFontSize(setting *gio.Settings) float64 {
	value := setting.GetString(gsKeyFontName)
	if len(value) == 0 {
		return 0
	}

	array := strings.Split(value, " ")
	size, _ := strconv.ParseFloat(array[len(array)-1], 64)
	return size
}

func isVirtualFont(name string) bool {
	switch name {
	case "monospace", "mono", "sans-serif", "sans", "serif":
		return true
	}
	return false
}

func sumStrHash(v string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(v)))
}
