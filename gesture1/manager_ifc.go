// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package gesture1

import (
	"encoding/json"
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
)

func (m *Manager) SetLongPressDuration(duration uint32) *dbus.Error {
	if value, _ := m.dconfigTouchScreen.GetValueInt64(dconfigKeyLongpressDuration); value == int64(duration) {
		return nil
	}
	err := m.sysDaemon.SetLongPressDuration(0, duration)
	if err != nil {
		return dbusutil.ToError(err)
	}
	m.dconfigTouchScreen.SetValue(dconfigKeyLongpressDuration, int64(duration))
	return nil
}

func (m *Manager) GetLongPressDuration() (duration uint32, busErr *dbus.Error) {
	value, err := m.dconfigTouchScreen.GetValueInt64(dconfigKeyLongpressDuration)
	return uint32(value), dbusutil.ToError(err)
}

func (m *Manager) SetShortPressDuration(duration uint32) *dbus.Error {
	if value, _ := m.dconfigTouchScreen.GetValueInt64(dconfigKeyShortpressDuration); value == int64(duration) {
		return nil
	}
	err := m.gesture.SetShortPressDuration(0, duration)
	if err != nil {
		return dbusutil.ToError(err)
	}
	m.dconfigTouchScreen.SetValue(dconfigKeyShortpressDuration, duration)
	return nil
}

func (m *Manager) GetShortPressDuration() (duration uint32, busErr *dbus.Error) {
	value, err := m.dconfigTouchScreen.GetValueInt64(dconfigKeyShortpressDuration)
	return uint32(value), dbusutil.ToError(err)
}

func (m *Manager) SetEdgeMoveStopDuration(duration uint32) *dbus.Error {
	if value, _ := m.dconfigTouchScreen.GetValueInt64(dconfigKeyEdgemovestopDuration); value == int64(duration) {
		return nil
	}
	err := m.gesture.SetEdgeMoveStopDuration(0, duration)
	if err != nil {
		return dbusutil.ToError(err)
	}
	m.dconfigTouchScreen.SetValue(dconfigKeyEdgemovestopDuration, int32(duration))
	return nil
}

func (m *Manager) GetEdgeMoveStopDuration() (duration uint32, busErr *dbus.Error) {
	value, err := m.dconfigTouchScreen.GetValueInt64(dconfigKeyEdgemovestopDuration)
	return uint32(value), dbusutil.ToError(err)
}

// GetGestureAvaiableActions 获取手势可选的操作
func (m *Manager) GetGestureAvaiableActions(name string, fingers int32) (string, *dbus.Error) {
	search := func(typ string) (string, *dbus.Error) {
		if m.availableGestures[typ] == nil {
			return "", dbusutil.ToError(fmt.Errorf("%s has no available actions", typ))
		}
		var actionMap actionInfos
		for _, gusture := range m.availableGestures[typ] {
			for _, info := range actions {
				if info.Name == gusture {
					actionMap = append(actionMap, info)
					continue
				}
			}
		}
		infos, err := json.Marshal(actionMap)
		if err != nil {
			return "", dbusutil.ToError(err)
		}
		return string(infos), nil
	}
	if name == "swipe" {
		if fingers == 3 {
			return search(availableGesturesWith3Fingers)
		} else if fingers == 4 {
			return search(availableGesturesWith4Fingers)
		}
	} else if name == "tap" {
		return search(availableGesturesWithActionTap)
	}
	return "", nil
}

func (m *Manager) SetGesture(name string, direction string, fingers int32, action string) *dbus.Error {
	var typ string
	if name == "swipe" {
		if fingers == 3 {
			typ = availableGesturesWith3Fingers

		} else if fingers == 4 {
			typ = availableGesturesWith4Fingers
		}
	} else if name == "tap" {
		typ = availableGesturesWithActionTap
	}
	if typ != "" {
		available := m.availableGestures[typ]
		if !strv.Strv(available).Contains(action) {
			return dbusutil.ToError(fmt.Errorf("actions %v not found", action))
		}
	}
	for _, gesture := range m.Infos {
		if gesture.Name == name &&
			gesture.Direction == direction &&
			gesture.Fingers == fingers {
			gesture.ActionName = action
			err := m.emitPropChangedInfos(m.Infos)
			if err != nil {
				logger.Warning(err)
			}
			m.saveGestureConfig()
			break
		}
	}
	return nil
}
