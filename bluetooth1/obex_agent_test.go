// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package bluetooth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Normal filenames - should pass
		{"normal jpg", "photo.jpg", false},
		{"normal pdf", "document.pdf", false},
		{"with spaces", "my document.pdf", false},
		{"with dots", "file.name.txt", false},
		{"with underscores", "my_file.txt", false},
		{"with hyphens", "my-file.txt", false},
		{"chinese filename", "照片.jpg", false},
		{"long name", "a]very_long_filename_with_many_characters.txt", false},

		// Path traversal attacks - should reject
		{"relative traversal", "../.config/autostart/evil.desktop", true},
		{"double traversal", "../../etc/passwd", true},
		{"triple traversal", "../../../tmp/evil", true},
		{"absolute path unix", "/etc/passwd", true},
		{"absolute path home", "/home/user/.bashrc", true},
		{"windows path backslash", "..\\windows\\system32", true},
		{"windows path forward", "..\\..\\windows\\system32", true},
		{"mixed separators", "../foo/bar", true},
		{"separator in middle", "foo/bar", true},
		{"backslash in middle", "foo\\bar", true},

		// Edge cases - should reject
		{"empty string", "", true},
		{"just dot", ".", true},
		{"just dotdot", "..", true},
		{"just slash", "/", true},
		{"just backslash", "\\", true},
		{"dot with slash", "./", true},
		{"dotdot with slash", "../", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilename(tt.input)
			if tt.wantErr {
				assert.Error(t, err, "expected error for input: %q", tt.input)
			} else {
				assert.NoError(t, err, "unexpected error for input: %q", tt.input)
			}
		})
	}
}

func TestValidateFilenameSecurity(t *testing.T) {
	// Specific security-focused tests

	t.Run("prevents path traversal to autostart", func(t *testing.T) {
		err := validateFilename("../.config/autostart/evil.desktop")
		assert.Error(t, err)
	})

	t.Run("prevents path traversal to ssh", func(t *testing.T) {
		err := validateFilename("../../.ssh/authorized_keys")
		assert.Error(t, err)
	})

	t.Run("prevents absolute path to sensitive file", func(t *testing.T) {
		err := validateFilename("/etc/passwd")
		assert.Error(t, err)
	})

	t.Run("prevents windows style path traversal", func(t *testing.T) {
		err := validateFilename("..\\..\\windows\\system32\\config\\sam")
		assert.Error(t, err)
	})

	t.Run("accepts normal filename", func(t *testing.T) {
		err := validateFilename("vacation_photo.jpg")
		assert.NoError(t, err)
	})

	t.Run("accepts filename with spaces", func(t *testing.T) {
		err := validateFilename("my vacation photo.jpg")
		assert.NoError(t, err)
	})

	t.Run("accepts filename with unicode", func(t *testing.T) {
		err := validateFilename("照片.jpg")
		assert.NoError(t, err)
	})
}
