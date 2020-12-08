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
	"unicode"

	"github.com/Lofanmi/pinyin-golang/pinyin"
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
		name = appInfo.GetGenericName()
		if name == "" {
			name = appInfo.GetName()
		}
	} else {
		name = appInfo.GetName()
	}

	if name == "" {
		name = appInfo.GetId()
	}

	filename := appInfo.GetFileName()
	ctime, err := getFileCTime(filename)
	if err != nil {
		logger.Warningf("failed to get file %q ctime: %v", filename, err)
	}

	item := &Item{
		Path:            filename,
		TimeInstalled:   ctime,
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
	return getXCategory(item.categories)
}

const (
	idScore          = 100
	nameScore        = 80
	genericNameScore = 70
	keywordScore     = 60
	categoryScore    = 60
)

func isHans(data rune) bool {
	return unicode.Is(unicode.Han, data)
}

func hansToPinyinAndAbbr(hans string) (string, string) {
	dict := pinyin.NewDict()
	pyTmp := dict.Sentence(hans).None()
	abbrTmp := dict.Abbr(hans, "")
	return pyTmp, abbrTmp
}

//获取拼音和拼音简拼,如Qt 5 设计器 -> Qt5shejiqi, Qt5sjq
//当字符中有字母和数字以及汉字时，只转换汉字部分，其他部分不转换
func toPinyinAndAbbr(str string) (string, string) {
	var hans strings.Builder
	var haveHans bool
	var pinYin strings.Builder
	var abbr strings.Builder
	for _, v := range str {
		if isHans(v) {
			hans.WriteRune(v)
			haveHans = true
		} else {
			if haveHans {
				pyTmp, abbrTmp := hansToPinyinAndAbbr(hans.String())
				pinYin.WriteString(pyTmp)
				abbr.WriteString(abbrTmp)
				haveHans = false
				hans.Reset()
			}
			pinYin.WriteRune(v)
			abbr.WriteRune(v)
		}
	}
	if haveHans {
		pyTmp, abbrTmp := hansToPinyinAndAbbr(hans.String())
		pinYin.WriteString(pyTmp)
		abbr.WriteString(abbrTmp)
	}

	return strings.Replace(pinYin.String(), " ", "", -1), strings.Replace(abbr.String(), " ", "", -1)

}

func (item *Item) setSearchTargets(pinyinEnabled bool) {
	item.addSearchTarget(nameScore, item.Name)
	item.addSearchTarget(nameScore, item.enName)

	if pinyinEnabled {
		pinyin, abbr := toPinyinAndAbbr(item.Name)
		item.addSearchTarget(nameScore, pinyin)
		item.addSearchTarget(nameScore, abbr)
	}
}

func (item *Item) addSearchTarget(score SearchScore, str string) {
	if str == "" {
		return
	}
	str = strings.Replace(str, " ", "", -1)
	str = strings.ToLower(str)
	scoreInDict, ok := item.searchTargets[str]
	if !ok || (ok && scoreInDict < score) {
		item.searchTargets[str] = score
	}
}

func (item *Item) deleteSearchTarget(str string) {
	if str == "" {
		return
	}
	str = strings.Replace(str, " ", "", -1)
	str = strings.ToLower(str)
	delete(item.searchTargets, str)
}
