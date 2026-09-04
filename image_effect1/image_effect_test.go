// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package image_effect

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModTimeEqual_SameTime(t *testing.T) {
	t1 := time.Date(2026, 1, 15, 10, 30, 0, 500000, time.UTC)
	t2 := time.Date(2026, 1, 15, 10, 30, 0, 500000, time.UTC)
	assert.True(t, modTimeEqual(t1, t2))
}

func TestModTimeEqual_DifferentTime(t *testing.T) {
	t1 := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 15, 10, 30, 5, 0, time.UTC)
	assert.False(t, modTimeEqual(t1, t2))
}

func TestModTimeEqual_TruncatesNanoseconds(t *testing.T) {
	// modTimeEqual 仅比较到微秒（/1000），同微秒但不同纳秒应判等
	t1 := time.Date(2026, 1, 15, 10, 30, 0, 500999, time.UTC)
	t2 := time.Date(2026, 1, 15, 10, 30, 0, 500000, time.UTC)
	assert.True(t, modTimeEqual(t1, t2))
}

func TestModTimeEqual_DifferentMicrosecond(t *testing.T) {
	t1 := time.Date(2026, 1, 15, 10, 30, 0, 500000, time.UTC)
	t2 := time.Date(2026, 1, 15, 10, 30, 0, 600000, time.UTC)
	assert.False(t, modTimeEqual(t1, t2))
}

func TestModTimeEqual_BothZero(t *testing.T) {
	assert.True(t, modTimeEqual(time.Time{}, time.Time{}))
}

func TestSetFileModTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0600))

	target := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	err := setFileModTime(path, target)
	assert.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, target.Unix(), info.ModTime().Unix())
}

func TestSetFileModTime_NonExistent(t *testing.T) {
	err := setFileModTime("/nonexistent/file/path.txt", time.Now())
	assert.Error(t, err)
}

func TestGetOutputFile(t *testing.T) {
	// getOutputFile = filepath.Join(cacheDir, effect, md5(filename)+ext(filename))
	out := getOutputFile("blur", "/home/user/photo.png")
	assert.Contains(t, out, cacheDir)
	assert.Contains(t, out, "blur")
	assert.Contains(t, out, ".png")
}

func TestGetOutputFile_NoExtension(t *testing.T) {
	out := getOutputFile("effect", "noextfile")
	assert.Contains(t, out, cacheDir)
	assert.Contains(t, out, "effect")
	// 无扩展名，filepath.Ext 返回 ""
	assert.False(t, containsExt(out))
}

func containsExt(path string) bool {
	return filepath.Ext(path) != ""
}
