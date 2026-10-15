// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package shortcuts

import (
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-x11-client/util/keysyms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mediaKeyList struct{ proxy.PropStringArray }

func (*mediaKeyList) Get(dbus.Flags) ([]string, error) {
	return []string{"mail"}, nil
}

type mediaConfigStub struct {
	configManager.Manager
	value  dbus.Variant
	err    error
	writes int
}

func (*mediaConfigStub) KeyList() proxy.PropStringArray { return &mediaKeyList{} }

func (c *mediaConfigStub) Value(dbus.Flags, string) (dbus.Variant, error) {
	return c.value, c.err
}

func (c *mediaConfigStub) SetValue(dbus.Flags, string, dbus.Variant) error {
	c.writes++
	return nil
}

func (c *mediaConfigStub) Reset(dbus.Flags, string) error {
	c.writes++
	return nil
}

func TestAddMediaReadsDConfig(t *testing.T) {
	oldLogger := logger
	SetLogger(log.NewLogger("test/keybinding"))
	t.Cleanup(func() { SetLogger(oldLogger) })
	for _, tc := range []struct {
		name    string
		value   interface{}
		err     error
		want    []string
		present bool
	}{
		{"string array", []string{"XF86Mail"}, nil, []string{"XF86Mail"}, true},
		{"variant array", []dbus.Variant{dbus.MakeVariant("XF86Mail")}, nil, []string{"XF86Mail"}, true},
		{"empty array", []dbus.Variant{}, nil, []string{}, true},
		{"custom binding", []string{"<Super>m"}, nil, []string{"<Super>m"}, true},
		{"read error", []string{"XF86Mail"}, errors.New("config unavailable"), nil, false},
		{"invalid type", true, nil, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &mediaConfigStub{value: dbus.MakeVariant(tc.value), err: tc.err}
			sm := &ShortcutManager{
				shortcutMediaConfigMgr: config,
				idShortcutMap:          make(map[string]Shortcut),
				keySymbols:             &keysyms.KeySymbols{},
			}
			sm.AddMedia(nil)
			shortcut := sm.GetByIdType("mail", ShortcutTypeMedia)
			if tc.present {
				require.NotNil(t, shortcut)
				assert.Equal(t, tc.want, shortcut.getKeystrokesStrv())
			} else {
				assert.Nil(t, shortcut)
			}
			assert.Zero(t, config.writes, "loading shortcuts must not rewrite DConfig")
			assert.Empty(t, sm.ConflictingKeystrokes)
		})
	}
}

func TestGrabUnmappedMediaKeystroke(t *testing.T) {
	oldLogger := logger
	SetLogger(log.NewLogger("test/keybinding"))
	t.Cleanup(func() { SetLogger(oldLogger) })

	// An unavailable keyboard map produces no keycodes, without an error.
	// Such a shortcut must not enter the list that is deleted from DConfig.
	sm := &ShortcutManager{keySymbols: &keysyms.KeySymbols{}}
	shortcut := &MediaShortcut{
		ShortcutObject: NewShortcut(nil, "mail", ShortcutTypeMedia,
			[]string{"XF86Mail"}, "Mail", nil, nil),
	}

	sm.grabShortcut(shortcut)

	assert.Empty(t, sm.ConflictingKeystrokes)
	assert.Equal(t, []string{"XF86Mail"}, shortcut.getKeystrokesStrv())
}

func Test_systemType(t *testing.T) {

	var versions = []string{
		"Community", "Professional", "Home",
		"Server", "Personal", "Desktop",
		"",
	}

	assert.Contains(t, versions, systemType())
}

func Test_arr2set(t *testing.T) {
	testData := []struct {
		input  []string
		output map[string]bool
	}{
		{
			input: []string{"key1", "key2", "key3"},
			output: map[string]bool{
				"key1": true,
				"key2": true,
				"key3": true,
			},
		},
	}

	for _, data := range testData {
		assert.Equal(t, data.output, arr2set(data.input))
	}

}

func Test_strvLower(t *testing.T) {
	testData := []struct {
		in  []string
		out []string
	}{
		{
			in:  []string{"What's", "Your", "Problem"},
			out: []string{"what's", "your", "problem"},
		},
	}

	for _, data := range testData {
		assert.Equal(t, data.out, strvLower(data.in))
	}
}
