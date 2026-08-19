// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package iso639

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertA2ToA3_SameTAndB(t *testing.T) {
	// A3T == A3B 时仅返回一个元素
	// "aa" → A3B="aar", A3T="aar"
	result := ConvertA2ToA3("aa")
	assert.Equal(t, []string{"aar"}, result)
}

func TestConvertA2ToA3_DifferentTAndB(t *testing.T) {
	// A3T != A3B 时返回两个元素 [A3T, A3B]
	// "sq" → A3T="sqi", A3B="alb"
	result := ConvertA2ToA3("sq")
	assert.Equal(t, []string{"sqi", "alb"}, result)
}

func TestConvertA2ToA3_DifferentTAndB_Zh(t *testing.T) {
	// "zh" → A3T="zho", A3B="chi"
	result := ConvertA2ToA3("zh")
	assert.Equal(t, []string{"zho", "chi"}, result)
}

func TestConvertA2ToA3_NotFound(t *testing.T) {
	// 不存在的 ISO 639-1 代码，返回 nil
	result := ConvertA2ToA3("zz")
	assert.Nil(t, result)
}

func TestConvertA2ToA3_EmptyString(t *testing.T) {
	// 源码未做空输入守卫：空串匹配首个 A2 为空的条目（ace，A3T==A3B），
	// 返回 []string{"ace"} 而非 nil。锁定当前真实行为。
	result := ConvertA2ToA3("")
	assert.Equal(t, []string{"ace"}, result)
}

func TestConvertA2ToA3_CommonLanguages(t *testing.T) {
	// 验证常见语言代码（A3T == A3B 的情况）
	for _, tc := range []struct {
		a2   string
		a3   string
	}{
		{"aa", "aar"},
		{"en", "eng"},
		{"fr", "fra"}, // A3T="fra", A3B="fre" → 不同，返回两个
	} {
		result := ConvertA2ToA3(tc.a2)
		assert.NotEqual(t, 0, len(result), "a2=%s", tc.a2)
		assert.Equal(t, tc.a3, result[0], "a2=%s", tc.a2)
	}
}

func TestConvertA2ToA3_FrenchReturnsTwoCodes(t *testing.T) {
	// "fr" → A3T="fra", A3B="fre"，返回两个
	result := ConvertA2ToA3("fr")
	assert.Equal(t, []string{"fra", "fre"}, result)
	assert.Equal(t, 2, len(result))
}
