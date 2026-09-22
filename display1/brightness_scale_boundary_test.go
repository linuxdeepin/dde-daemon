// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package display1

import (
	"math"
	"testing"
)

// TestRescaleBrightness 覆盖"存显示值、切换时换算"模型的各需求场景。
// rescaleBrightness(displayed, oldScale, newScale)：
// 先按旧系数还原逻辑亮度（上限 100%），再乘新系数，钳制到 [0.1, 1.0]。
func TestRescaleBrightness(t *testing.T) {
	approx := func(got, want float64) bool { return math.Abs(got-want) < 0.001 }

	// 注：BUG 修复本身在恢复路径——RefreshBrightness/配置应用直接使用
	// config.Brightness（存的即显示值），不再反复施加缩放。rescaleBrightness
	// 只在节能开关/比例变化时调用（oldScale != newScale），下面覆盖这些换算。

	// 场景2：开启节能——亮度 = 当前 × (1-X%)。
	// 当前 100%，drop=10% → 90%
	if got := rescaleBrightness(1.0, 1.0, 0.9); !approx(got, 0.9) {
		t.Errorf("开节能 drop=10%%: got %v, want 0.9", got)
	}
	// 当前 100%，drop=20% → 80%
	if got := rescaleBrightness(1.0, 1.0, 0.8); !approx(got, 0.8) {
		t.Errorf("开节能 drop=20%%: got %v, want 0.8", got)
	}

	// 场景3：关闭节能——亮度 = 当前 / (1-X%)，超 100% 显示 100%。
	// 节能下 80%（drop=20%），关闭 → 80%/0.8 = 100%
	if got := rescaleBrightness(0.8, 0.8, 1.0); !approx(got, 1.0) {
		t.Errorf("关节能 80%%/0.8: got %v, want 1.0", got)
	}
	// 节能下 90%（drop=10%），关闭 → 90%/0.9 = 100%
	if got := rescaleBrightness(0.9, 0.9, 1.0); !approx(got, 1.0) {
		t.Errorf("关节能 90%%/0.9: got %v, want 1.0", got)
	}
	// 节能下 50%（drop=40%），关闭 → 50%/0.6 = 83.3%
	if got := rescaleBrightness(0.5, 0.6, 1.0); !approx(got, 0.833) {
		t.Errorf("关节能 50%%/0.6: got %v, want 0.833", got)
	}

	// 场景4：降低后低于总亮度 10% → 显示 10%。
	// balance 设 10%（0.1），开节能 drop=40% → 0.1×0.6=0.06 钳到 0.1
	if got := rescaleBrightness(0.1, 1.0, 0.6); !approx(got, 0.1) {
		t.Errorf("降低触底钳到10%%: got %v, want 0.1", got)
	}

	// 场景5：关闭节能以 10% 为基准提高 = 10%/(1-X%)。
	// 承接场景4，节能下显示 10%（drop=40%），关闭 → 0.1/0.6 = 16.7%
	if got := rescaleBrightness(0.1, 0.6, 1.0); !approx(got, 0.167) {
		t.Errorf("关节能以10%%为基准: got %v, want 0.167", got)
	}

	// 场景6：改降低比例，按"提高后的值"折算，超 100% 按 100% 上限折算。
	// 节能下显示 90%（drop=10%），改成 drop=40%(scale=0.6)：
	// 逻辑=min(0.9/0.9,1.0)=1.0，× 0.6 = 60%
	if got := rescaleBrightness(0.9, 0.9, 0.6); !approx(got, 0.6) {
		t.Errorf("改比例 90%%→drop40%%: got %v, want 0.6", got)
	}
	// 节能下手动调到 100%（drop=10%），改成 drop=20%：
	// 逻辑=min(1.0/0.9,1.0)=1.0（上限封顶），× 0.8 = 80%
	if got := rescaleBrightness(1.0, 0.9, 0.8); !approx(got, 0.8) {
		t.Errorf("改比例 100%%→drop20%% 按100%%上限: got %v, want 0.8", got)
	}

	// 场景7：drop=100%（newScale=0）→ 显示钳到最低 10%
	if got := rescaleBrightness(1.0, 1.0, 0.0); !approx(got, 0.1) {
		t.Errorf("drop=100%%: got %v, want 0.1", got)
	}

	// 场景8：旧系数为 0（不可逆）保持原值
	if got := rescaleBrightness(0.5, 0.0, 1.0); !approx(got, 0.5) {
		t.Errorf("oldScale=0 保持原值: got %v, want 0.5", got)
	}
}

// TestScaleBrightness 覆盖自动亮度推荐值缩放（仍为连续缩放）。
func TestScaleBrightness(t *testing.T) {
	approx := func(got, want float64) bool { return math.Abs(got-want) < 0.001 }

	// 不缩放
	if got := scaleBrightness(1.0, 1.0); !approx(got, 1.0) {
		t.Errorf("scale=1.0: got %v, want 1.0", got)
	}
	// drop=20%
	if got := scaleBrightness(1.0, 0.8); !approx(got, 0.8) {
		t.Errorf("scale=0.8: got %v, want 0.8", got)
	}
	// 原始值 <= 最低亮度不缩放
	if got := scaleBrightness(0.1, 0.6); !approx(got, 0.1) {
		t.Errorf("base<=0.1: got %v, want 0.1", got)
	}
	// 缩放触底钳到最低亮度
	if got := scaleBrightness(0.15, 0.6); !approx(got, 0.1) {
		t.Errorf("触底钳制: got %v, want 0.1", got)
	}
}
