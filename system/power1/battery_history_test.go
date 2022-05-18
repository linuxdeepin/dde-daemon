/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
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

package power1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getHistoryLength(t *testing.T) {
	bat := Battery{}
	bat.batteryHistory = []float64{1.0, 2.0}
	len := bat.getHistoryLength()
	assert.Equal(t, 2, len)
}

func Test_appendToHistory(t *testing.T) {
	bat := Battery{}

	bat.appendToHistory(1.0)
	assert.Equal(t, 1.0, bat.batteryHistory[0])

	bat.appendToHistory(2.0)
	bat.appendToHistory(3.0)
	bat.appendToHistory(4.0)
	bat.appendToHistory(5.0)
	bat.appendToHistory(6.0)
	bat.appendToHistory(7.0)
	bat.appendToHistory(8.0)
	bat.appendToHistory(9.0)
	bat.appendToHistory(10.0)
	assert.Equal(t, 10.0, bat.batteryHistory[9])
}

func Test_calcHistoryVariance(t *testing.T) {
	bat := Battery{}

	bat.batteryHistory = []float64{1.0, 2.0}
	variance := bat.calcHistoryVariance()
	assert.Equal(t, 0.25, variance)
}
