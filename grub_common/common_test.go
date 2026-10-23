// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub_common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeShellValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain", "auto", "auto"},
		{"double quoted", `"deepin"`, "deepin"},
		{"single quoted", `'deepin'`, "deepin"},
		{"quoted path", `"/boot/grub/themes/deepin/theme.txt"`, "/boot/grub/themes/deepin/theme.txt"},
		{"default entry", "0", "0"},
		{"quoted spaces", `"quiet splash"`, "quiet splash"},
		{"unquoted spaces kept", "quiet   splash", "quiet   splash"},
		{"escaped space", `a\ b`, "a b"},
		{"concatenated quotes", `"a"'b'c`, "abc"},
		{"concatenated path", `"/boot/grub"/themes`, "/boot/grub/themes"},
		{"quoted with trailing text", `a"b"c`, "abc"},
		{"empty quotes", `""`, ""},
		{"escaped backslash in quotes", `"a\\b"`, `a\b`},
		{"escaped double quote", `"a\"b"`, `a"b`},
		{"single quote keeps double quote", `'a"b'`, `a"b`},
		{"escaped dollars", "\\$HOME", "$HOME"},
		{"backslash newline in quotes", "\"line1\\\nline2\"", "line1\nline2"},
		{"backslash newline unquoted", "a\\\nb", "a\nb"},
		{"trailing backslash", `a\`, `a\`},
		{"dollar not expanded", `$HOME`, "$HOME"},
		{"dollar brace not expanded", `"${HOME}"`, "${HOME}"},
		{"command substitution is literal", `$(touch /tmp/never-created)`, "$(touch /tmp/never-created)"},
		{"backtick is literal", "\"`id`\"", "`id`"},
		{"hash is not a comment", "foo #comment", "foo #comment"},
		{"hash inside word", "foo#bar", "foo#bar"},
		{"distributor fixture", "`( . /etc/os-release; echo ${NAME:-Deepin} ) 2>/dev/null || echo Deepin`", "`( . /etc/os-release; echo ${NAME:-Deepin} ) 2>/dev/null || echo Deepin`"},
		{"deepin cfg fixture", `"$GRUB_CMDLINE_LINUX_DEFAULT DEEPIN_GFXMODE=\$DEEPIN_GFXMODE"`, `$GRUB_CMDLINE_LINUX_DEFAULT DEEPIN_GFXMODE=$DEEPIN_GFXMODE`},
		{"unterminated double quote fallback", `"abc`, "abc"},
		{"unterminated single quote fallback", `'abc`, `'abc`},
		{"words around quotes", `a "b c" d`, "a b c d"},
		{"single quote keeps backslash", `'a\nb'`, `a\nb`},
		{"single quote keeps dollar", `'$HOME'`, `$HOME`},
		{"escaped single quote outside quotes", `a\'b`, "a'b"},
		{"escaped single quote inside double quotes", `"a\'b"`, "a'b"},
		{"unescaped double quotes inside single quotes", `'a"b"c'`, `a"b"c`},
		{"hash inside quotes", `"a#b"`, "a#b"},
		{"newline inside quotes kept", "\"a\nb\"", "a\nb"},
		{"empty quoted segments", `""''`, ""},
		{"trailing backslash in unterminated quote", `"a\`, `a\`},
		{"multiple escaped characters", `a\ b\ c`, "a b c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DecodeShellValue(tt.in))
		})
	}
}

func TestDecodeShellValueDoesNotExecute(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pwned")
	payloads := []string{
		"; touch " + marker + " ;",
		"$(touch " + marker + ")",
		"`touch " + marker + "`",
		`"$(touch ` + marker + `)"`,
		`'; touch ` + marker + ` ;'`,
		`\"; touch ` + marker + ` ; echo \"`,
	}
	for _, in := range payloads {
		out := DecodeShellValue(in)
		_, err := os.Stat(marker)
		if !os.IsNotExist(err) {
			t.Fatalf("input %q executed: output %q, stat err %v", in, out, err)
		}
		assert.Contains(t, out, marker, "shell syntax should be preserved as data")
	}
}

// readGrubParamsFile keeps the raw value, so the quoted form reaches
// DecodeShellValue exactly as written in the config file.
func TestReadGrubParamsFileKeepsQuotes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "grub")
	content := "# comment\n" +
		"GRUB_TIMEOUT=\"5\"\n" +
		"GRUB_THEME='/boot/grub/themes/deepin/theme.txt'\n" +
		"GRUB_CMDLINE_LINUX_DEFAULT=\"quiet splash\"\n" +
		"GRUB_DISTRIBUTOR=`( . /etc/os-release; echo ${NAME:-Deepin} ) 2>/dev/null || echo Deepin`\n" +
		"\n" +
		"not-a-pair\n"
	assert.NoError(t, os.WriteFile(path, []byte(content), 0644))

	params := make(map[string]string)
	assert.NoError(t, readGrubParamsFile(path, params))

	assert.Equal(t, `"5"`, params["GRUB_TIMEOUT"])
	assert.Equal(t, `'/boot/grub/themes/deepin/theme.txt'`, params["GRUB_THEME"])
	assert.Equal(t, `"quiet splash"`, params["GRUB_CMDLINE_LINUX_DEFAULT"])
	assert.NotContains(t, params, "not-a-pair")

	assert.Equal(t, "5", DecodeShellValue(params["GRUB_TIMEOUT"]))
	assert.Equal(t, "/boot/grub/themes/deepin/theme.txt", DecodeShellValue(params["GRUB_THEME"]))
	assert.Equal(t, "quiet splash", DecodeShellValue(params["GRUB_CMDLINE_LINUX_DEFAULT"]))
	assert.Equal(t,
		"`( . /etc/os-release; echo ${NAME:-Deepin} ) 2>/dev/null || echo Deepin`",
		DecodeShellValue(params["GRUB_DISTRIBUTOR"]))
}

func TestReadGrubParamsFileMissing(t *testing.T) {
	params := make(map[string]string)
	assert.NoError(t, readGrubParamsFile(filepath.Join(t.TempDir(), "absent"), params))
	assert.Empty(t, params)
}

func TestGfxmodesSortDesc(t *testing.T) {
	gfxmodes := Gfxmodes{
		{1366, 768},
		{800, 600},
		{1360, 768},
		{1024, 768},
		{960, 720},
		{1024, 576},
	}
	gfxmodes.SortDesc()

	assert.Equal(t, Gfxmodes{
		{1366, 768},
		{1360, 768},
		{1024, 768},
		{960, 720},
		{1024, 576},
		{800, 600},
	}, gfxmodes)
}

func TestGfxmodesMax(t *testing.T) {
	gfxmodes := Gfxmodes{
		{800, 600},
		{1360, 768},
		{1024, 768},
		{1366, 768},
		{960, 720},
		{1024, 576},
	}
	assert.Equal(t, Gfxmode{1366, 768}, gfxmodes.Max())
	assert.Equal(t, Gfxmode{}, Gfxmodes(nil).Max())
}

func TestParseGfxmode(t *testing.T) {
	mode, err := ParseGfxmode("1024x768")
	assert.NoError(t, err)
	assert.Equal(t, Gfxmode{1024, 768}, mode)

	_, err = ParseGfxmode("auto")
	assert.Error(t, err)

	_, err = ParseGfxmode("1024x768x32")
	assert.Error(t, err)
}

func Test_parseBootArgDeepinGfxmode(t *testing.T) {
	cur, all, err := parseBootArgDeepinGfxmode("1,1280x1024,1366x768,1024x768")
	assert.NoError(t, err)
	assert.Equal(t, cur, Gfxmode{1366, 768})
	assert.Equal(t, all, Gfxmodes{
		{1280, 1024},
		{1366, 768},
		{1024, 768},
	})

	cur, all, err = parseBootArgDeepinGfxmode("0,1280x1024")
	assert.NoError(t, err)
	assert.Equal(t, cur, Gfxmode{1280, 1024})
	assert.Equal(t, all, Gfxmodes{
		{1280, 1024},
	})

	_, _, err = parseBootArgDeepinGfxmode("")
	assert.Error(t, err)

	_, _, err = parseBootArgDeepinGfxmode("3,1280x1024,1366x768,1024x768")
	assert.Error(t, err)

	_, _, err = parseBootArgDeepinGfxmode("-1,1280x1024,1366x768,1024x768")
	assert.Error(t, err)

	_, _, err = parseBootArgDeepinGfxmode("1,1280x1024,1366x768,1024x768,auto")
	assert.Error(t, err)
}
