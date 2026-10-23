// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package lastore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseActiveWindowID(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{name: "valid window", output: "_NET_ACTIVE_WINDOW(WINDOW): window id # 0x3600007", want: "0x3600007"},
		{name: "no active window", output: "_NET_ACTIVE_WINDOW(WINDOW): window id # 0x0", wantErr: true},
		{name: "empty output", output: "", wantErr: true},
		{name: "missing atom", output: "_NET_ACTIVE_WINDOW:  no such atom on any window.", wantErr: true},
		{name: "invalid hex", output: "_NET_ACTIVE_WINDOW(WINDOW): window id # 0xZZ", wantErr: true},
		{name: "shell metacharacter", output: "_NET_ACTIVE_WINDOW(WINDOW): window id # 0x3600007;id", wantErr: true},
		{name: "missing digits", output: "_NET_ACTIVE_WINDOW(WINDOW): window id # 0x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseActiveWindowID([]byte(tt.output))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// installFakeXprop puts an executable named "xprop" first in PATH. It records
// its arguments and echoes the configured output, exercising the real process
// invocation directly.
func installFakeXprop(t *testing.T, rootOutput, idOutput string) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$XPROP_LOG"
if [ "$1" = "-root" ]; then
	if [ -n "$XPROP_ROOT_EXIT" ]; then
		exit "$XPROP_ROOT_EXIT"
	fi
	printf '%s' "$XPROP_ROOT_OUTPUT"
elif [ "$1" = "-id" ]; then
	if [ -n "$XPROP_ID_EXIT" ]; then
		exit "$XPROP_ID_EXIT"
	fi
	printf '%s' "$XPROP_ID_OUTPUT"
fi
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "xprop"), []byte(script), 0755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XPROP_LOG", logPath)
	t.Setenv("XPROP_ROOT_OUTPUT", rootOutput)
	t.Setenv("XPROP_ID_OUTPUT", idOutput)
	t.Setenv("XPROP_ROOT_EXIT", "")
	t.Setenv("XPROP_ID_EXIT", "")
	return logPath
}

func Test_getActiveWindowWMClass(t *testing.T) {
	logPath := installFakeXprop(t,
		"_NET_ACTIVE_WINDOW(WINDOW): window id # 0x3600007",
		"WM_CLASS(STRING) = \"dde-control-center\", \"Dde-control-center\"\n")

	wmClass, err := getActiveWindowWMClass()
	require.NoError(t, err)
	assert.Contains(t, wmClass, "dde-control-center")

	args, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Equal(t, "-root _NET_ACTIVE_WINDOW\n-id 0x3600007 WM_CLASS\n", string(args))
}

func Test_getActiveWindowWMClassNoActiveWindow(t *testing.T) {
	logPath := installFakeXprop(t,
		"_NET_ACTIVE_WINDOW(WINDOW): window id # 0x0",
		"WM_CLASS(STRING) = \"should-not-be-used\"\n")

	wmClass, err := getActiveWindowWMClass()
	assert.Error(t, err)
	assert.Empty(t, wmClass)

	args, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Equal(t, "-root _NET_ACTIVE_WINDOW\n", string(args))
}

func Test_getActiveWindowWMClassIDQueryFails(t *testing.T) {
	installFakeXprop(t,
		"_NET_ACTIVE_WINDOW(WINDOW): window id # 0x3600007",
		"WM_CLASS(STRING) = \"dde-lock\", \"Dde-lock\"\n")
	t.Setenv("XPROP_ID_EXIT", "1")

	wmClass, err := getActiveWindowWMClass()
	assert.Error(t, err)
	assert.Empty(t, wmClass)
}
