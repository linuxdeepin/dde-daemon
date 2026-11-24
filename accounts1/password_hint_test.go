// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePasswordHint(t *testing.T) {
	tests := []struct {
		name    string
		hint    string
		wantErr bool
	}{
		{"empty is allowed and clears the hint", "", false},
		{"plain text", "my first pet", false},
		{"chinese text", "第一只宠物的名字", false},
		{"at the limit", strings.Repeat("a", maxPasswordHintLength), false},
		{"over the limit", strings.Repeat("a", maxPasswordHintLength+1), true},
		{"multi-byte counted in characters", strings.Repeat("密", maxPasswordHintLength), false},
		{"newline", "a\nb", true},
		{"carriage return", "a\rb", true},
		{"tab", "a\tb", true},
		{"del", "a\x7fb", true},
		{"nul", "a\x00b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePasswordHint(tt.hint)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
