// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package keybinding

import (
	"testing"

	"github.com/linuxdeepin/dde-daemon/keybinding1/shortcuts"
)

func TestX11MailLaunch(t *testing.T) {
	m := &Manager{handlers: make([]shortcuts.KeyEventFunc, shortcuts.ActionTypeCount)}
	called := false
	m.handlers[shortcuts.ActionTypeLaunchMimeType] = func(ev *shortcuts.KeyEvent) {
		called = true
		if action := ev.Shortcut.GetAction(); action.Arg != "x-scheme-handler/mailto" {
			t.Fatalf("unexpected Mail action: %+v", action)
		}
	}

	shortcut := &shortcuts.MediaShortcut{
		ShortcutObject: shortcuts.NewShortcut(nil, "mail", shortcuts.ShortcutTypeMedia,
			[]string{"XF86Mail"}, "Mail", nil, nil),
	}
	m.handleKeyEvent(&shortcuts.KeyEvent{Shortcut: shortcut})
	if !called {
		t.Fatal("Mail did not reach the MIME application launch handler")
	}
}
