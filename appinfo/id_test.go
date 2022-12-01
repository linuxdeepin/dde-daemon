/*
 * Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:     zsien <i@zsien.cn>
 *
 * Maintainer: zsien <i@zsien.cn>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
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
