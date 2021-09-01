/*
 * Copyright (C) 2018 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
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

package calltrace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestEnv() (string, error) {
	const key = "XDG_CACHE_HOME"
	if testDir, err := filepath.Abs("testdata"); err != nil {
		return "", err
	} else {
		return os.Getenv(key), os.Setenv(key, testDir)
	}
}

func revertEnv(val string) error {
	const key = "XDG_CACHE_HOME"
	return os.Setenv(key, val)
}

func TestManager_sample(t *testing.T) {
	val, err := initTestEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, revertEnv(val))
	}()
	m, err := NewManager(3)
	require.NoError(t, err)
	require.NotNil(t, m)

	m.SetAutoDestroy(5)
	time.Sleep(10 * time.Second)
	assert.Nil(t, m.quit)
}

func Test_ensureDirExisit(t *testing.T) {
	val, err := initTestEnv()
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, revertEnv(val))
	}()
	dir, err := ensureDirExist()
	if err != nil {
		assert.Equal(t, "", dir)
		return
	}

	assert.NotEmpty(t, dir)
	_, err = os.Stat(dir)
	want := (err == nil || os.IsExist(err))

	assert.Equal(t, true, want)
}
