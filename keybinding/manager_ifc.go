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

package keybinding

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus"
	"pkg.deepin.io/dde/daemon/keybinding/shortcuts"
	"pkg.deepin.io/dde/daemon/keybinding/util"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	dbusServiceName = "com.deepin.daemon.Keybinding"
	dbusPath        = "/com/deepin/daemon/Keybinding"
	dbusInterface   = "com.deepin.daemon.Keybinding"
)

type ErrInvalidShortcutType struct {
	Type int32
}

func (err ErrInvalidShortcutType) Error() string {
	return fmt.Sprintf("shortcut type %v is invalid", err.Type)
}

type ErrShortcutNotFound struct {
	Id   string
	Type int32
}

func (err ErrShortcutNotFound) Error() string {
	return fmt.Sprintf("shortcut id %q type %v is not found", err.Id, err.Type)
}

var errTypeAssertionFail = errors.New("type assertion failed")
var errShortcutKeystrokesUnmodifiable = errors.New("keystrokes of this shortcut is unmodifiable")
var errKeystrokeUsed = errors.New("keystroke had been used")
var errNameUsed = errors.New("name had been used")

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

//true : ignore
func (m *Manager) isIgnoreRepeat(name string) bool {
	const minKeyEventInterval = 200 * time.Millisecond
	now := time.Now()
	duration := now.Sub(m.lastMethodCalledTime)
	logger.Debug("duration:", duration)
	if 0 < duration && duration < minKeyEventInterval {
		logger.Debug(">>Ignore ", name)
		return true
	}
	m.lastMethodCalledTime = now
	return false
}

// Reset reset all shortcut
func (m *Manager) Reset() *dbus.Error {
	if m.isIgnoreRepeat("Reset") {
		return nil
	}

	customShortcuts := m.customShortcutManager.List()
	m.shortcutManager.UngrabAll()

	m.enableListenGSettingsChanged(false)
	// reset all gsettings
	resetGSettings(m.gsSystem)
	resetGSettings(m.gsMediaKey)
	if m.gsGnomeWM != nil {
		resetGSettings(m.gsGnomeWM)
	}

	// reset for KWin
	if shouldUseDDEKwin() {
		err := resetKWin(m.wm)
		if err != nil {
			logger.Warning("failed to reset for KWin:", err)
		}
		// 由于快捷键冲突原因，有必要重置两遍
		err = resetKWin(m.wm)
		if err != nil {
			logger.Warning("failed to reset for KWin:", err)
		}
	}

	changes := m.shortcutManager.ReloadAllShortcutsKeystrokes()
	m.enableListenGSettingsChanged(true)
	m.shortcutManager.GrabAll()

	for _, cs := range customShortcuts {
		keystrokes := cs.GetKeystrokes()
		var newKeystrokes []*shortcuts.Keystroke
		modifyFlag := false
		for _, keystroke := range keystrokes {
			conflictKeystroke, err := m.shortcutManager.FindConflictingKeystroke(keystroke)
			if err != nil {
				logger.Warning(err)
				modifyFlag = true
				continue
			}
			if conflictKeystroke != nil {
				logger.Debugf("keystroke %v has conflict", keystroke)
				modifyFlag = true
			} else {
				newKeystrokes = append(newKeystrokes, keystroke)
			}
		}

		cs0 := m.shortcutManager.GetByUid(cs.GetUid())
		if cs0 == nil {
			logger.Warning("cs0 is nil")
			continue
		}

		m.shortcutManager.ModifyShortcutKeystrokes(cs0, newKeystrokes)
		if modifyFlag {
			err := cs0.SaveKeystrokes()
			if err != nil {
				logger.Warning(err)
			}
			// 将修改的自定义快捷键补充到 changes 列表中
			changes = append(changes, cs0)
		}
	}

	for _, shortcut := range changes {
		m.emitShortcutSignal(shortcutSignalChanged, shortcut)
	}
	return nil
}

func (m *Manager) ListAllShortcuts() (shortcuts string, busErr *dbus.Error) {
	list := m.shortcutManager.List()
	ret, err := util.MarshalJSON(list)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return ret, nil
}

func (m *Manager) ListShortcutsByType(type0 int32) (shortcuts string, busErr *dbus.Error) {
	list := m.shortcutManager.ListByType(type0)
	ret, err := util.MarshalJSON(list)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return ret, nil
}

func (m *Manager) AddCustomShortcut(name, action, keystroke string) (id string,
	type0 int32, busErr *dbus.Error) {

	logger.Debugf("Add custom key: %q %q %q", name, action, keystroke)
	ks, err := shortcuts.ParseKeystroke(keystroke)
	if err != nil {
		logger.Warning(err)
		busErr = dbusutil.ToError(err)
		return
	}

	exist := m.shortcutManager.GetByIdType(name, shortcuts.ShortcutTypeCustom)
	if exist != nil {
		err = errNameUsed
		logger.Warning(err)
		busErr = dbusutil.ToError(err)
		return
	}

	conflictKeystroke, err := m.shortcutManager.FindConflictingKeystroke(ks)
	if err != nil {
		logger.Warning(err)
		busErr = dbusutil.ToError(err)
		return
	}
	if conflictKeystroke != nil {
		err = errKeystrokeUsed
		logger.Warning(err)
		busErr = dbusutil.ToError(err)
		return
	}

	shortcut, err := m.customShortcutManager.Add(name, action, []*shortcuts.Keystroke{ks})
	if err != nil {
		logger.Warning(err)
		busErr = dbusutil.ToError(err)
		return
	}
	m.shortcutManager.Add(shortcut)
	m.emitShortcutSignal(shortcutSignalAdded, shortcut)
	id = shortcut.GetId()
	type0 = shortcut.GetType()
	return
}

func (m *Manager) DeleteCustomShortcut(id string) *dbus.Error {
	shortcut := m.shortcutManager.GetByIdType(id, shortcuts.ShortcutTypeCustom)
	if err := m.customShortcutManager.Delete(shortcut.GetId()); err != nil {
		return dbusutil.ToError(err)
	}
	m.shortcutManager.Delete(shortcut)
	m.emitShortcutSignal(shortcutSignalDeleted, shortcut)
	return nil
}

func (m *Manager) ClearShortcutKeystrokes(id string, type0 int32) *dbus.Error {
	logger.Debug("ClearShortcutKeystrokes", id, type0)
	shortcut := m.shortcutManager.GetByIdType(id, type0)
	if shortcut == nil {
		return dbusutil.ToError(ErrShortcutNotFound{id, type0})
	}
	m.shortcutManager.ModifyShortcutKeystrokes(shortcut, nil)
	err := shortcut.SaveKeystrokes()
	if err != nil {
		return dbusutil.ToError(err)
	}
	if shortcut.ShouldEmitSignalChanged() {
		m.emitShortcutSignal(shortcutSignalChanged, shortcut)
	}
	return nil
}

func (m *Manager) LookupConflictingShortcut(keystroke string) (shortcut string, busErr *dbus.Error) {
	ks, err := shortcuts.ParseKeystroke(keystroke)
	if err != nil {
		// parse keystroke error
		return "", dbusutil.ToError(err)
	}

	conflictKeystroke, err := m.shortcutManager.FindConflictingKeystroke(ks)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	if conflictKeystroke != nil {
		detail, err := util.MarshalJSON(conflictKeystroke.Shortcut)
		if err != nil {
			return "", dbusutil.ToError(err)
		}
		return detail, nil
	}
	return "", nil
}

// ModifyCustomShortcut modify custom shortcut
//
// id: shortcut id
// name: new name
// cmd: new commandline
// keystroke: new keystroke
func (m *Manager) ModifyCustomShortcut(id, name, cmd, keystroke string) *dbus.Error {
	logger.Debugf("ModifyCustomShortcut id: %q, name: %q, cmd: %q, keystroke: %q", id, name, cmd, keystroke)
	const ty = shortcuts.ShortcutTypeCustom
	// get the shortcut
	shortcut := m.shortcutManager.GetByIdType(id, ty)
	if shortcut == nil {
		return dbusutil.ToError(ErrShortcutNotFound{id, ty})
	}
	customShortcut, ok := shortcut.(*shortcuts.CustomShortcut)
	if !ok {
		return dbusutil.ToError(errTypeAssertionFail)
	}

	var keystrokes []*shortcuts.Keystroke
	if keystroke != "" {
		ks, err := shortcuts.ParseKeystroke(keystroke)
		if err != nil {
			return dbusutil.ToError(err)
		}
		// check conflicting
		conflictKeystroke, err := m.shortcutManager.FindConflictingKeystroke(ks)
		if err != nil {
			return dbusutil.ToError(err)
		}
		if conflictKeystroke != nil && conflictKeystroke.Shortcut != shortcut {
			return dbusutil.ToError(errKeystrokeUsed)
		}
		keystrokes = []*shortcuts.Keystroke{ks}
	}

	// modify then save
	customShortcut.SetName(name)
	customShortcut.Cmd = cmd
	m.shortcutManager.ModifyShortcutKeystrokes(shortcut, keystrokes)
	err := customShortcut.Save()
	if err != nil {
		return dbusutil.ToError(err)
	}
	m.emitShortcutSignal(shortcutSignalChanged, shortcut)
	return nil
}

func (m *Manager) AddShortcutKeystroke(id string, type0 int32, keystroke string) *dbus.Error {
	logger.Debug("AddShortcutKeystroke", id, type0, keystroke)
	shortcut := m.shortcutManager.GetByIdType(id, type0)
	if shortcut == nil {
		return dbusutil.ToError(ErrShortcutNotFound{id, type0})
	}
	if !shortcut.GetKeystrokesModifiable() {
		return dbusutil.ToError(errShortcutKeystrokesUnmodifiable)
	}

	ks, err := shortcuts.ParseKeystroke(keystroke)
	if err != nil {
		// parse keystroke error
		return dbusutil.ToError(err)
	}
	logger.Debug("keystroke:", ks.DebugString())

	if type0 == shortcuts.ShortcutTypeWM && ks.Mods == 0 {
		keyLower := strings.ToLower(ks.Keystr)
		if keyLower == "super_l" || keyLower == "super_r" {
			return dbusutil.ToError(errors.New(
				"keystroke of shortcut which type is wm can not be set to the Super key"))
		}
	}

	conflictKeystroke, err := m.shortcutManager.FindConflictingKeystroke(ks)
	if err != nil {
		return dbusutil.ToError(err)
	}
	if conflictKeystroke == nil {
		m.shortcutManager.AddShortcutKeystroke(shortcut, ks)
		err := shortcut.SaveKeystrokes()
		if err != nil {
			return dbusutil.ToError(err)
		}
		if shortcut.ShouldEmitSignalChanged() {
			m.emitShortcutSignal(shortcutSignalChanged, shortcut)
		}
	} else if conflictKeystroke.Shortcut != shortcut {
		return dbusutil.ToError(errKeystrokeUsed)
	}
	return nil
}

func (m *Manager) DeleteShortcutKeystroke(id string, type0 int32, keystroke string) *dbus.Error {
	logger.Debug("DeleteShortcutKeystroke", id, type0, keystroke)
	shortcut := m.shortcutManager.GetByIdType(id, type0)
	if shortcut == nil {
		return dbusutil.ToError(ErrShortcutNotFound{id, type0})
	}
	if !shortcut.GetKeystrokesModifiable() {
		return dbusutil.ToError(errShortcutKeystrokesUnmodifiable)
	}

	ks, err := shortcuts.ParseKeystroke(keystroke)
	if err != nil {
		// parse keystroke error
		return dbusutil.ToError(err)
	}
	logger.Debug("keystroke:", ks.DebugString())

	m.shortcutManager.DeleteShortcutKeystroke(shortcut, ks)
	err = shortcut.SaveKeystrokes()
	if err != nil {
		return dbusutil.ToError(err)
	}
	if shortcut.ShouldEmitSignalChanged() {
		m.emitShortcutSignal(shortcutSignalChanged, shortcut)
	}
	return nil
}

func (m *Manager) GetShortcut(id string, type0 int32) (shortcut string, busErr *dbus.Error) {
	s := m.shortcutManager.GetByIdType(id, type0)
	if s == nil {
		return "", dbusutil.ToError(ErrShortcutNotFound{id, type0})
	}
	detail, err := s.Marshal()
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return detail, nil
}

func (m *Manager) SelectKeystroke() *dbus.Error {
	logger.Debug("SelectKeystroke")
	err := m.selectKeystroke()
	return dbusutil.ToError(err)
}

func (m *Manager) SetNumLockState(state int32) *dbus.Error {
	logger.Debug("SetNumLockState", state)
	if len(os.Getenv("WAYLAND_DISPLAY")) != 0 {
		err := setNumLockState(m.conn, m.keySymbols, NumLockState(state))
		m.handleKeyEventByWayland("numlock")
		return dbusutil.ToError(err)
	}

	err := setNumLockState(m.conn, m.keySymbols, NumLockState(state))
	return dbusutil.ToError(err)
}

func (m *Manager) SearchShortcuts(query string) (shortcuts string, busErr *dbus.Error) {
	list := m.shortcutManager.Search(query)
	ret, err := util.MarshalJSON(list)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return ret, nil
}

func (m *Manager) GetCapsLockState() (state int32, busErr *dbus.Error) {
	lockState, err := queryCapsLockState(m.conn)
	return int32(lockState), dbusutil.ToError(err)
}

func (m *Manager) SetCapsLockState(state int32) *dbus.Error {
	logger.Debug("SetCapsLockState", state)
	err := setCapsLockState(m.conn, m.keySymbols, CapsLockState(state))
	return dbusutil.ToError(err)
}
