// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

func TestGetStdMonitorNameAt(t *testing.T) {
	base := bytes.Repeat([]byte{0x42}, 128)
	full := append(append([]byte(nil), base...), bytes.Repeat([]byte{0x24}, 128)...)
	other := bytes.Repeat([]byte{0x13}, 128)
	tests := []struct {
		name       string
		connectors map[string][]byte
		edid       []byte
		want       string
		wantErr    bool
	}{
		{"unique", map[string][]byte{"card0-HDMI-A-2": base, "card0-HDMI-A-3": other}, base, "HDMI-A-2", false},
		{"identical EDIDs", map[string][]byte{"card0-HDMI-A-2": full, "card0-HDMI-A-3": full}, full, "", true},
		{"identical base blocks", map[string][]byte{"card0-HDMI-A-2": full, "card0-HDMI-A-3": full}, base, "", true},
		{"multiple GPUs", map[string][]byte{"card0-HDMI-A-2": full, "card1-HDMI-A-2": full}, full, "", true},
		{"unique base block", map[string][]byte{"card0-HDMI-A-2": full}, base, "HDMI-A-2", false},
		{"disconnected connector", map[string][]byte{"card0-HDMI-A-2": base, "card0-HDMI-A-3": nil}, base, "HDMI-A-2", false},
		{"no match", map[string][]byte{"card0-HDMI-A-2": other}, base, "", true},
		{"ignore nonconnector", map[string][]byte{"unrelated-HDMI-A-2": base}, base, "", true},
		{"missing EDID file", map[string][]byte{"card0-HDMI-A-2": base, "card0-HDMI-A-3": nil}, full, "", true},
		{"empty EDID", map[string][]byte{"card0-HDMI-A-2": nil}, nil, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for connector, edid := range tt.connectors {
				path := filepath.Join(dir, connector)
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				if edid != nil {
					if err := os.WriteFile(filepath.Join(path, "edid"), edid, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			got, err := getStdMonitorNameAt(dir, 0, tt.edid)
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("getStdMonitorNameAt = (%q, %v), want (%q, error=%v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestGetStdMonitorNameAtMissingDirectory(t *testing.T) {
	got, err := getStdMonitorNameAt(filepath.Join(t.TempDir(), "missing"), 0, []byte{1})
	if got != "" || err == nil {
		t.Fatalf("missing directory = (%q, %v), want error", got, err)
	}
}

func TestGetStdMonitorNameConnectorID(t *testing.T) {
	edid := bytes.Repeat([]byte{0x42}, 256)
	other := bytes.Repeat([]byte{0x13}, 256)
	type connector struct {
		name string
		id   string
		edid []byte
	}
	ports := []connector{{"card0-HDMI-A-2", "281\n", edid}, {"card0-HDMI-A-3", "285\n", edid}}
	tests := []struct {
		name       string
		connectors []connector
		id         uint32
		edid       []byte
		want       string
		wantErr    bool
	}{
		{"same EDID first port", ports, 281, edid, "HDMI-A-2", false},
		{"same EDID second port", ports, 285, edid, "HDMI-A-3", false},
		{"base EDID second port", ports, 285, edid[:128], "HDMI-A-3", false},
		{"missing property unique EDID", ports[:1], 0, edid, "HDMI-A-2", false},
		{"missing property ambiguous EDID", ports, 0, edid, "", true},
		{"unknown connector ID", ports[:1], 999, edid, "", true},
		{"EDID mismatch", []connector{{"card0-HDMI-A-2", "281", other}, {"card0-HDMI-A-3", "285", edid}}, 281, edid, "", true},
		{"GPU-local IDs with distinct EDIDs", []connector{{"card0-HDMI-A-2", "281", other}, {"card1-HDMI-A-3", "281", edid}}, 281, edid, "HDMI-A-3", false},
		{"ambiguous GPU-local IDs", []connector{{"card0-HDMI-A-2", "281", edid}, {"card1-HDMI-A-3", "281", edid}}, 281, edid, "", true},
		{"ambiguous standard names", []connector{{"card0-HDMI-A-2", "281", edid}, {"card1-HDMI-A-2", "285", edid}}, 285, edid, "", true},
		{"missing connector ID file", []connector{{"card0-HDMI-A-2", "", edid}}, 281, edid, "", true},
		{"malformed connector ID", []connector{{"card0-HDMI-A-2", "invalid", edid}}, 281, edid, "", true},
		{"overflow connector ID", []connector{{"card0-HDMI-A-2", "4294967296", edid}}, 281, edid, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, port := range tt.connectors {
				path := filepath.Join(dir, port.name)
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "edid"), port.edid, 0o600); err != nil {
					t.Fatal(err)
				}
				if port.id != "" {
					if err := os.WriteFile(filepath.Join(path, "connector_id"), []byte(port.id), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			got, err := getStdMonitorNameAt(dir, tt.id, tt.edid)
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("connector ID %d = (%q, %v), want (%q, error=%v)", tt.id, got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestDecodeOutputConnectorID(t *testing.T) {
	tests := []struct {
		name  string
		reply *randr.GetOutputPropertyReply
		want  uint32
	}{
		{"integer", &randr.GetOutputPropertyReply{Type: x.AtomInteger, Format: 32, ValueLen: 1, Value: []byte{0x19, 1, 0, 0}}, 281},
		{"cardinal", &randr.GetOutputPropertyReply{Type: x.AtomCardinal, Format: 32, ValueLen: 1, Value: []byte{0x1d, 1, 0, 0}}, 285},
		{"nil", nil, 0},
		{"absent", &randr.GetOutputPropertyReply{}, 0},
		{"wrong type", &randr.GetOutputPropertyReply{Type: x.AtomString, Format: 32, ValueLen: 1, Value: []byte{1, 0, 0, 0}}, 0},
		{"wrong format", &randr.GetOutputPropertyReply{Type: x.AtomInteger, Format: 8, ValueLen: 4, Value: []byte{1, 0, 0, 0}}, 0},
		{"wrong count", &randr.GetOutputPropertyReply{Type: x.AtomInteger, Format: 32, ValueLen: 2, Value: []byte{1, 0, 0, 0}}, 0},
		{"short data", &randr.GetOutputPropertyReply{Type: x.AtomInteger, Format: 32, ValueLen: 1, Value: []byte{1}}, 0},
		{"long data", &randr.GetOutputPropertyReply{Type: x.AtomInteger, Format: 32, ValueLen: 1, Value: []byte{1, 0, 0, 0, 2, 0, 0, 0}}, 0},
		{"truncated property", &randr.GetOutputPropertyReply{Type: x.AtomInteger, Format: 32, ValueLen: 1, BytesAfter: 4, Value: []byte{1, 0, 0, 0}}, 0},
		{"zero ID", &randr.GetOutputPropertyReply{Type: x.AtomInteger, Format: 32, ValueLen: 1, Value: []byte{0, 0, 0, 0}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeOutputConnectorID(tt.reply)
			if got != tt.want || (err != nil) != (tt.want == 0) {
				t.Fatalf("decodeOutputConnectorID = (%d, %v), want %d", got, err, tt.want)
			}
		})
	}
}
