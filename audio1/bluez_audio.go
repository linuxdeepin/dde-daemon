// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"strings"

	"github.com/linuxdeepin/go-lib/strv"
)

const (
	bluezModeA2dp      = "a2dp"
	bluezModeHeadset   = "headset"
	bluezModeHandsfree = "handsfree"
)

const (
	PriorityA2dp     = 20000
	PriorityHandset  = 10000
	PriorityHandfree = 10000
)

var (
	bluezModeDefault    = bluezModeA2dp
	bluezModeFilterList = []string{"a2dp_source"}
)

/* 判断设备是否是蓝牙设备，可以用声卡名，也可以用sink、端口等名称 */
func isBluezAudio(name string) bool {
	return strings.Contains(strings.ToLower(name), "bluez")
}

/* 获取蓝牙声卡的模式(a2dp/headset) */
func (card *Card) BluezMode() string {
	profileName := strings.ToLower(card.ActiveProfile.Name)
	if strings.Contains(strings.ToLower(profileName), bluezModeA2dp) {
		return bluezModeA2dp
	} else if strings.Contains(strings.ToLower(profileName), bluezModeHeadset) {
		return bluezModeHeadset
	} else if strings.Contains(strings.ToLower(profileName), bluezModeHandsfree) {
		return bluezModeHandsfree
	} else {
		return ""
	}
}

/* 获取蓝牙声卡的可用模式 */
func (card *Card) BluezModeOpts() []string {
	opts := []string{}
	filterList := strv.Strv(bluezModeFilterList)
	for _, profile := range card.Profiles {
		if profile.Available == 0 {
			logger.Debugf("%s %s is unavailable", card.core.Name, profile.Name)
			continue
		}

		v := strings.ToLower(profile.Name)

		// pulseaudio和pipewier返回的端口、profile的名称的风格不一样，一个使用中横线一个使用下划线
		if filterList.Contains(strings.ReplaceAll(v, "-", "_")) {
			logger.Debug("filter bluez mode", v)
			continue
		}

		if strings.Contains(strings.ToLower(profile.Name), "a2dp") {
			opts = append(opts, "a2dp")
		}

		if strings.Contains(strings.ToLower(profile.Name), "headset") {
			opts = append(opts, "headset")
		}

		if strings.Contains(strings.ToLower(profile.Name), "handsfree") {
			opts = append(opts, "handsfree")
		}
	}
	return opts
}
