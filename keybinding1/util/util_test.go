// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarshalJSON_NoHTMLEscape(t *testing.T) {
	// SetEscapeHTML(false) 应保留 < > 等字符，不转义为 \u003c
	got, err := MarshalJSON(&KWinAccel{
		Id:         "x",
		Keystrokes: []string{"<Alt>Print"},
	})
	assert.NoError(t, err)
	// enc.Encode 会追加一个换行
	assert.Equal(t, `{"Id":"x","Accels":["<Alt>Print"]}`+"\n", got)
	// 关键断言：尖括号未被 HTML 转义
	assert.True(t, strings.Contains(got, "<Alt>Print"))
	assert.False(t, strings.Contains(got, `\u003c`))
}

func TestMarshalJSON_WithDefault(t *testing.T) {
	got, err := MarshalJSON(&KWinAccel{
		Id:                "y",
		Keystrokes:        []string{"a"},
		DefaultKeystrokes: []string{"b"},
	})
	assert.NoError(t, err)
	assert.Equal(t, `{"Id":"y","Accels":["a"],"Default":["b"]}`+"\n", got)
}

func TestMarshalJSON_DefaultOmittedWhenEmpty(t *testing.T) {
	got, err := MarshalJSON(&KWinAccel{
		Id:                "z",
		Keystrokes:        []string{"a"},
		DefaultKeystrokes: nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, `{"Id":"z","Accels":["a"]}`+"\n", got)
	assert.False(t, strings.Contains(got, "Default"))
}

func TestMarshalJSON_Error(t *testing.T) {
	// 不可序列化的值（chan）应使 enc.Encode 返回错误，函数返回 ""
	got, err := MarshalJSON(make(chan int))
	assert.Error(t, err)
	assert.Equal(t, "", got)
}

func TestKWinAccel_FixFiltersKeystrokes(t *testing.T) {
	kwa := &KWinAccel{
		Keystrokes:        []string{"a", "", "b", ""},
		DefaultKeystrokes: []string{"x", "y z", "", "z", " w"},
	}
	kwa.fix()

	assert.Equal(t, []string{"a", "b"}, kwa.Keystrokes)
	// 含空格或为空的默认值应被剔除
	assert.Equal(t, []string{"x", "z"}, kwa.DefaultKeystrokes)
}

func TestKWinAccel_FixAllEmpty(t *testing.T) {
	kwa := &KWinAccel{
		Keystrokes:        []string{"", ""},
		DefaultKeystrokes: []string{"", "  "},
	}
	kwa.fix()

	assert.Equal(t, 0, len(kwa.Keystrokes))
	assert.Equal(t, 0, len(kwa.DefaultKeystrokes))
}

func TestKWinAccel_FixIdempotentOnClean(t *testing.T) {
	kwa := &KWinAccel{
		Keystrokes:        []string{"a", "b"},
		DefaultKeystrokes: []string{"c"},
	}
	kwa.fix()
	assert.Equal(t, []string{"a", "b"}, kwa.Keystrokes)
	assert.Equal(t, []string{"c"}, kwa.DefaultKeystrokes)
}

func TestKWinAccel_FixKeystrokesKeepsSpaces(t *testing.T) {
	// 锁定 fix() 的非对称语义：Keystrokes 仅过滤空串、保留含空格值；
	// DefaultKeystrokes 同时过滤空串与含空格值。
	kwa := &KWinAccel{
		Keystrokes:        []string{"a b", "c", ""},
		DefaultKeystrokes: []string{"d e", "f", ""},
	}
	kwa.fix()
	// Keystrokes 保留含空格的 "a b"，仅剔除空串
	assert.Equal(t, []string{"a b", "c"}, kwa.Keystrokes)
	// DefaultKeystrokes 剔除含空格的 "d e" 与空串
	assert.Equal(t, []string{"f"}, kwa.DefaultKeystrokes)
}

func TestKWinAccel_FixOnZeroValue(t *testing.T) {
	// 零值（nil 切片）入参不应 panic，结果为 nil
	kwa := &KWinAccel{}
	kwa.fix() // 若 panic 则用例直接失败
	assert.Nil(t, kwa.Keystrokes)
	assert.Nil(t, kwa.DefaultKeystrokes)
}
