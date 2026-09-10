// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package display1

import (
	"math"
	"testing"
)

func TestUnscaleBrightnessBoundary(t *testing.T) {
	// 场景1：balance 模式（scale=1.0）设 10%，应保持 0.1
	if got := unscaleBrightness(0.1, 1.0); math.Abs(got-0.1) > 1e-9 {
		t.Errorf("balance 设10%%: got %v, want 0.1", got)
	}
	// 场景2：节能模式（scale=0.6，drop=40%）调到最低 10%，应还原为 0.167（17%）
	got := unscaleBrightness(0.1, 0.6)
	if math.Abs(got-0.167) > 0.001 {
		t.Errorf("节能设10%%: got %v, want 0.167", got)
	}
	// 场景3：切回 balance 后恢复（scaleBrightness(0.167, 1.0)）
	if restored := scaleBrightness(got, 1.0); math.Abs(restored-got) > 1e-9 {
		t.Errorf("恢复失败: %v", restored)
	}
	// 场景4：节能设 50%，还原为 0.833
	if got := unscaleBrightness(0.5, 0.6); math.Abs(got-0.833) > 0.001 {
		t.Errorf("节能设50%%: got %v, want 0.833", got)
	}
	// 场景5：scale=0（drop=100%）不可逆，保持原值
	if got := unscaleBrightness(0.5, 0.0); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("scale=0: got %v, want 0.5", got)
	}
	// 场景6：还原后不超过 1.0（节能设 100%，drop=40% → 1.67 clamp 到 1.0）
	if got := unscaleBrightness(1.0, 0.6); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("上限 clamp: got %v, want 1.0", got)
	}
}
