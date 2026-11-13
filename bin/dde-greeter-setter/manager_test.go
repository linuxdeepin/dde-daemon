// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManager_GetInterfaceName(t *testing.T) {
	m := Manager{}
	assert.Equal(t, dbusInterface, m.GetInterfaceName())
}

func Test_isInteger(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "isInteger",
			args: args{
				str: "123",
			},
			want: true,
		},
		{
			name: "isInteger false",
			args: args{
				str: "123a",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInteger(tt.args.str)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_quoteXSettingsValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"plain name", "Noto Sans", `"Noto Sans"`},
		{"empty", "", `""`},
		{"quote cannot terminate the string", `a"b`, ""},
		{"backslash cannot escape the quote", `a\"b`, ""},
		{"newline cannot inject a key", "a\nNet/IconThemeName \"x\"", ""},
		{"carriage return", "a\rb", ""},
		{"tab", "a\tb", ""},
		{"del", "a\x7fb", ""},
		{"unicode is preserved", "文泉驿", `"文泉驿"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, quoteXSettingsValue(tt.value))
		})
	}
}

func Test_updateXSettingsConfigInjectionBlocked(t *testing.T) {
	dir := t.TempDir()
	themePath := filepath.Join(dir, "qt-theme.ini")
	xsettingsPath := filepath.Join(dir, "xsettingsd.conf")

	// The Font value contains a quote followed by a complete injected key.
	// updateXSettingsConfig must skip this value instead of rendering it.
	theme := `[Theme]
IconThemeName=Adwaita
FontSize=10
Font=Noto Sans" Net/IconThemeName "attacker
MonoFont=Noto Mono
ScreenScaleFactors=1
ScaleLogicalDpi=96,96
`
	assert.NoError(t, os.WriteFile(themePath, []byte(theme), 0600))

	assert.NoError(t, updateXSettingsConfigFiles(themePath, xsettingsPath))
	got, err := os.ReadFile(xsettingsPath)
	assert.NoError(t, err)
	assert.NotContains(t, string(got), `Net/IconThemeName "attacker"`)
	assert.Contains(t, string(got), `Net/IconThemeName "Adwaita"`)
}

func Test_updateXSettingsConfigMissingDPI(t *testing.T) {
	dir := t.TempDir()
	themePath := filepath.Join(dir, "qt-theme.ini")
	xsettingsPath := filepath.Join(dir, "xsettingsd.conf")

	theme := `[Theme]
IconThemeName=Adwaita
FontSize=10
`
	assert.NoError(t, os.WriteFile(themePath, []byte(theme), 0600))
	assert.Error(t, updateXSettingsConfigFiles(themePath, xsettingsPath))
}

func Test_updateXSettingsConfigOptionalKeys(t *testing.T) {
	dir := t.TempDir()
	themePath := filepath.Join(dir, "qt-theme.ini")
	xsettingsPath := filepath.Join(dir, "xsettingsd.conf")

	theme := `[Theme]
IconThemeName=Adwaita
ScreenScaleFactors=1
ScaleLogicalDpi=96,96
`
	assert.NoError(t, os.WriteFile(themePath, []byte(theme), 0600))
	assert.NoError(t, updateXSettingsConfigFiles(themePath, xsettingsPath))
	got, err := os.ReadFile(xsettingsPath)
	assert.NoError(t, err)
	assert.Contains(t, string(got), `Net/IconThemeName "Adwaita"`)
}
