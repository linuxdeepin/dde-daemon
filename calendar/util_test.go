/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     hubenchang <hubenchang@uniontech.com>
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
package calendar

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_newTimeYMDHM(t *testing.T) {
	testData := []struct {
		year   int
		month  time.Month
		day    int
		hour   int
		minute int
		t      time.Time
	}{
		{
			year:   1995,
			month:  time.May,
			day:    15,
			hour:   2,
			minute: 36,
			t:      time.Date(1995, time.May, 15, 2, 36, 0, 0, time.Local),
		},
	}

	for _, data := range testData {
		assert.Equal(t, data.t, newTimeYMDHM(data.year, data.month, data.day, data.hour, data.minute))
	}
}

func Test_newTimeYMDHMS(t *testing.T) {
	testData := []struct {
		year   int
		month  time.Month
		day    int
		hour   int
		minute int
		second int
		t      time.Time
	}{
		{
			year:   1995,
			month:  time.May,
			day:    15,
			hour:   2,
			minute: 36,
			second: 29,
			t:      time.Date(1995, time.May, 15, 2, 36, 29, 0, time.Local),
		},
	}

	for _, data := range testData {
		assert.Equal(t, data.t, newTimeYMDHMS(data.year, data.month, data.day, data.hour, data.minute, data.second))
	}
}

func Test_formatTime(t *testing.T) {
	tm := time.Now()
	assert.Equal(t, tm.Format("2006-01-02 15:04"), formatTime(tm))
}

func Test_setClock(t *testing.T) {
	t1 := time.Now()
	c := Clock{
		Hour:   10,
		Minute: 10,
		Second: 10,
	}
	t2 := time.Date(t1.Year(), t1.Month(), t1.Day(), c.Hour, c.Minute, c.Second, t1.Nanosecond(), t1.Location())
	assert.Equal(t, t2, setClock(t1, c))
}

func Test_isZH(t *testing.T) {
	lang := os.Getenv("LANG")
	is := strings.HasPrefix(lang, "zh")
	assert.Equal(t, is, isZH())
}
