// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path"
	"regexp"

	dutils "github.com/linuxdeepin/go-lib/utils"
)

type dataInfo struct {
	Key   string
	Value string
}

type matchResInfo struct {
	highestList   []string
	excellentList []string
	veryGoodList  []string
	goodList      []string
	aboveAvgList  []string
	avgList       []string
	belowAvgList  []string
	poorList      []string
}

func getCachePath() (string, bool) {
	userCache := dutils.GetCacheDir()
	if len(userCache) < 1 {
		return "", false
	}

	cachePath := path.Join(userCache, "deepin", "dde-daemon", "search")
	if !dutils.IsFileExist(cachePath) {
		if err := os.MkdirAll(cachePath, 0755); err != nil {
			logger.Warningf("MkdirAll '%s' failed: %v",
				cachePath, err)
			return "", false
		}
	}

	return cachePath, true
}

func sortMatchResult(res *matchResInfo) []string {
	ret := []string{}
	if res == nil {
		return ret
	}

	ret = append(ret, res.highestList...)
	ret = append(ret, res.excellentList...)
	ret = append(ret, res.veryGoodList...)
	ret = append(ret, res.goodList...)
	ret = append(ret, res.aboveAvgList...)
	ret = append(ret, res.avgList...)
	ret = append(ret, res.belowAvgList...)
	ret = append(ret, res.poorList...)

	return ret
}

func getMatchReuslt(info *dataInfo, matchers map[*regexp.Regexp]uint32,
	res *matchResInfo) {
	if info == nil || matchers == nil || res == nil {
		return
	}

	curScore := uint32(0)
	for matcher, score := range matchers {
		if matcher == nil {
			continue
		}

		if matcher.MatchString(info.Key) {
			logger.Debugf("score %d, target str: %q", score, info.Key)
			if score > curScore {
				curScore = score
			}
		}
	}

	switch curScore {
	case POOR:
		res.poorList = append(res.poorList,
			info.Value)
	case BELOW_AVERAGE:
		res.belowAvgList = append(res.belowAvgList,
			info.Value)
	case AVERAGE:
		res.avgList = append(res.avgList,
			info.Value)
	case ABOVE_AVERAGE:
		res.aboveAvgList = append(res.aboveAvgList,
			info.Value)
	case GOOD:
		res.goodList = append(res.goodList,
			info.Value)
	case VERY_GOOD:
		res.veryGoodList = append(res.veryGoodList,
			info.Value)
	case EXCELLENT:
		res.excellentList = append(res.excellentList,
			info.Value)
	case HIGHEST:
		res.highestList = append(res.highestList,
			info.Value)
	}
}

func searchString(key, md5 string) []string {
	list := []string{}
	cachePath, ok := getCachePath()
	if !ok {
		return list
	}

	filename := path.Join(cachePath, md5)
	if !dutils.IsFileExist(filename) {
		logger.Warningf("'%s' not exist", filename)
		return list
	}

	datas := []dataInfo{}
	if !readDatasFromFile(&datas, filename) {
		return list
	}

	matchers := getMatchers(key)
	var matchRes = matchResInfo{}
	for _, v := range datas {
		getMatchReuslt(&v, matchers, &matchRes)
	}

	list = sortMatchResult(&matchRes)

	return list
}

func searchStartWithString(key, md5 string) []string {
	list := []string{}
	cachePath, ok := getCachePath()
	if !ok {
		return list
	}

	filename := path.Join(cachePath, md5)
	if !dutils.IsFileExist(filename) {
		logger.Warningf("'%s' not exist", filename)
		return list
	}

	datas := []dataInfo{}
	if !readDatasFromFile(&datas, filename) {
		return list
	}

	match := regexp.MustCompile(fmt.Sprintf(`(?i)^(%s)`, key))
	for _, v := range datas {
		if match.MatchString(v.Key) {
			list = append(list, v.Value)
		}
	}

	return list
}
