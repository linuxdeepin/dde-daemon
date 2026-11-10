// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeTestEdid(t *testing.T, dir, connector string, edid []byte) {
	t.Helper()

	path := filepath.Join(dir, "card0-"+connector)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s failed: %v", path, err)
	}
	if err := os.WriteFile(filepath.Join(path, "edid"), edid, 0644); err != nil {
		t.Fatalf("write edid failed: %v", err)
	}
}

func TestGetStdMonitorNameIdenticalEdid(t *testing.T) {
	dir := t.TempDir()
	edid := bytes.Repeat([]byte{0x12}, 256)
	connectors := []string{"DP-1", "DP-2", "DP-3", "DP-4"}
	for _, connector := range connectors {
		writeTestEdid(t, dir, connector, edid)
	}

	for _, connector := range connectors {
		got, err := getStdMonitorNameFromDir(dir, connector, edid[:128])
		if err != nil {
			t.Fatalf("getStdMonitorNameFromDir(%q) failed: %v", connector, err)
		}
		if got != connector {
			t.Fatalf("getStdMonitorNameFromDir(%q) = %q, want %q", connector, got, connector)
		}
	}
}

func TestGetStdMonitorNameAmbiguousEdid(t *testing.T) {
	dir := t.TempDir()
	edid := bytes.Repeat([]byte{0x34}, 256)
	writeTestEdid(t, dir, "HDMI-A-1", edid)
	writeTestEdid(t, dir, "HDMI-A-2", edid)

	if _, err := getStdMonitorNameFromDir(dir, "HDMI-0", edid[:128]); err == nil {
		t.Fatal("getStdMonitorNameFromDir should fail when EDID matches multiple connectors")
	}
}

func TestGetStdMonitorNameUniqueNonExact(t *testing.T) {
	dir := t.TempDir()
	edid := bytes.Repeat([]byte{0x56}, 256)
	writeTestEdid(t, dir, "eDP-1", edid)

	got, err := getStdMonitorNameFromDir(dir, "eDP-0", edid[:128])
	if err != nil {
		t.Fatalf("getStdMonitorNameFromDir failed: %v", err)
	}
	if got != "eDP-1" {
		t.Fatalf("getStdMonitorNameFromDir = %q, want eDP-1", got)
	}
}

func TestGetOutputUuidIdenticalEdidUsesDistinctStdNames(t *testing.T) {
	edid := bytes.Repeat([]byte{0x78}, 128)
	seen := make(map[string]string)
	for _, connector := range []string{"DP-1", "DP-2", "DP-3", "DP-4"} {
		uuid := getOutputUuid(connector, connector, edid)
		if other, ok := seen[uuid]; ok {
			t.Fatalf("connector %s and %s got the same UUID %s", other, connector, uuid)
		}
		seen[uuid] = connector
	}
}

func TestSysMonitorConfigsGetByUuidAndName(t *testing.T) {
	const uuid = "@DP-1|hash|v1"
	cfgs := SysMonitorConfigs{
		{UUID: uuid, Name: "DP-4", X: 0},
		{UUID: uuid, Name: "DP-1", X: 3840},
		{UUID: uuid, Name: "DP-2", X: 7680},
		{UUID: uuid, Name: "DP-3", X: 11520},
	}

	want := map[string]int16{
		"DP-1": 3840,
		"DP-2": 7680,
		"DP-3": 11520,
		"DP-4": 0,
	}
	for name, wantX := range want {
		cfg := cfgs.getByUuidAndName(uuid, name)
		if cfg == nil {
			t.Fatalf("getByUuidAndName(%q) returned nil", name)
		}
		if cfg.X != wantX {
			t.Fatalf("getByUuidAndName(%q).X = %d, want %d", name, cfg.X, wantX)
		}
	}

	if cfg := cfgs.getByUuidAndName(uuid, "DP-9"); cfg == nil {
		t.Fatal("getByUuidAndName should fall back to UUID for unknown name")
	}

	cfgs.setPrimaryByUuidAndName(uuid, "DP-2")
	primaryCount := 0
	for _, cfg := range cfgs {
		if cfg.Primary {
			primaryCount++
			if cfg.Name != "DP-2" {
				t.Fatalf("unexpected primary config: %+v", cfg)
			}
		}
	}
	if primaryCount != 1 {
		t.Fatalf("primary count = %d, want 1", primaryCount)
	}
}

func TestSysMonitorConfigsGetByUuidAndNameUniqueFallback(t *testing.T) {
	cfgs := SysMonitorConfigs{
		{UUID: "unique-uuid", Name: "DP-1", X: 3840},
	}

	cfg := cfgs.getByUuidAndName("unique-uuid", "DP-9")
	if cfg == nil {
		t.Fatal("getByUuidAndName should fall back to unique UUID")
	}
	if cfg.X != 3840 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestMonitorsGetByUuidAndNameOrUuid(t *testing.T) {
	monitors := Monitors{
		&Monitor{Name: "DP-1", uuid: "duplicate-uuid"},
		&Monitor{Name: "DP-2", uuid: "duplicate-uuid"},
	}

	if got := monitors.GetByUuidAndNameOrUuid("duplicate-uuid", "DP-2"); got == nil || got.Name != "DP-2" {
		t.Fatalf("GetByUuidAndNameOrUuid returned %+v, want DP-2", got)
	}
	if got := monitors.GetByUuidAndNameOrUuid("duplicate-uuid", "DP-9"); got == nil {
		t.Fatal("GetByUuidAndNameOrUuid should fall back to UUID for unknown name")
	}

	unique := Monitors{&Monitor{Name: "DP-1", uuid: "unique-uuid"}}
	if got := unique.GetByUuidAndNameOrUuid("unique-uuid", "DP-9"); got == nil || got.Name != "DP-1" {
		t.Fatalf("GetByUuidAndNameOrUuid unique fallback returned %+v, want DP-1", got)
	}
}
