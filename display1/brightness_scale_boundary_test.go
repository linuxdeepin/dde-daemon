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
	// 场景6：节能设 100%（drop=40%），还原为 1.667（不再钳制到 1.0）
	// round-trip 验证：scaleBrightness(1.667, 0.6) = 1.0，唤醒后亮度保持 100%
	got = unscaleBrightness(1.0, 0.6)
	if math.Abs(got-1.667) > 0.001 {
		t.Errorf("节能设100%%: got %v, want 1.667", got)
	}
	if restored := scaleBrightness(got, 0.6); math.Abs(restored-1.0) > 1e-9 {
		t.Errorf("round-trip 恢复失败: %v, want 1.0", restored)
	}
}

func TestBrightnessBaseAfterScale(t *testing.T) {
	// 场景1：BUG-376299 原始步骤——balance 设 10%（0.1），开节能 drop=40%
	// 显示被钳到最低 10%，逻辑基准应改写为 0.1/0.6=0.167，
	// 关闭节能后恢复 17%
	base, changed := brightnessBaseAfterScale(0.1, 0.6)
	if !changed || math.Abs(base-0.167) > 0.001 {
		t.Errorf("钳到最低: got (%v, %v), want (0.167, true)", base, changed)
	}
	if restored := scaleBrightness(base, 1.0); math.Abs(restored-0.167) > 0.001 {
		t.Errorf("关闭节能恢复: got %v, want 0.167", restored)
	}
	// 场景2：需求示例——base 14%，drop=20%（scale 0.8）显示 11.2% 未被钳制，基准不变
	if base, changed := brightnessBaseAfterScale(0.14, 0.8); changed || math.Abs(base-0.14) > 1e-9 {
		t.Errorf("未钳制: got (%v, %v), want (0.14, false)", base, changed)
	}
	// 场景3：需求示例——base 14%，drop=30%（scale 0.7）折算 9.8% 被钳制
	if base, changed := brightnessBaseAfterScale(0.14, 0.7); !changed || math.Abs(base-0.143) > 0.001 {
		t.Errorf("折算被钳制: got (%v, %v), want (0.143, true)", base, changed)
	}
	// 场景4：亮且未被钳制（1.0 × 0.6 = 0.6）基准不变
	if _, changed := brightnessBaseAfterScale(1.0, 0.6); changed {
		t.Error("0.6 未被钳制，不应改写基准")
	}
	// 场景5：关闭节能（scale=1.0）不改写基准
	if _, changed := brightnessBaseAfterScale(0.1, 1.0); changed {
		t.Error("scale=1.0 不应改写基准")
	}
	// 场景6：scale=0（drop=100%）不可逆，基准不变
	if _, changed := brightnessBaseAfterScale(0.5, 0.0); changed {
		t.Error("scale=0 不应改写基准")
	}
	// 场景7：改写后的基准再按同一 scale 应用不再被钳制，避免反复落盘或漂移
	if _, changed := brightnessBaseAfterScale(0.167, 0.6); changed {
		t.Error("改写后的基准不应再次改写（幂等）")
	}
	// 场景8：drop=10%（scale 0.9）时 0.111×0.9 < 0.1 仍被钳制，
	// 但基准未变化，不应重复落盘
	if base, changed := brightnessBaseAfterScale(0.1, 0.9); !changed || math.Abs(base-0.111) > 0.001 {
		t.Errorf("drop=10%%: got (%v, %v), want (0.111, true)", base, changed)
	}
	if _, changed := brightnessBaseAfterScale(0.111, 0.9); changed {
		t.Error("drop=10%% 重复应用不应重复落盘")
	}
	// 场景9：drop>90%（基准钳到 1.0）同样只写一次
	if base, changed := brightnessBaseAfterScale(0.1, 0.05); !changed || math.Abs(base-1.0) > 1e-9 {
		t.Errorf("drop=95%%: got (%v, %v), want (1.0, true)", base, changed)
	}
	if _, changed := brightnessBaseAfterScale(1.0, 0.05); changed {
		t.Error("drop=95%% 重复应用不应重复落盘")
	}
}
