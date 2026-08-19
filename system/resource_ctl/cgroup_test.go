// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package resource_control

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToSystemdPath(t *testing.T) {
	assert.Equal(t, "/sys/fs/cgroup/systemd/app-dde-x.scope",
		toSystemdPath("app-dde-x.scope"))
	assert.Equal(t, "/sys/fs/cgroup/systemd/", toSystemdPath(""))
}

func TestToCpuPath(t *testing.T) {
	assert.Equal(t, "/sys/fs/cgroup/cpu/app-dde-x.scope",
		toCpuPath("app-dde-x.scope"))
	assert.Equal(t, "/sys/fs/cgroup/cpu/", toCpuPath(""))
}

func TestToMemPath(t *testing.T) {
	assert.Equal(t, "/sys/fs/cgroup/memory/app-dde-x.scope",
		toMemPath("app-dde-x.scope"))
	assert.Equal(t, "/sys/fs/cgroup/memory/", toMemPath(""))
}

func TestGetTasksFromFile(t *testing.T) {
	dir := t.TempDir()

	// 多行（含结尾换行）：按 "\n" 切分，末尾产生一个空元素
	p1 := filepath.Join(dir, "tasks_trailing")
	require.NoError(t, os.WriteFile(p1, []byte("10\n20\n"), 0600))
	tasks, err := getTasksFromFile(p1)
	assert.NoError(t, err)
	assert.Equal(t, [][]byte{[]byte("10"), []byte("20"), []byte("")}, tasks)

	// 多行（无结尾换行）
	p2 := filepath.Join(dir, "tasks_notrail")
	require.NoError(t, os.WriteFile(p2, []byte("10\n20"), 0600))
	tasks, err = getTasksFromFile(p2)
	assert.NoError(t, err)
	assert.Equal(t, [][]byte{[]byte("10"), []byte("20")}, tasks)

	// 单行无换行
	p3 := filepath.Join(dir, "tasks_single")
	require.NoError(t, os.WriteFile(p3, []byte("42"), 0600))
	tasks, err = getTasksFromFile(p3)
	assert.NoError(t, err)
	assert.Equal(t, [][]byte{[]byte("42")}, tasks)

	// 空文件：得到单个空元素
	p4 := filepath.Join(dir, "tasks_empty")
	require.NoError(t, os.WriteFile(p4, []byte(""), 0600))
	tasks, err = getTasksFromFile(p4)
	assert.NoError(t, err)
	assert.Equal(t, [][]byte{[]byte("")}, tasks)
}

func TestGetTasksFromFile_NonExistent(t *testing.T) {
	dir := t.TempDir()
	tasks, err := getTasksFromFile(filepath.Join(dir, "missing"))
	assert.Error(t, err)
	assert.Nil(t, tasks)
	// 错误应包含来源路径上下文
	assert.Contains(t, err.Error(), "failed to get tasks from")
	assert.Contains(t, err.Error(), "missing")
}
