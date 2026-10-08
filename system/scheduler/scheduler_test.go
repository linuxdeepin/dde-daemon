// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU32SliceContains(t *testing.T) {
	slice := []uint32{1, 10, 100, 1000}
	assert.True(t, u32SliceContains(slice, 1))
	assert.True(t, u32SliceContains(slice, 1000))
	assert.True(t, u32SliceContains(slice, 10))
	assert.False(t, u32SliceContains(slice, 5))
	assert.False(t, u32SliceContains(slice, 0))
}

func TestU32SliceContains_Empty(t *testing.T) {
	assert.False(t, u32SliceContains(nil, 1))
	assert.False(t, u32SliceContains([]uint32{}, 1))
}

func TestConfig_GetPriority_ByFullPath(t *testing.T) {
	cfg := &config{
		Processes: map[string]*priorityCfg{
			"/usr/bin/foo": {CPU: 5},
		},
	}
	p := cfg.getPriority("/usr/bin/foo")
	assert.NotNil(t, p)
	assert.Equal(t, 5, p.CPU)
}

func TestConfig_GetPriority_ByBaseName(t *testing.T) {
	// 完整路径未命中时，回退用 filepath.Base(exe) 查找
	cfg := &config{
		Processes: map[string]*priorityCfg{
			"foo": {CPU: 9},
		},
	}
	p := cfg.getPriority("/usr/bin/foo")
	assert.NotNil(t, p)
	assert.Equal(t, 9, p.CPU)

	p = cfg.getPriority("foo")
	assert.NotNil(t, p)
	assert.Equal(t, 9, p.CPU)
}

func TestConfig_GetPriority_NotFound(t *testing.T) {
	cfg := &config{
		Processes: map[string]*priorityCfg{
			"bar": {CPU: 1},
		},
	}
	assert.Nil(t, cfg.getPriority("/usr/bin/foo"))
	assert.Nil(t, cfg.getPriority("foo"))
}

func TestConfig_GetPriority_EmptyProcesses(t *testing.T) {
	cfg := &config{Processes: map[string]*priorityCfg{}}
	assert.Nil(t, cfg.getPriority("/usr/bin/anything"))
}

func TestLoadConfigAux(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
		"processes": {
			"/usr/bin/foo": {"cpu": 5},
			"bar": {"cpu": 3}
		},
		"enabled": true,
		"procMonitorEnabled": false
	}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	cfg, err := loadConfigAux(path)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, path, cfg.filename)
	assert.True(t, cfg.Enabled)
	assert.False(t, cfg.ProcMonitorEnabled)
	assert.Equal(t, 5, cfg.Processes["/usr/bin/foo"].CPU)
	assert.Equal(t, 3, cfg.Processes["bar"].CPU)

	// getPriority 能命中
	p := cfg.getPriority("/usr/bin/foo")
	assert.Equal(t, 5, p.CPU)
	p = cfg.getPriority("/usr/bin/bar")
	assert.Equal(t, 3, p.CPU)
}

func TestLoadConfigAux_NonExistent(t *testing.T) {
	_, err := loadConfigAux("/nonexistent/config.json")
	assert.Error(t, err)
}

func TestLoadConfigAux_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte("{invalid json"), 0600))

	_, err := loadConfigAux(path)
	assert.Error(t, err)
}

func TestLoadConfigAux_EmptyProcesses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{"processes": {}, "enabled": false}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	cfg, err := loadConfigAux(path)
	require.NoError(t, err)
	assert.NotNil(t, cfg.Processes)
	assert.False(t, cfg.Enabled)
}
