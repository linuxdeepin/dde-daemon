// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package brightness

import (
	"strings"
	"testing"
)

func TestInterpolateCurveValue(t *testing.T) {
	points := []BrightnessPoint{
		{Percentage: 0, Value: 1},
		{Percentage: 50, Value: 12},
		{Percentage: 100, Value: 99},
	}

	tests := []struct {
		name       string
		percentage float64
		want       float64
	}{
		{"below first point", -5, 1},
		{"first point", 0, 1},
		{"between points", 25, 6.5},
		{"exact point", 50, 12},
		{"above last point", 120, 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := interpolateCurveValue(tt.percentage, points); got != tt.want {
				t.Errorf("interpolateCurveValue(%v) = %v, want %v", tt.percentage, got, tt.want)
			}
		})
	}
}

func TestInterpolateCurveValueOutsidePointRange(t *testing.T) {
	// 曲线的最后一个点不到 100% 时，超过该点的输入不能回退到第一个点的值。
	points := []BrightnessPoint{
		{Percentage: 0, Value: 1},
		{Percentage: 90, Value: 50},
	}
	if got := interpolateCurveValue(95, points); got != 50 {
		t.Errorf("interpolateCurveValue(95) = %v, want 50", got)
	}

	// 第一个点大于 0 时，低于该点的输入使用第一个点的值。
	points = []BrightnessPoint{
		{Percentage: 10, Value: 20},
		{Percentage: 90, Value: 60},
	}
	if got := interpolateCurveValue(5, points); got != 20 {
		t.Errorf("interpolateCurveValue(5) = %v, want 20", got)
	}
}

func TestInterpolateCurveValueEmpty(t *testing.T) {
	if got := interpolateCurveValue(50, nil); got != 0 {
		t.Errorf("interpolateCurveValue(50, nil) = %v, want 0", got)
	}
}

func TestValidateBrightnessCurve(t *testing.T) {
	valid := BrightnessCurveConfig{
		CurvePoints: []BrightnessPoint{
			{Percentage: 0, Value: 1},
			{Percentage: 50, Value: 12},
			{Percentage: 100, Value: 99},
		},
	}

	tests := []struct {
		name    string
		curve   BrightnessCurveConfig
		wantErr bool
	}{
		{"valid", valid, false},
		{"empty points", BrightnessCurveConfig{}, true},
		{"negative max limit", BrightnessCurveConfig{MaxLimit: -1, CurvePoints: valid.CurvePoints}, true},
		{"max limit too large", BrightnessCurveConfig{MaxLimit: 101, CurvePoints: valid.CurvePoints}, true},
		{"negative max scale", BrightnessCurveConfig{MaxScale: -1, CurvePoints: valid.CurvePoints}, true},
		{"negative percentage", BrightnessCurveConfig{CurvePoints: []BrightnessPoint{{Percentage: -1, Value: 1}}}, true},
		{"percentage too large", BrightnessCurveConfig{CurvePoints: []BrightnessPoint{{Percentage: 101, Value: 1}}}, true},
		{"negative value", BrightnessCurveConfig{CurvePoints: []BrightnessPoint{{Percentage: 0, Value: -1}}}, true},
		{"value too large", BrightnessCurveConfig{CurvePoints: []BrightnessPoint{{Percentage: 0, Value: 101}}}, true},
		{"descending percentages", BrightnessCurveConfig{CurvePoints: []BrightnessPoint{
			{Percentage: 50, Value: 12},
			{Percentage: 40, Value: 10},
		}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBrightnessCurve(tt.curve)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBrightnessCurve() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnmarshalCurveJSON(t *testing.T) {
	raw := `{"curve_points":[{"percentage":0,"value":1},{"percentage":100,"value":99}]}`

	var curve BrightnessCurveConfig
	if err := unmarshalCurveJSON(raw, &curve); err != nil {
		t.Fatalf("unmarshal raw JSON failed: %v", err)
	}
	if len(curve.CurvePoints) != 2 {
		t.Fatalf("unexpected curve points: %v", curve.CurvePoints)
	}

	// 兼容旧配置中作为转义字符串保存的 JSON。
	escaped := strings.ReplaceAll(raw, `"`, `\"`)
	curve = BrightnessCurveConfig{}
	if err := unmarshalCurveJSON(escaped, &curve); err != nil {
		t.Fatalf("unmarshal escaped JSON failed: %v", err)
	}
	if len(curve.CurvePoints) != 2 {
		t.Fatalf("unexpected curve points from escaped JSON: %v", curve.CurvePoints)
	}

	// 合法 JSON 字符串中的反斜杠不能被破坏。
	withBackslash := `{"curve_points":[{"percentage":0,"value":1},{"percentage":100,"value":99}],"edid":"vendor\\model"}`
	curve = BrightnessCurveConfig{}
	if err := unmarshalCurveJSON(withBackslash, &curve); err != nil {
		t.Fatalf("unmarshal JSON with backslash failed: %v", err)
	}
	if curve.EDID != `vendor\model` {
		t.Errorf("EDID = %q, want %q", curve.EDID, `vendor\model`)
	}
}

func TestChooseBigProduct(t *testing.T) {
	cm := &CurveManager{}
	names := []string{"ProductA", "ProductB"}
	cm.setChooseBigProductNames(names)
	// 修改调用方切片不应影响 CurveManager 内部状态。
	names[0] = "Changed"

	cm.setProductName("ProductA")
	if !cm.isChooseBigProduct() {
		t.Error("expected ProductA to match choose-big list")
	}

	cm.setProductName("ProductC")
	if cm.isChooseBigProduct() {
		t.Error("did not expect ProductC to match choose-big list")
	}
}

func TestParseDefaultBrightnessCurveFromOverride(t *testing.T) {
	// 与 4100-override.curve.json 中的 default-brightness-curve 保持一致。
	const curveJSON = `{"curve_points":[{"percentage":0,"value":1},{"percentage":5,"value":2},{"percentage":10,"value":3},{"percentage":15,"value":4},{"percentage":20,"value":5},{"percentage":25,"value":6},{"percentage":30,"value":7},{"percentage":35,"value":8},{"percentage":40,"value":9},{"percentage":45,"value":10},{"percentage":50,"value":12},{"percentage":55,"value":14},{"percentage":60,"value":18},{"percentage":65,"value":23},{"percentage":70,"value":27},{"percentage":75,"value":33},{"percentage":80,"value":39},{"percentage":85,"value":53},{"percentage":90,"value":67},{"percentage":95,"value":84},{"percentage":100,"value":99}]}`

	cm := &CurveManager{}
	if err := cm.parseDefaultBrightnessCurve(curveJSON); err != nil {
		t.Fatalf("parseDefaultBrightnessCurve() error = %v", err)
	}
	if cm.defaultCurveFunc == nil {
		t.Fatal("defaultCurveFunc is nil")
	}

	tests := []struct {
		percentage float64
		want       int32
	}{
		{0, 3},
		{50, 31},
		{100, 252},
	}
	for _, tt := range tests {
		if got := cm.defaultCurveFunc(tt.percentage, 255); got != tt.want {
			t.Errorf("defaultCurveFunc(%v, 255) = %d, want %d", tt.percentage, got, tt.want)
		}
	}
}
