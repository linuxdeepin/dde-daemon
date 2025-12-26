// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"fmt"

	"github.com/godbus/dbus/v5"
	langselector "github.com/linuxdeepin/dde-daemon/langselector1"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (m *Mouse) Reset() *dbus.Error {
	return dbusutil.ToError(m.dsgMouseConfig.ResetAll())
}

func (m *Mouse) Enable(enabled bool) *dbus.Error {
	if err := m.enable(enabled); err != nil {
		return dbusutil.ToError(err)
	}

	return nil
}

func (tp *TrackPoint) Reset() *dbus.Error {
	return dbusutil.ToError(tp.dsgTrackPointConfig.ResetAll())
}

func (tpad *Touchpad) Reset() *dbus.Error {
	return dbusutil.ToError(tpad.dsgTouchpadConfig.ResetAll())
}

func (tpad *Touchpad) Enable(enabled bool) *dbus.Error {
	sysTP := tpad.sysTouchPad
	if sysTP != nil {
		return dbusutil.ToError(sysTP.SetTouchpadEnable(0, enabled))
	} else {
		return dbusutil.ToError(fmt.Errorf("system touchpad is nul"))
	}
}

func (w *Wacom) Reset() *dbus.Error {
	return dbusutil.ToError(w.dsgWacomConfig.ResetAll())
}

func (kbd *Keyboard) Reset() *dbus.Error {
	return dbusutil.ToError(kbd.dsgKeyboardConfig.ResetAll())
}

func (kbd *Keyboard) LayoutList() (map[string]string, *dbus.Error) {
	locales := langselector.GetLocales()
	result := kbd.layoutMap.filterByLocales(locales)

	kbd.PropsMu.RLock()
	for _, layout := range kbd.UserLayoutList.Get() {
		layoutDetail := kbd.layoutMap[layout]
		result[layout] = layoutDetail.Description
	}
	kbd.PropsMu.RUnlock()

	return result, nil
}

func (kbd *Keyboard) AllLayoutList() (map[string]string, *dbus.Error) {
	result := kbd.layoutMap.getAll()
	kbd.PropsMu.RLock()
	for _, layout := range kbd.UserLayoutList.Get() {
		layoutDetail := kbd.layoutMap[layout]
		result[layout] = layoutDetail.Description
	}
	kbd.PropsMu.RUnlock()

	return result, nil
}

func (kbd *Keyboard) GetLayoutDesc(layout string) (string, *dbus.Error) {
	if len(layout) == 0 {
		return "", nil
	}

	value, ok := kbd.layoutMap[layout]
	if !ok {
		return "", nil
	}

	return value.Description, nil
}

func (kbd *Keyboard) AddUserLayout(layout string) *dbus.Error {
	err := kbd.checkLayout(layout)
	if err != nil {
		return dbusutil.ToError(errInvalidLayout)
	}

	kbd.addUserLayout(layout)
	return nil
}

func (kbd *Keyboard) DeleteUserLayout(layout string) *dbus.Error {
	kbd.delUserLayout(layout)
	return nil
}

func (kbd *Keyboard) AddLayoutOption(option string) *dbus.Error {
	kbd.addUserOption(option)
	return nil
}

func (kbd *Keyboard) DeleteLayoutOption(option string) *dbus.Error {
	kbd.delUserOption(option)
	return nil
}

func (kbd *Keyboard) ClearLayoutOption() *dbus.Error {
	kbd.UserOptionList.Set([]string{})
	return nil
}

func (kbd *Keyboard) ToggleNextLayout() *dbus.Error {
	kbd.toggleNextLayout()
	return nil
}
