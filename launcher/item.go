/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package launcher

import (
	"fmt"
	"strings"

	"github.com/linuxdeepin/go-dbus-factory/com.deepin.api.pinyin"
	"pkg.deepin.io/lib/appinfo/desktopappinfo"
)

type SearchScore uint64

type Item struct {
	Path          string
	Name          string // display name
	enName        string
	ID            string
	Icon          string
	CategoryID    CategoryID
	TimeInstalled int64

	keywords        []string
	categories      []string
	xDeepinCategory string
	exec            string
	genericName     string
	comment         string
	searchTargets   map[string]SearchScore
}

func (item *Item) String() string {
	if item == nil {
		return "<nil>"
	}
	return fmt.Sprintf("<item %v>", item.ID)
}

const (
	desktopExt = ".desktop"
)

func NewItemWithDesktopAppInfo(appInfo *desktopappinfo.DesktopAppInfo) *Item {
	enName, _ := appInfo.GetString(desktopappinfo.MainSection, desktopappinfo.KeyName)
	enComment, _ := appInfo.GetString(desktopappinfo.MainSection, desktopappinfo.KeyComment)
	xDeepinCategory, _ := appInfo.GetString(desktopappinfo.MainSection, "X-Deepin-Category")
	xDeepinVendor, _ := appInfo.GetString(desktopappinfo.MainSection, "X-Deepin-Vendor")

	var name string
	if xDeepinVendor == "deepin" {
		name, _ = appInfo.GetLocaleString(desktopappinfo.MainSection,
			desktopappinfo.KeyGenericName, "")
		if name == "" {
			name = appInfo.GetName()
		}
	} else {
		name = appInfo.GetName()
	}

	if name == "" {
		name = appInfo.GetId()
	}

	item := &Item{
		Path:            appInfo.GetFileName(),
		Name:            name,
		enName:          enName,
		Icon:            appInfo.GetIcon(),
		exec:            appInfo.GetCommandline(),
		genericName:     appInfo.GetGenericName(),
		comment:         enComment,
		searchTargets:   make(map[string]SearchScore),
		xDeepinCategory: strings.ToLower(xDeepinCategory),
	}
	for _, kw := range appInfo.GetKeywords() {
		item.keywords = append(item.keywords, strings.ToLower(kw))
	}

	categories := appInfo.GetCategories()
	for _, c := range categories {
		item.categories = append(item.categories, strings.ToLower(c))
	}
	return item
}

func (item *Item) getXCategory() CategoryID {
	logger.Debug("getXCategory item.categories:", item.categories)
	categoriesCountMap := make(map[CategoryID]int)
	if len(item.categories) == 1 {
		return parseXCategoryString(item.categories[0])
	}

	for _, categoryStr := range item.categories {
		cid := parseXCategoryString(categoryStr)
		categoriesCountMap[cid] = categoriesCountMap[cid] + 1
	}

	// ignore CategoryOthers
	delete(categoriesCountMap, CategoryOthers)
	logger.Debug("getXCategory categoriesCountMap:", categoriesCountMap)

	if len(categoriesCountMap) > 0 {
		var categoryCountMax int
		categoryMax := CategoryOthers
		for cid, count := range categoriesCountMap {
			if count > categoryCountMax {
				categoryCountMax = count
				categoryMax = cid
			}
		}
		logger.Debugf("category max %v count %v", categoryMax, categoryCountMax)
		return categoryMax
	}
	return CategoryOthers
}

const (
	idScore          = 100
	nameScore        = 80
	genericNameScore = 70
	keywordScore     = 60
	categoryScore    = 60
	commentScore     = 50
)

func (item *Item) setSearchTargets(pinyin *pinyin.Pinyin) {
	item.addSearchTarget(idScore, item.ID)
	item.addSearchTarget(nameScore, item.Name)
	item.addSearchTarget(nameScore, item.enName)
	item.addSearchTarget(genericNameScore, item.genericName)
	item.addSearchTarget(categoryScore, item.CategoryID.String())
	item.addSearchTarget(commentScore, item.comment)
	for _, c := range item.categories {
		item.addSearchTarget(categoryScore, c)
	}
	if pinyin != nil {
		pinyinList, err := pinyin.Query(0, item.Name)
		if err == nil {
			for _, v := range pinyinList {
				item.addSearchTarget(nameScore, v)
			}
		} else {
			logger.Warning(err)
		}

		item.addSearchTarget(categoryScore, item.CategoryID.Pinyin())
	}

	// add keywords
	for _, kw := range item.keywords {
		item.addSearchTarget(keywordScore, kw)
	}
}

func (item *Item) addSearchTarget(score SearchScore, str string) {
	if str == "" {
		return
	}
	str = strings.ToLower(str)
	scoreInDict, ok := item.searchTargets[str]
	if !ok || (ok && scoreInDict < score) {
		item.searchTargets[str] = score
	}
}
