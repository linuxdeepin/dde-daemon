// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package brightness

import "testing"

func TestFillColorRamp(t *testing.T) {
	const size = 1024
	r, g, b := initGammaRamp(size)
	rCp := make([]uint16, size)
	gCp := make([]uint16, size)
	bCp := make([]uint16, size)
	for br := 0.0; br < 1.0; br += 0.05 {
		for temp := 1000; temp <= 25000; temp += 1 {
			copy(rCp, r)
			copy(gCp, g)
			copy(bCp, b)
			fillColorRamp(rCp, gCp, bCp, gammaSetting{
				brightness:  br,
				temperature: temp,
			})
		}
	}
}
