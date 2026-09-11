// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package bluetooth

import "testing"

func TestDebugInfoDoesNotExposeBluetoothDetails(t *testing.T) {
	b := &SysBluetooth{}

	info, err := b.DebugInfo()
	if err != nil {
		t.Fatalf("DebugInfo returned an unexpected error: %v", err)
	}
	if want := ""; info != want {
		t.Fatalf("DebugInfo returned %q, want %q", info, want)
	}
}
