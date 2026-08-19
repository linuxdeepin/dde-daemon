// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeReadFile_RegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	content := []byte("hello dde-daemon")
	require.NoError(t, os.WriteFile(path, content, 0600))

	got, err := SafeReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestSafeReadFile_NonExistent(t *testing.T) {
	dir := t.TempDir()
	_, err := SafeReadFile(filepath.Join(dir, "missing"))
	assert.Error(t, err)
}

func TestSafeReadFile_SymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))

	_, err := SafeReadFile(link)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")
}

func TestSafeReadFile_DirectoryRejected(t *testing.T) {
	dir := t.TempDir()
	// 目录不是普通文件，应被拒绝
	_, err := SafeReadFile(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular file")
}

func TestSafeWriteFile_CreateNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	content := []byte("fresh content")

	err := SafeWriteFile(path, content, 0600)
	assert.NoError(t, err)

	// 权限：owner 读写位必置（用掩码校验，对仅清 group/other 位的 umask 稳健）
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm()&os.FileMode(0600))

	got, err := SafeReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestSafeWriteFile_OverwriteRegular(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0600))

	err := SafeWriteFile(path, []byte("new"), 0600)
	assert.NoError(t, err)

	got, err := SafeReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, []byte("new"), got)
}

func TestSafeWriteFile_SymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))

	err := SafeWriteFile(link, []byte("y"), 0600)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "symlink")

	// 被拒绝后，符号链接目标内容不应被改写
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, []byte("x"), got)
}

func TestSafeWriteFile_DirectoryRejected(t *testing.T) {
	// 目标已存在且为目录（非普通文件），写侧应拒绝，与读侧对称
	dir := t.TempDir()
	err := SafeWriteFile(dir, []byte("x"), 0600)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular file")
}

func TestSafeWriteFile_NonExistentParent(t *testing.T) {
	dir := t.TempDir()
	// 父目录不存在，O_CREAT|O_EXCL 创建应失败
	err := SafeWriteFile(filepath.Join(dir, "sub", "file"), []byte("z"), 0600)
	assert.Error(t, err)
}

func TestSafeWriteFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "round.txt")
	payload := []byte{0x00, 0x01, 0xFF, 'A', '\n', '\t'}

	require.NoError(t, SafeWriteFile(path, payload, 0600))
	got, err := SafeReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, payload, got)
}
