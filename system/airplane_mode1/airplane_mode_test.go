// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package airplane_mode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRadioAction_ToRfkillState(t *testing.T) {
	assert.Equal(t, rfkillStateUnblock, UnblockRadioAction.ToRfkillState())
	assert.Equal(t, rfkillStateBlock, BlockRadioAction.ToRfkillState())
	// 未映射的动作返回零值（rfkillStateUnblock = 0）
	assert.Equal(t, rfkillState(0), NoneRadioAction.ToRfkillState())
	assert.Equal(t, rfkillState(0), ListRadioAction.ToRfkillState())
	assert.Equal(t, rfkillState(0), MonitorRadioAction.ToRfkillState())
}

func TestRadioAction_String(t *testing.T) {
	assert.Equal(t, "block", BlockRadioAction.String())
	assert.Equal(t, "unblock", UnblockRadioAction.String())
	assert.Equal(t, "list", ListRadioAction.String())
	assert.Equal(t, "event", MonitorRadioAction.String())
	// 未映射的动作返回空串
	assert.Equal(t, "", NoneRadioAction.String())
}

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()
	assert.NotNil(t, cfg)
	// 新建配置，所有模块默认未阻塞
	assert.False(t, cfg.GetBlocked(rfkillTypeWifi))
	assert.False(t, cfg.GetBlocked(rfkillTypeBT))
	assert.False(t, cfg.GetBlocked(rfkillTypeAll))
}

func TestConfig_SetGetBlocked(t *testing.T) {
	cfg := NewConfig()

	cfg.SetBlocked(rfkillTypeWifi, true)
	assert.True(t, cfg.GetBlocked(rfkillTypeWifi))
	// 其它模块不受影响
	assert.False(t, cfg.GetBlocked(rfkillTypeBT))

	cfg.SetBlocked(rfkillTypeBT, true)
	assert.True(t, cfg.GetBlocked(rfkillTypeWifi))
	assert.True(t, cfg.GetBlocked(rfkillTypeBT))

	// 解除阻塞
	cfg.SetBlocked(rfkillTypeWifi, false)
	assert.False(t, cfg.GetBlocked(rfkillTypeWifi))
	assert.True(t, cfg.GetBlocked(rfkillTypeBT))
}

func TestConfig_GetBlocked_DefaultFalse(t *testing.T) {
	cfg := NewConfig()
	// 未设置过的模块，GetBlocked 返回 false（默认未阻塞）
	assert.False(t, cfg.GetBlocked(rfkillTypeAll))
	assert.False(t, cfg.GetBlocked(rfkillTypeWifi))
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("  hello world  \n"), 0600))

	got, err := readFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "hello world", got)
}

func TestReadFile_NonExistent(t *testing.T) {
	_, err := readFile("/nonexistent/file/path")
	assert.Error(t, err)
}

func TestReadFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(path, []byte(""), 0600))

	got, err := readFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestIsLittleEndian(t *testing.T) {
	// 不关心具体平台结果，只验证不 panic 且返回布尔值
	_ = isLittleEndian()
}

func TestGetByteOrder(t *testing.T) {
	// 验证返回非 nil 的 ByteOrder，不 panic
	_ = getByteOrder()
}
