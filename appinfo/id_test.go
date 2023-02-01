// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package appinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeAppID(t *testing.T) {
	tests := []struct {
		name        string
		candidateID string
		want        string
	}{
		{
			name:        "NormalizeAppID",
			candidateID: "org.deepin.Control_Center",
			want:        "org.deepin.control-center",
		},
		{
			name:        "NormalizeAppID",
			candidateID: "org.deepin.ControlCenter",
			want:        "org.deepin.controlcenter",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeAppID(tt.candidateID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeAppIDWithCaseSensitive(t *testing.T) {
	tests := []struct {
		name        string
		candidateID string
		want        string
	}{
		{
			name:        "NormalizeAppIDWithCaseSensitive",
			candidateID: "org.deepin.Control_Center",
			want:        "org.deepin.Control-Center",
		},
		{
			name:        "NormalizeAppIDWithCaseSensitive not change",
			candidateID: "org.deepin.ControlCenter",
			want:        "org.deepin.ControlCenter",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeAppIDWithCaseSensitive(tt.candidateID)
			assert.Equal(t, tt.want, got)
		})
	}
}
