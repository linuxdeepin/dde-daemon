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
	if m.tsSetting.GetInt(tsSchemaKeyLongPress) == int32(duration) {
		return nil
	}
	err := m.sysDaemon.SetLongPressDuration(0, duration)
	if err != nil {
		return dbusutil.ToError(err)
	}
	m.tsSetting.SetInt(tsSchemaKeyLongPress, int32(duration))
	return nil
}

func (m *Manager) GetLongPressDuration() (duration uint32, busErr *dbus.Error) {
	return uint32(m.tsSetting.GetInt(tsSchemaKeyLongPress)), nil
}

func (m *Manager) SetShortPressDuration(duration uint32) *dbus.Error {
	if m.tsSetting.GetInt(tsSchemaKeyShortPress) == int32(duration) {
		return nil
	}
	err := m.gesture.SetShortPressDuration(0, duration)
	if err != nil {
		return dbusutil.ToError(err)
	}
	m.tsSetting.SetInt(tsSchemaKeyShortPress, int32(duration))
	return nil
}

func (m *Manager) GetShortPressDuration() (duration uint32, busErr *dbus.Error) {
	return uint32(m.tsSetting.GetInt(tsSchemaKeyShortPress)), nil
}

func (m *Manager) SetEdgeMoveStopDuration(duration uint32) *dbus.Error {
	if m.tsSetting.GetInt(tsSchemaKeyShortPress) == int32(duration) {
		return nil
	}
	err := m.gesture.SetEdgeMoveStopDuration(0, duration)
	if err != nil {
		return dbusutil.ToError(err)
	}
	m.tsSetting.SetInt(tsSchemaKeyEdgeMoveStop, int32(duration))
	return nil
}

func (m *Manager) GetEdgeMoveStopDuration() (duration uint32, busErr *dbus.Error) {
	return uint32(m.tsSetting.GetInt(tsSchemaKeyEdgeMoveStop)), nil
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
			m.saveGestureConfig()
			break
		}
	}
	return nil
}
