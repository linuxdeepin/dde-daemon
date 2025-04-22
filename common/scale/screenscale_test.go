// SPDX-FileCopyrightText: 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package scale

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCalcRecommendedScaleFactor(t *testing.T) {
	for _, rec := range []struct {
		widthPx  float64
		heightPx float64
		widthMm  float64
		heightMm float64
		expect   float64
	}{
		{1366, 768, 310, 147, 1},
		{1366, 768, 277, 165, 1},
		{1366, 768, 309, 174, 1},

		{1600, 900, 294, 166, 1},

		{1920, 1080, 344, 194, 1.25},
		{1920, 1080, 477, 268, 1},
		{1920, 1080, 527, 296, 1},
		{1920, 1080, 476, 268, 1},
		{1920, 1080, 520, 310, 1},
		{1920, 1080, 708, 398, 1},
		{1920, 1080, 518, 324, 1},
		{1920, 1080, 510, 287, 1},
		{1920, 1080, 527, 296, 1},
		{1920, 1080, 309, 174, 1.25},
		{1920, 1080, 293, 165, 1.25},
		{1920, 1080, 294, 165, 1.25},

		{2160, 1440, 280, 180, 1.5},

		{3000, 2000, 290, 200, 2},

		{3840, 2160, 600, 340, 2},
		{3840, 2160, 344, 193, 2.25},
	} {
		factor := calcRecommendedScaleFactor(rec.widthPx, rec.heightPx, rec.widthMm, rec.heightMm)
		assert.Equal(t, rec.expect, factor, "%gx%g %gmm x %gmm",
			rec.widthPx, rec.heightPx, rec.widthMm, rec.heightMm)
	}
}
