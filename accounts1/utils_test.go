// SPDX-FileCopyrightText: 2018-2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
)

var str = []string{"/bin/sh", "/bin/bash",
	"/bin/zsh", "/usr/bin/zsh",
	"/usr/bin/fish",
}

func Test_GetLocaleFromFile(t *testing.T) {
	assert.Equal(t, getLocaleFromFile("testdata/locale"), "zh_CN.UTF-8")
}

func Test_SystemLayout(t *testing.T) {
	layout, err := getSystemLayout("testdata/keyboard_us")
	assert.NoError(t, err)
	assert.Equal(t, layout, "us;")
	layout, _ = getSystemLayout("testdata/keyboard_us_chr")
	assert.Equal(t, layout, "us;chr")
}

func TestAvailableShells(t *testing.T) {
	var ret = []string{"/bin/sh", "/bin/bash",
		"/bin/zsh", "/usr/bin/zsh",
		"/usr/bin/fish",
	}
	shells := getAvailableShells("testdata/shells")
	assert.ElementsMatch(t, shells, ret)
}

func TestIsStrInArray(t *testing.T) {
	ret := isStrInArray("testdata/shells", str)
	assert.Equal(t, ret, false)
	ret = isStrInArray("/bin/sh", str)
	assert.Equal(t, ret, true)
}

func TestIsStrvEqual(t *testing.T) {
	var str1 = []string{"/bin/sh", "/bin/bash",
		"/bin/zsh", "/usr/bin/zsh",
		"/usr/bin/fish",
	}
	var str2 = []string{"/bin/sh", "/bin/bash"}
	ret := isStrvEqual(str, str1)
	assert.Equal(t, ret, true)
	ret = isStrvEqual(str, str2)
	assert.Equal(t, ret, false)
}

func TestGetValueFromLine(t *testing.T) {
	ret := getValueFromLine("testdata/shells", "/")
	assert.Equal(t, ret, "shells")
}

func TestRecoverOwnershipDoesNotDereferenceSymlink(t *testing.T) {
	homeDir := t.TempDir()
	linkPath := filepath.Join(homeDir, "dde-computer.desktop")
	missingTarget := filepath.Join(homeDir, "missing-target")
	if err := os.Symlink(missingTarget, linkPath); err != nil {
		t.Fatal(err)
	}

	user := &User{
		Uid:     strconv.Itoa(os.Getuid()),
		Gid:     strconv.Itoa(os.Getgid()),
		HomeDir: homeDir,
	}
	if err := recoverOwnership(user); err != nil {
		t.Fatalf("recoverOwnership followed the symbolic link: %v", err)
	}
}

func Test_getDetailsKey(t *testing.T) {
	type args struct {
		details map[string]dbus.Variant
		key     string
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "getDetailsKey",
			args: args{
				details: map[string]dbus.Variant{
					"te": dbus.MakeVariant(true),
				},
				key: "te",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "getDetailsKey not found",
			args: args{
				details: map[string]dbus.Variant{
					"te": dbus.MakeVariant(true),
				},
				key: "te1",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDetailsKey(tt.args.details, tt.args.key)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
