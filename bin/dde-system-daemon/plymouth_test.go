// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
)

// realThemeList mirrors the output of `plymouth-set-default-theme --list` on a
// UOS desktop: one theme name per line.
const realThemeList = `deepin-hidpi-logo
deepin-hidpi-ssd-logo
deepin-logo
deepin-ssd-logo
details
text
tribar
uos-hidpi-ssd-logo
uos-ssd-logo
`

func TestIsValidPlymouthThemeName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"uos-ssd-logo", true},
		{"deepin_hidpi.logo", true},
		{"Text2", true},
		{"", false},
		{"-l", false},
		{"--list", false},
		{"--rebuild-initrd", false},
		{".", false},
		{"..", false},
		{"../uos-ssd-logo", false},
		{"uos-ssd-logo/../text", false},
		{"/usr/share/plymouth/themes/text", false},
		{"uos ssd logo", false},
		{"uos-ssd-logo\n--list", false},
		{"uos-ssd-logo;id", false},
		{"uos-ssd-logo$(id)", false},
		{strings.Repeat("a", 64), true},
		{strings.Repeat("a", 65), false},
	}

	for _, test := range tests {
		if got := isValidPlymouthThemeName(test.name); got != test.valid {
			t.Errorf("isValidPlymouthThemeName(%q) = %v, want %v", test.name, got, test.valid)
		}
	}
}

func TestCheckPlymouthThemeName(t *testing.T) {
	tests := []struct {
		name      string
		themeName string
		wantErr   bool
	}{
		{"exact name", "uos-ssd-logo", false},
		{"exact name on a middle line", "details", false},
		{"trailing newline is not part of the name", "uos-ssd-logo", false},
		{"substring of a theme name", "logo", true},
		{"prefix of a theme name", "uos-ssd", true},
		{"empty name", "", true},
		{"name assembled from two entries", "text\ntribar", true},
		{"option passed as a theme", "--list", true},
	}

	for _, test := range tests {
		err := checkPlymouthThemeName(realThemeList, test.themeName)
		if (err != nil) != test.wantErr {
			t.Errorf("%s: checkPlymouthThemeName(%q) error = %v, wantErr %v",
				test.name, test.themeName, err, test.wantErr)
		}
	}
}

func TestCheckPlymouthThemeNameEmptyList(t *testing.T) {
	if err := checkPlymouthThemeName("", "text"); err == nil {
		t.Error("checkPlymouthThemeName accepted a theme when no theme was listed")
	}
}
