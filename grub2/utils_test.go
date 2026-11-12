// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub2

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_replaceAndbackupDir(t *testing.T) {
	tmpDirA, err := getTempDir()
	require.NoError(t, err)

	testFile := filepath.Join(tmpDirA, "test")
	dataA := []byte{'a'}
	dataB := []byte{'b'}
	err = os.WriteFile(testFile, dataA, 0755)
	require.NoError(t, err)

	tmpDirB, err := getTempDir()
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDirB, "test"), dataB, 0755)
	require.NoError(t, err)

	err = replaceAndBackupDir(tmpDirA, tmpDirB)
	require.NoError(t, err)

	get, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Equal(t, dataB, get)

	bakDir := tmpDirA + ".bak"
	get, err = os.ReadFile(filepath.Join(bakDir, "test"))
	require.NoError(t, err)
	require.Equal(t, dataA, get)

	_ = os.RemoveAll(bakDir)
	_ = os.RemoveAll(tmpDirA)
}

func Test_copyBgSource(t *testing.T) {
	src := "testdata/theme/deepin"
	dst, err := getTempDir()
	require.NoError(t, err)

	err = copyBgSource(src, dst)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(dst, "background_source"))

	_ = os.RemoveAll(dst)
}

func Test_replaceAndBackupDirKeepsSourceWhenTargetMissing(t *testing.T) {
	src, err := getTempDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(src, "theme"), []byte("old"), 0644))

	// The target does not exist: the swap must fail, and the previous theme
	// has to be restored instead of being left in the backup directory.
	err = replaceAndBackupDir(src, filepath.Join(src, "does-not-exist"))
	require.Error(t, err)

	content, err := os.ReadFile(filepath.Join(src, "theme"))
	require.NoError(t, err)
	assert.Equal(t, "old", string(content))
}
