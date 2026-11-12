// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
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
	// The rendered line must never contain a second key/value pair, no
	// matter what the caller put into the theme file.
	value := "Sans\"\nNet/IconThemeName \"attacker\"\nQt/FontName \"" + "x"
	quoted := quoteXSettingsValue(value)
	assert.Empty(t, quoted)
	if quoted != "" {
		assert.NotContains(t, fmt.Sprintf("%s %s\n", xsettingsKeyFont, quoted), "\nNet/")
	}
}
