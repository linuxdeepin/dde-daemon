// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package shortcuts

import (
	wm "github.com/linuxdeepin/go-dbus-factory/session/com.deepin.wm"
)

type ShortcutObject struct {
	BaseShortcut
	wm                 wm.Wm
	configSaveCallback func(id string, keystrokes []string) error
	configLoadCallback func(id string) ([]string, error)
}

func NewShortcut(wm wm.Wm, id string, type0 int32,
	keystrokes []string, name string,
	configSaveCallback func(id string, keystrokes []string) error,
	configLoadCallback func(id string) ([]string, error)) *ShortcutObject {
	gs := &ShortcutObject{
		BaseShortcut: BaseShortcut{
			Id:         id,
			Type:       type0,
			Keystrokes: ParseKeystrokes(keystrokes),
			Name:       name,
		},
		wm:                 wm,
		configSaveCallback: configSaveCallback,
		configLoadCallback: configLoadCallback,
	}

	return gs
}

func (gs *ShortcutObject) SaveKeystrokes() error {
	keystrokesStrv := make([]string, 0, len(gs.Keystrokes))
	for _, ks := range gs.Keystrokes {
		keystrokesStrv = append(keystrokesStrv, ks.String())
	}
	if _useWayland {
		ok, err := setShortForWayland(gs, gs.wm)
		if !ok {
			return err
		}
	}

	// 使用配置管理器回调保存配置
	if gs.configSaveCallback != nil {
		err := gs.configSaveCallback(gs.Id, keystrokesStrv)
		if err != nil {
			logger.Warning("Failed to save keystrokes via callback:", err)
			return err
		}
	}

	logger.Debugf("ShortcutObject.SaveKeystrokes id: %v, keystrokes: %v", gs.Id, keystrokesStrv)
	return nil
}

func keystrokesEqual(s1 []*Keystroke, s2 []*Keystroke) bool {
	l1 := len(s1)
	l2 := len(s2)

	if l1 != l2 {
		return false
	}

	for i := 0; i < l1; i++ {
		ks1 := s1[i]
		ks2 := s2[i]

		if ks1.String() != ks2.String() {
			return false
		}
	}
	return true
}

func (gs *ShortcutObject) ReloadKeystrokes() bool {
	oldVal := gs.GetKeystrokes()
	id := gs.GetId()

	// 使用配置管理器回调加载配置
	var keystrokesStrv []string
	if gs.configLoadCallback != nil {
		var err error
		keystrokesStrv, err = gs.configLoadCallback(id)
		if err != nil {
			logger.Warning("Failed to reload keystrokes via callback:", err)
			return false
		}
	} else {
		logger.Warning("No config load callback available for shortcut:", id)
		return false
	}

	newVal := ParseKeystrokes(keystrokesStrv)
	gs.setKeystrokes(newVal)
	return !keystrokesEqual(oldVal, newVal)
}
