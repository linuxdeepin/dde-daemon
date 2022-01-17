/*
 * Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:     liaohanqin <liaohanqin@uniontech.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package grub2

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_replaceAndbackupDir(t *testing.T) {
	tmpDirA, err := getTempDir()
	require.NoError(t, err)

	testFile := filepath.Join(tmpDirA, "test")
	dataA := []byte{'a'}
	dataB := []byte{'b'}
	err = ioutil.WriteFile(testFile, dataA, 0755)
	require.NoError(t, err)

	tmpDirB, err := getTempDir()
	require.NoError(t, err)

	err = ioutil.WriteFile(filepath.Join(tmpDirB, "test"), dataB, 0755)
	require.NoError(t, err)

	err = replaceAndBackupDir(tmpDirA, tmpDirB)
	require.NoError(t, err)

	get, err := ioutil.ReadFile(testFile)
	require.NoError(t, err)
	require.Equal(t, dataB, get)

	bakDir := tmpDirA + ".bak"
	get, err = ioutil.ReadFile(filepath.Join(bakDir, "test"))
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
