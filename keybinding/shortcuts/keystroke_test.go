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
	"testing"

	"github.com/linuxdeepin/go-x11-client/util/keysyms"
	"github.com/stretchr/testify/assert"
)

func TestSplitKeystroke(t *testing.T) {
	var keys []string
	var err error
	keys, err = splitKeystroke("<Super>L")
	assert.Nil(t, err)
	assert.ElementsMatch(t, keys, []string{"Super", "L"})

	// single key
	keys, err = splitKeystroke("<Super>")
	assert.Nil(t, err)
	assert.ElementsMatch(t, keys, []string{"Super"})

	keys, err = splitKeystroke("Super_L")
	assert.Nil(t, err)
	assert.ElementsMatch(t, keys, []string{"Super_L"})

	keys, err = splitKeystroke("<Shift><Super>T")
	assert.Nil(t, err)
	assert.ElementsMatch(t, keys, []string{"Shift", "Super", "T"})

	// abnormal situation:
	_, err = splitKeystroke("<Super>>")
	assert.NotNil(t, err)

	_, err = splitKeystroke("<Super><")
	assert.NotNil(t, err)

	_, err = splitKeystroke("Super<")
	assert.NotNil(t, err)

	_, err = splitKeystroke("<Super><shiftT")
	assert.NotNil(t, err)

	_, err = splitKeystroke("<Super><Shift><>T")
	assert.NotNil(t, err)
}

func TestParseKeystroke(t *testing.T) {
	var ks *Keystroke
	var err error

	ks, err = ParseKeystroke("Super_L")
	assert.Nil(t, err)
	assert.Equal(t, ks, &Keystroke{
		Keystr: "Super_L",
		Keysym: keysyms.XK_Super_L,
	})

	ks, err = ParseKeystroke("Num_Lock")
	assert.Nil(t, err)
	assert.Equal(t, ks, &Keystroke{
		Keystr: "Num_Lock",
		Keysym: keysyms.XK_Num_Lock,
	})

	ks, err = ParseKeystroke("<Control><Super>T")
	assert.Nil(t, err)
	assert.Equal(t, ks, &Keystroke{
		Keystr: "T",
		Keysym: keysyms.XK_T,
		Mods:   keysyms.ModMaskSuper | keysyms.ModMaskControl,
	})

	ks, err = ParseKeystroke("<Control><Alt><Shift><Super>T")
	assert.Nil(t, err)
	assert.Equal(t, ks, &Keystroke{
		Keystr: "T",
		Keysym: keysyms.XK_T,
		Mods:   keysyms.ModMaskShift | keysyms.ModMaskSuper | keysyms.ModMaskAlt | keysyms.ModMaskControl,
	})

	// abnormal situation:
	_, err = ParseKeystroke("<Shift>XXXXX")
	assert.NotNil(t, err)

	_, err = ParseKeystroke("")
	assert.NotNil(t, err)

	_, err = ParseKeystroke("<lock><Shift>A")
	assert.NotNil(t, err)
}

func TestParseKeystrokes(t *testing.T) {
	keystrokes := []string{
		"<Super>S", "<Control>C",
		"<Alt>A", "<Control><Alt>V",
	}
	ret := ParseKeystrokes(keystrokes)
	assert.Equal(t, len(ret), len(keystrokes))
}

func TestKeystrokeMethodString(t *testing.T) {
	var ks Keystroke
	ks = Keystroke{
		Keystr: "percent",
		Mods:   keysyms.ModMaskControl | keysyms.ModMaskShift,
	}
	assert.Equal(t, ks.String(), "<Shift><Control>percent")

	ks = Keystroke{
		Keystr: "T",
		Mods:   keysyms.ModMaskShift | keysyms.ModMaskSuper | keysyms.ModMaskAlt | keysyms.ModMaskControl,
	}
	assert.Equal(t, ks.String(), "<Shift><Control><Alt><Super>T")
}

func TestParseLoopback(t *testing.T) {
	ks, err := ParseKeystroke("<SHIFT><CONTROL><ALT><SUPER>T")
	assert.Nil(t, err)
	assert.Equal(t, ks.String(), "<Shift><Control><Alt><Super>T")

	ks, err = ParseKeystroke("<shift><control><alt><super>t")
	assert.Nil(t, err)
	assert.Equal(t, ks.String(), "<Shift><Control><Alt><Super>t")
}

func TestParseMediaKey(t *testing.T) {
	keys := []string{
		"XF86Messenger",
		"XF86Save",
		"XF86New",
		"XF86WakeUp",
		"XF86AudioRewind",
		"XF86AudioMute",
		"XF86MonBrightnessUp",
		"XF86WLAN",
		"XF86AudioMedia",
		"XF86Reply",
		"XF86Favorites",
		"XF86AudioPlay",
		"XF86AudioMicMute",
		"XF86AudioPause",
		"XF86AudioStop",
		"XF86PowerOff",
		"XF86Documents",
		"XF86Game",
		"XF86Search",
		"XF86AudioRecord",
		"XF86Display",
		"XF86Reload",
		"XF86Explorer",
		"XF86Calculator",
		"XF86Calendar",
		"XF86Forward",
		"XF86Cut",
		"XF86MonBrightnessDown",
		"XF86Copy",
		"XF86Tools",
		"XF86AudioRaiseVolume",
		"XF86Close",
		"XF86WWW",
		"XF86HomePage",
		"XF86Sleep",
		"XF86AudioLowerVolume",
		"XF86AudioPrev",
		"XF86AudioNext",
		"XF86Paste",
		"XF86Open",
		"XF86Send",
		"XF86MyComputer",
		"XF86Mail",
		"XF86BrightnessAdjust",
		"XF86LogOff",
		"XF86Pictures",
		"XF86Terminal",
		"XF86Video",
		"XF86Music",
		"XF86ApplicationLeft",
		"XF86ApplicationRight",
		"XF86Meeting",
	}

	for _, key := range keys {
		ks, err := ParseKeystroke(key)
		assert.Nil(t, err)
		assert.Equal(t, ks.String(), key)
	}
}
