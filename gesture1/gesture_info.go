// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package gesture1

import (
	"fmt"
)

const (
	gestureSchemaId         = "com.deepin.dde.gesture"
	gsKeyTouchPadEnabled    = "touch-pad-enabled"
	gsKeyTouchScreenEnabled = "touch-screen-enabled"
)

type EventInfo struct {
	Name      string
	Direction string
	Fingers   int32
}

type GestureInfo struct {
	Name       string
	Direction  string
	Fingers    int32
	ActionName string
}

type GestureInfos []*GestureInfo

var gestureInfos = GestureInfos{
	{"swipe", "up", 3, "MaximizeWindow"},
	{"swipe", "down", 3, "RestoreWindow"},
	{"swipe", "left", 3, "SplitWindowLeft"},
	{"swipe", "right", 3, "SplitWindowRight"},
	{"tap", "none", 3, "ToggleGrandSearch"},
	{"swipe", "up", 4, "ShowMultiTask"},
	{"swipe", "down", 4, "HideMultitask"},
	{"swipe", "left", 4, "SwitchToPreDesktop"},
	{"swipe", "right", 4, "SwitchToNextDesktop"},
	{"tap", "none", 4, "ToggleLaunchPad"},
	{"touch right button", "down", 0, "MouseRightButtonDown"},
	{"touch right button", "up", 0, "MouseRightButtonUp"},
}

func (m *Manager) GetGestureByEvent(event EventInfo) *GestureInfo {
	for _, gesture := range m.Infos {
		if gesture.Name == event.Name &&
			gesture.Direction == event.Direction &&
			gesture.Fingers == event.Fingers {
			return gesture
		}
	}
	return nil
}

func (info *GestureInfo) toString() string {
	return fmt.Sprintf("Name=%s, Direction=%s, Fingers=%d action info:%v", info.Name, info.Direction, info.Fingers, info.ActionName)
}

func (evInfo EventInfo) toString() string {
	return fmt.Sprintf("Name=%s, Direction=%s, Fingers=%d", evInfo.Name, evInfo.Direction, evInfo.Fingers)
}

func (info *GestureInfo) doAction() error {
	for _, action := range actions {
		if info.ActionName == action.Name {
			if action.fn != nil {
				return action.fn()
			}
		}
	}
	return nil
}
