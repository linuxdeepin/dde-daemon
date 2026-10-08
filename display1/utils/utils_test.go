// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package utils

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeEdidBase64(t *testing.T) {
	// 与标准 base64 编码结果一致（确认使用 StdEncoding，非 URLEncoding）
	edid := []byte{0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00}
	assert.Equal(t, base64.StdEncoding.EncodeToString(edid), EncodeEdidBase64(edid))

	// 手工可验证的小样本
	assert.Equal(t, "/w==", EncodeEdidBase64([]byte{0xFF}))
	assert.Equal(t, "AAA=", EncodeEdidBase64([]byte{0x00, 0x00}))
}

func TestEncodeEdidBase64_StdEncodingLocked(t *testing.T) {
	// 0xFB,0xFF 经 StdEncoding 编码为 "+/8="（含 '+' 与 '/'），
	// URLEncoding 则为 "-_8="（含 '-' 与 '_'）。用字面量锁死 StdEncoding，
	// 防止被误改为 URLEncoding。
	assert.Equal(t, "+/8=", EncodeEdidBase64([]byte{0xFB, 0xFF}))
	assert.Contains(t, EncodeEdidBase64([]byte{0xFB, 0xFF}), "+")
	assert.Contains(t, EncodeEdidBase64([]byte{0xFB, 0xFF}), "/")
}

func TestDecodeEdidBase64(t *testing.T) {
	got, err := DecodeEdidBase64("/w==")
	assert.NoError(t, err)
	assert.Equal(t, []byte{0xFF}, got)

	got, err = DecodeEdidBase64("AAA=")
	assert.NoError(t, err)
	assert.Equal(t, []byte{0x00, 0x00}, got)
}

func TestEncodeDecodeEdidBase64_RoundTrip(t *testing.T) {
	// 模拟 128 字节 EDID 数据
	edid := make([]byte, 128)
	for i := range edid {
		edid[i] = byte(i)
	}

	encoded := EncodeEdidBase64(edid)
	decoded, err := DecodeEdidBase64(encoded)
	require.NoError(t, err)
	assert.Equal(t, edid, decoded)
}

func TestDecodeEdidBase64_Empty(t *testing.T) {
	got, err := DecodeEdidBase64("")
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, got)
}

func TestDecodeEdidBase64_Invalid(t *testing.T) {
	// 非法 base64 输入应返回错误
	_, err := DecodeEdidBase64("!!!not-base64!!!")
	assert.Error(t, err)
}

func TestEncodeEdidBase64_Empty(t *testing.T) {
	assert.Equal(t, "", EncodeEdidBase64(nil))
	assert.Equal(t, "", EncodeEdidBase64([]byte{}))
}
