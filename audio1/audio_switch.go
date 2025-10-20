// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

// 处理端口切换、声卡切换逻辑

package audio

import (
	"strings"

	"github.com/linuxdeepin/go-lib/pulse"
	"github.com/linuxdeepin/go-lib/strv"
)

func (a *Audio) getCardById(id uint32) *Card {
	for _, c := range a.cards {
		if c.Id == id {
			return c
		}
	}
	return nil
}

// checkCardIsReady: 声卡如果发生变化，不能进行自动切换逻辑
// 应该检查dde内存信息，不应该直接去查pulse接口
// 因为dde的状态需要通过pulse事件同步，这个是有滞后性的，会有一个中间状态，上层未刷新信息，pulse的状态已经变化了。
// 应该以收到的事件为准来更新dde音频的内存信息。
// 只有dde内存信息刷新后，才算声卡准备完成。
func (a *Audio) checkCardIsReady(id uint32) bool {
	card := a.getCardById(id)
	if card == nil {
		// 声卡不存在，可能已经被删除。需要自动更新端口
		logger.Warningf("check card %v is ready, mybe card has been delete", id)
		return true
	}
	// 如果没有可用的profile，则声卡不会产生sink/source，视为准备完成
	if len(card.Profiles) == 0 {
		logger.Warningf("check card %v is ready, no avaiable profile", card.Name)
		return true
	}

	// 可用配置文件为空？没有准备好，不能引发端口切换
	if card.ActiveProfile == nil {
		logger.Warningf("card %v not ready, active profile is nil", card.Name)
		return false
	}
	// 遍历声卡端口，找出和当前配置文件匹配的端口
	var availablePorts strv.Strv
	var hasSink, hasSource bool
	for _, port := range card.Ports {
		if port.Profiles.Exists(card.ActiveProfile.Name) {
			switch port.Direction {
			case pulse.DirectionSink:
				hasSink = true
			case pulse.DirectionSource:
				hasSource = true
			}
			availablePorts = append(availablePorts, port.Name)
		}
	}
	// 检查声卡的profile是否已经设置完成
	// 检查当前profile的可用端口
	// 检查输入输出列表
	var isSinkReady, isSourceReady bool

	if hasSink {
		for _, sink := range a.sinks {
			if sink.Card == card.Id && len(sink.Ports) > 0 && availablePorts.Contains(sink.ActivePort.Name) {
				isSinkReady = true
				break
			}
		}
	}

	if hasSource {
		// TODO: 蓝牙的source在切换profile时，虽然底层显示activePort变化了，但是dde没有收到变化的事件
		// 该问题导致声卡没有准备好，无法进行端口切换
		for _, source := range a.sources {
			if strings.Contains(source.Name, ".monitor") {
				continue
			}
			if source.Card == card.Id && len(source.Ports) > 0 && availablePorts.Contains(source.ActivePort.Name) {
				isSourceReady = true
				break
			}
		}
	}
	// 如果有sink，检查sink是否准备好；如果有source，检查source是否准备好
	// 只要存在的设备都准备好了，就认为声卡准备完成
	sinkOk := !hasSink || isSinkReady
	sourceOk := !hasSource || isSourceReady

	logger.Infof("check card %s is finaly ready: %v", card.Name, sinkOk && sourceOk)
	return sinkOk && sourceOk
}
