// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGrubParamsDecodeQuotedValues(t *testing.T) {
	params := map[string]string{
		grubTimeout: `"7"`,
		grubGfxmode: `'1024x768'`,
		grubDefault: `"deepin"`,
		grubTheme:   `"/boot/grub/themes/deepin/theme.txt"`,
	}
	assert.Equal(t, 7, getTimeout(params))
	assert.Equal(t, "1024x768", getGfxMode(params))
	assert.Equal(t, "deepin", getDefaultEntry(params))
	assert.Equal(t, "/boot/grub/themes/deepin/theme.txt", getTheme(params))
}

func TestGrubParamsDoNotExecuteShellSyntax(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pwned")
	params := map[string]string{
		grubTheme:   `"$(touch ` + marker + `)"`,
		grubGfxmode: "`touch " + marker + "`",
		grubDefault: "touch " + marker,
		grubTimeout: "1; touch " + marker,
	}
	assert.Contains(t, getTheme(params), marker)
	assert.Contains(t, getGfxMode(params), marker)
	assert.Contains(t, getDefaultEntry(params), marker)
	assert.Equal(t, defaultGrubTimeoutInt, getTimeout(params))

	_, err := os.Stat(marker)
	assert.True(t, os.IsNotExist(err), "shell syntax must not be executed: %v", err)
}

// Values written without quotes must keep working, they are the common case
// in distribution shipped config files.
func TestGrubParamsDecodeUnquotedValues(t *testing.T) {
	params := map[string]string{
		grubTimeout: "10",
		grubGfxmode: "auto",
		grubDefault: "0",
		grubTheme:   "/boot/grub/themes/deepin/theme.txt",
	}
	assert.Equal(t, 10, getTimeout(params))
	assert.Equal(t, "auto", getGfxMode(params))
	assert.Equal(t, "0", getDefaultEntry(params))
	assert.Equal(t, "/boot/grub/themes/deepin/theme.txt", getTheme(params))
}

func TestGrubParamsFallbackOnMissingOrBadValues(t *testing.T) {
	// absent keys fall back to the built-in defaults
	empty := map[string]string{}
	assert.Equal(t, defaultGrubTimeoutInt, getTimeout(empty))
	assert.Equal(t, defaultGrubGfxMode, getGfxMode(empty))
	assert.Equal(t, defaultGrubDefault, getDefaultEntry(empty))
	assert.Equal(t, "", getTheme(empty))

	// a non numeric timeout falls back instead of failing
	assert.Equal(t, defaultGrubTimeoutInt, getTimeout(map[string]string{grubTimeout: `"abc"`}))
	assert.Equal(t, defaultGrubTimeoutInt, getTimeout(map[string]string{grubTimeout: `"5 5"`}))

	// an empty quoted value is empty, so the defaults apply
	assert.Equal(t, defaultGrubGfxMode, getGfxMode(map[string]string{grubGfxmode: `""`}))
	assert.Equal(t, defaultGrubDefault, getDefaultEntry(map[string]string{grubDefault: `""`}))
}
