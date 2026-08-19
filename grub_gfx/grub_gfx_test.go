// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub_gfx

import (
	"testing"

	"github.com/linuxdeepin/dde-daemon/grub_common"

	"github.com/stretchr/testify/assert"
)

func TestDetectCache_Equal_Match(t *testing.T) {
	c := DetectCache{
		EdidsHash:  "abc123",
		MaxGfxmode: "1920x1080",
	}
	assert.True(t, c.equal("abc123", grub_common.Gfxmode{Width: 1920, Height: 1080}))
}

func TestDetectCache_Equal_HashMismatch(t *testing.T) {
	c := DetectCache{
		EdidsHash:  "abc123",
		MaxGfxmode: "1920x1080",
	}
	assert.False(t, c.equal("different", grub_common.Gfxmode{Width: 1920, Height: 1080}))
}

func TestDetectCache_Equal_GfxmodeMismatch(t *testing.T) {
	c := DetectCache{
		EdidsHash:  "abc123",
		MaxGfxmode: "1920x1080",
	}
	assert.False(t, c.equal("abc123", grub_common.Gfxmode{Width: 1280, Height: 720}))
}

func TestDetectCache_Equal_BothMismatch(t *testing.T) {
	c := DetectCache{
		EdidsHash:  "abc123",
		MaxGfxmode: "1920x1080",
	}
	assert.False(t, c.equal("wrong", grub_common.Gfxmode{Width: 800, Height: 600}))
}

func TestDetectCache_Equal_EmptyCache(t *testing.T) {
	c := DetectCache{}
	assert.False(t, c.equal("abc123", grub_common.Gfxmode{Width: 1920, Height: 1080}))
}

func TestDetectCache_Equal_EmptyHash(t *testing.T) {
	c := DetectCache{
		EdidsHash:  "",
		MaxGfxmode: "1920x1080",
	}
	assert.True(t, c.equal("", grub_common.Gfxmode{Width: 1920, Height: 1080}))
	assert.False(t, c.equal("nonempty", grub_common.Gfxmode{Width: 1920, Height: 1080}))
}

func TestSaveDetectCache_Invalid(t *testing.T) {
	// 空的 EdidsHash 或 MaxGfxmode 应返回错误
	err := saveDetectCache(DetectCache{})
	assert.Error(t, err)

	err = saveDetectCache(DetectCache{EdidsHash: "abc", MaxGfxmode: ""})
	assert.Error(t, err)

	err = saveDetectCache(DetectCache{EdidsHash: "", MaxGfxmode: "1920x1080"})
	assert.Error(t, err)
}
