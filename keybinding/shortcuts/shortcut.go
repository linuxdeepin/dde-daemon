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

package shortcuts

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"pkg.deepin.io/dde/daemon/keybinding/util"
)

type BaseShortcut struct {
	mu         sync.Mutex
	Id         string
	Type       int32
	Keystrokes []*Keystroke `json:"Accels"`
	Name       string
}

func (sb *BaseShortcut) String() string {
	sb.mu.Lock()
	str := fmt.Sprintf("Shortcut{id=%s type=%d name=%q keystrokes=%v}", sb.Id, sb.Type, sb.Name, sb.Keystrokes)
	sb.mu.Unlock()
	return str
}

func (sb *BaseShortcut) Marshal() (string, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return util.MarshalJSON(sb)
}

func (sb *BaseShortcut) GetId() string {
	return sb.Id
}

func idType2Uid(id string, type0 int32) string {
	return strconv.Itoa(int(type0)) + id
}

func (sb *BaseShortcut) GetUid() string {
	return idType2Uid(sb.Id, sb.Type)
}

func (sb *BaseShortcut) GetKeystrokes() []*Keystroke {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.Keystrokes
}

func (sb *BaseShortcut) getKeystrokesStrv() []string {
	keystrokes := sb.GetKeystrokes()
	strv := make([]string, len(keystrokes))
	for i, k := range keystrokes {
		strv[i] = k.String()
	}
	return strv
}

func (sb *BaseShortcut) setKeystrokes(val []*Keystroke) {
	sb.mu.Lock()
	sb.Keystrokes = val
	sb.mu.Unlock()
}

func (sb *BaseShortcut) GetType() int32 {
	return sb.Type
}

func (sb *BaseShortcut) GetName() string {
	return sb.Name
}

func (sb *BaseShortcut) GetAction() *Action {
	return ActionNoOp
}

func (sb *BaseShortcut) GetKeystrokesModifiable() bool {
	return sb.Type != ShortcutTypeFake
}

func (sb *BaseShortcut) ShouldEmitSignalChanged() bool {
	return false
}

const (
	ShortcutTypeSystem int32 = iota
	ShortcutTypeCustom
	ShortcutTypeMedia
	ShortcutTypeWM
	ShortcutTypeFake
)

type Shortcut interface {
	GetId() string
	GetUid() string
	GetType() int32

	GetName() string

	GetKeystrokesModifiable() bool
	GetKeystrokes() []*Keystroke
	setKeystrokes([]*Keystroke)
	SaveKeystrokes() error
	ReloadKeystrokes() bool

	GetAction() *Action
	Marshal() (string, error)
	ShouldEmitSignalChanged() bool
}

// errors:
var ErrOpNotSupported = errors.New("operation is not supported")
var ErrTypeAssertionFail = errors.New("type assertion failed")
var ErrNilAction = errors.New("action is nil")
var ErrInvalidActionType = errors.New("invalid action type")

type FakeShortcut struct {
	BaseShortcut
	action *Action
}

func NewFakeShortcut(action *Action) *FakeShortcut {
	return &FakeShortcut{
		BaseShortcut: BaseShortcut{
			Type: ShortcutTypeFake,
		},
		action: action,
	}
}

func (s *FakeShortcut) GetAction() *Action {
	return s.action
}

func (s *FakeShortcut) SaveKeystrokes() error {
	return ErrOpNotSupported
}

func (s *FakeShortcut) ReloadKeystrokes() bool {
	return false
}
