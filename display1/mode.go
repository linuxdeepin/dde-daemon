// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"math"
	"regexp"
	"strconv"

	"github.com/linuxdeepin/go-lib/strv"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

type ModeInfo struct {
	Id     uint32
	name   string
	Width  uint16
	Height uint16
	Rate   float64
}

func (mi ModeInfo) isZero() bool {
	return mi == ModeInfo{}
}

type ModeInfos []ModeInfo

func (infos ModeInfos) Len() int {
	return len(infos)
}

func (infos ModeInfos) Less(i, j int) bool {
	areaI := int(infos[i].Width) * int(infos[i].Height)
	areaJ := int(infos[j].Width) * int(infos[j].Height)
	if areaI == areaJ {
		return infos[i].Rate < infos[j].Rate
	}
	return areaI < areaJ
}

func (infos ModeInfos) Swap(i, j int) {
	infos[i], infos[j] = infos[j], infos[i]
}

func toModeInfo(info randr.ModeInfo) ModeInfo {
	return ModeInfo{
		Id:     info.Id,
		name:   info.Name,
		Width:  info.Width,
		Height: info.Height,
		Rate:   calcModeRate(info),
	}
}

// 匹配比如 1024x768i 这样的
var regMode = regexp.MustCompile(`^(\d+)x(\d+)(\D+)$`)

// 过滤重复的模式，一定要保留 saveMode。
func filterModeInfos(modes []ModeInfo, saveMode ModeInfo) []ModeInfo {
	result := make([]ModeInfo, 0, len(modes))
	var filteredModeNames strv.Strv
	for idx := range modes {
		mode := modes[idx]
		// TODO 也许这个实现不够好
		if mode.Id == saveMode.Id {
			// not skip
			result = append(result, mode)
			continue
		}
		// 是否被过滤掉
		skip := false

		if filteredModeNames.Contains(mode.name) {
			skip = true
		} else {
			match := regMode.FindStringSubmatch(mode.name)
			if match != nil {
				m := findFirstMode(modes, func(mode1 ModeInfo) bool {
					return mode.Width == mode1.Width &&
						mode.Height == mode1.Height &&
						len(mode1.name) > 0 &&
						isDigit(mode1.name[len(mode1.name)-1])
				})
				if !m.isZero() {
					// 找到大小相同的 mode
					skip = true
					filteredModeNames = append(filteredModeNames, mode.name)
				}
			}

			if !skip {
				// 结果 result 中查找是否已经有几乎相同的模式，如果有则过滤掉这个。
				m := findFirstMode(result, func(mode1 ModeInfo) bool {
					return mode.Width == mode1.Width &&
						mode.Height == mode1.Height &&
						formatRate(mode.Rate) == formatRate(mode1.Rate)
				})
				if !m.isZero() {
					//logger.Debugf("compare mode: %s, find m: %s",
					//	spew.Sdump(mode), spew.Sdump(m))
					skip = true
				}
			}
		}

		if skip {
			//logger.Debugf("filterModeInfos skip mode %d|%x %s %.2f", mode.Id, mode.Id, mode.name, mode.Rate)
		} else {
			//logger.Debugf("add mode %d|%x %s %.2f", mode.Id, mode.Id, mode.name, mode.Rate)
			result = append(result, mode)
		}
	}
	return result
}

func findFirstMode(modes []ModeInfo, fn func(mode ModeInfo) bool) ModeInfo {
	for _, mode := range modes {
		if fn(mode) {
			return mode
		}
	}
	return ModeInfo{}
}

func calcModeRate(info randr.ModeInfo) float64 {
	vTotal := float64(info.VTotal)
	if (info.ModeFlags & randr.ModeFlagDoubleScan) != 0 {
		/* doublescan doubles the number of lines */
		vTotal *= 2
	}
	if (info.ModeFlags & randr.ModeFlagInterlace) != 0 {
		/* interlace splits the frame into two fields */
		/* the field rate is what is typically reported by monitors */
		vTotal /= 2
	}

	if info.HTotal == 0 || vTotal == 0 {
		return 0
	} else {
		return float64(info.DotClock) / (float64(info.HTotal) * vTotal)
	}
}

// RateFilterMap pciId => (size => rates)
type RateFilterMap map[string]map[string][]float64

func filterModeInfosByRefreshRate(modes []ModeInfo, filter RateFilterMap) []ModeInfo {
	var reservedModes []ModeInfo

	pciId := getGraphicsCardPciId()
	if pciId == "" {
		logger.Warning("failed to get current using graphics card pci id")
		return modes
	}

	// no refresh rate need to be filtered, directly return
	filterRefreshRateMap := filter[pciId]
	if len(filterRefreshRateMap) == 0 {
		return modes
	}

	for _, modeInfo := range modes {
		resolution := strconv.FormatUint(uint64(modeInfo.Width), 10) + "*" + strconv.FormatUint(uint64(modeInfo.Height), 10)

		// refresh rates need to be filtered at this resolution
		if filterRates, ok := filterRefreshRateMap[resolution]; ok {
			if !hasRate(filterRates, modeInfo.Rate) {
				reservedModes = append(reservedModes, modeInfo)
			}
		} else {
			reservedModes = append(reservedModes, modeInfo)
		}
	}
	return reservedModes
}

func modeInfosEqual(v1, v2 []ModeInfo) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i, e1 := range v1 {
		if e1 != v2[i] {
			return false
		}
	}
	return true
}

func toModeInfos(modes []randr.ModeInfo, modeIds []randr.Mode) (modeInfos []ModeInfo) {
	for _, id := range modeIds {
		modeInfo := findModeInfo(modes, id)
		if !modeInfo.isZero() {
			modeInfos = append(modeInfos, modeInfo)
		}
	}
	return
}

func findModeInfo(modes []randr.ModeInfo, modeId randr.Mode) ModeInfo {
	for _, mode := range modes {
		if uint32(modeId) == mode.Id {
			return toModeInfo(mode)
		}
	}
	return ModeInfo{}
}

func findMode(modes []ModeInfo, modeId uint32) ModeInfo {
	for _, modeInfo := range modes {
		if modeInfo.Id == modeId {
			return modeInfo
		}
	}
	return ModeInfo{}
}

// modeId 是 preferred mode 的 id
func getPreferredMode(modes []ModeInfo, modeId uint32) ModeInfo {
	mode := findMode(modes, modeId)
	if !mode.isZero() {
		return mode
	}
	if len(modes) > 0 {
		return modes[0]
	}
	return ModeInfo{}
}

func getBestMode(modes []ModeInfo, preferredMode ModeInfo) ModeInfo {
	mode := findMode(modes, preferredMode.Id)
	if !mode.isZero() {
		return mode
	}
	mode = getFirstModeBySize(modes, preferredMode.Width, preferredMode.Height)
	if !mode.isZero() {
		return mode
	}
	if len(modes) > 0 {
		return modes[0]
	}
	return ModeInfo{}
}

func getFirstModeBySize(modes []ModeInfo, width, height uint16) ModeInfo {
	for _, modeInfo := range modes {
		if modeInfo.Width == width && modeInfo.Height == height {
			return modeInfo
		}
	}
	return ModeInfo{}
}

func getFirstModeBySizeRate(modes []ModeInfo, width, height uint16, rate float64) ModeInfo {
	roundedRate := math.Round(rate * 100)
	for _, modeInfo := range modes {
		if modeInfo.Width == width && modeInfo.Height == height &&
			math.Round(modeInfo.Rate * 100) == roundedRate {
			return modeInfo
		}
	}
	return ModeInfo{}
}

type Size struct {
	width  uint16
	height uint16
}

func getSizeModeMap(modes []ModeInfo) map[Size][]uint32 {
	result := make(map[Size][]uint32)
	for _, modeInfo := range modes {
		result[Size{modeInfo.Width, modeInfo.Height}] = append(
			result[Size{modeInfo.Width, modeInfo.Height}], modeInfo.Id)
	}
	return result
}
