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

package gesture

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	GESTURE_DIRECTION_NONE  = 0
	GESTURE_DIRECTION_UP    = 10
	GESTURE_DIRECTION_DOWN  = 11
	GESTURE_DIRECTION_LEFT  = 12
	GESTURE_DIRECTION_RIGHT = 13
	GESTURE_DIRECTION_IN    = 14
	GESTURE_DIRECTION_OUT   = 15
	TOUCH_TYPE_RIGHT_BUTTON = 50

	GESTURE_TYPE_SWIPE = 100
	GESTURE_TYPE_PINCH = 101
	GESTURE_TYPE_TAP   = 102
	BUTTON_TYPE_DOWN   = 501
	BUTTON_TYPE_UP     = 502

	DIR_NONE  = 0
	DIR_TOP   = 1
	DIR_RIGHT = 2
	DIR_BOT   = 3
	DIR_LEFT  = 4

	GT_NONE     = 0
	GT_TAP      = 1
	GT_MOVEMENT = 2
	GT_EDGE     = 3

	UNKNOWN = 999
)

func Test_String_GestureType(t *testing.T) {
	m1 := map[int]string{
		GESTURE_DIRECTION_NONE:  "none",
		GESTURE_DIRECTION_UP:    "up",
		GESTURE_DIRECTION_DOWN:  "down",
		GESTURE_DIRECTION_LEFT:  "left",
		GESTURE_DIRECTION_RIGHT: "right",
		GESTURE_DIRECTION_IN:    "in",
		GESTURE_DIRECTION_OUT:   "out",
		GESTURE_TYPE_SWIPE:      "swipe",
		GESTURE_TYPE_PINCH:      "pinch",
		GESTURE_TYPE_TAP:        "tap",
		UNKNOWN:                 "Unknown",
	}

	g := GestureType(GESTURE_TYPE_SWIPE)
	rtn := g.String()
	assert.Equal(t, m1[GESTURE_TYPE_SWIPE], rtn)

	g = GestureType(GESTURE_TYPE_PINCH)
	rtn = g.String()
	assert.Equal(t, m1[GESTURE_TYPE_PINCH], rtn)

	g = GestureType(GESTURE_TYPE_TAP)
	rtn = g.String()
	assert.Equal(t, m1[GESTURE_TYPE_TAP], rtn)

	g = GestureType(GESTURE_DIRECTION_NONE)
	rtn = g.String()
	assert.Equal(t, m1[GESTURE_DIRECTION_NONE], rtn)

	g = GestureType(GESTURE_DIRECTION_UP)
	rtn = g.String()
	assert.Equal(t, m1[GESTURE_DIRECTION_UP], rtn)

	g = GestureType(GESTURE_DIRECTION_DOWN)
	rtn = g.String()
	assert.Equal(t, m1[GESTURE_DIRECTION_DOWN], rtn)

	g = GestureType(GESTURE_DIRECTION_LEFT)
	rtn = g.String()
	assert.Equal(t, m1[GESTURE_DIRECTION_LEFT], rtn)

	g = GestureType(GESTURE_DIRECTION_RIGHT)
	rtn = g.String()
	assert.Equal(t, m1[GESTURE_DIRECTION_RIGHT], rtn)

	g = GestureType(GESTURE_DIRECTION_IN)
	rtn = g.String()
	assert.Equal(t, m1[GESTURE_DIRECTION_IN], rtn)

	g = GestureType(GESTURE_DIRECTION_OUT)
	rtn = g.String()
	assert.Equal(t, m1[GESTURE_DIRECTION_OUT], rtn)

	g = GestureType(UNKNOWN)
	rtn = g.String()
	assert.Equal(t, m1[UNKNOWN], rtn)
}

func Test_String_TouchType(t *testing.T) {
	m1 := map[int]string{
		TOUCH_TYPE_RIGHT_BUTTON: "touch right button",
		BUTTON_TYPE_DOWN:        "down",
		BUTTON_TYPE_UP:          "up",
		GT_NONE:                 "touch none",
		GT_TAP:                  "touch tap",
		GT_MOVEMENT:             "touch movement",
		GT_EDGE:                 "touch edge",
		UNKNOWN:                 "Unknown",
	}

	g := TouchType(TOUCH_TYPE_RIGHT_BUTTON)
	rtn := g.String()
	assert.Equal(t, m1[TOUCH_TYPE_RIGHT_BUTTON], rtn)

	g = TouchType(BUTTON_TYPE_DOWN)
	rtn = g.String()
	assert.Equal(t, m1[BUTTON_TYPE_DOWN], rtn)

	g = TouchType(BUTTON_TYPE_UP)
	rtn = g.String()
	assert.Equal(t, m1[BUTTON_TYPE_UP], rtn)

	g = TouchType(GT_NONE)
	rtn = g.String()
	assert.Equal(t, m1[GT_NONE], rtn)

	g = TouchType(GT_TAP)
	rtn = g.String()
	assert.Equal(t, m1[GT_TAP], rtn)

	g = TouchType(GT_MOVEMENT)
	rtn = g.String()
	assert.Equal(t, m1[GT_MOVEMENT], rtn)

	g = TouchType(GT_EDGE)
	rtn = g.String()
	assert.Equal(t, m1[GT_EDGE], rtn)

	g = TouchType(UNKNOWN)
	rtn = g.String()
	assert.Equal(t, m1[UNKNOWN], rtn)
}

func Test_String_TouchDirection(t *testing.T) {
	m1 := map[int]string{
		DIR_NONE:  "none",
		DIR_TOP:   "top",
		DIR_RIGHT: "right",
		DIR_BOT:   "bot",
		DIR_LEFT:  "left",
		UNKNOWN:   "Unknown",
	}

	g := TouchDirection(DIR_NONE)
	rtn := g.String()
	assert.Equal(t, m1[DIR_NONE], rtn)

	g = TouchDirection(DIR_TOP)
	rtn = g.String()
	assert.Equal(t, m1[DIR_TOP], rtn)

	g = TouchDirection(DIR_RIGHT)
	rtn = g.String()
	assert.Equal(t, m1[DIR_RIGHT], rtn)

	g = TouchDirection(DIR_BOT)
	rtn = g.String()
	assert.Equal(t, m1[DIR_BOT], rtn)

	g = TouchDirection(DIR_LEFT)
	rtn = g.String()
	assert.Equal(t, m1[DIR_LEFT], rtn)

	g = TouchDirection(UNKNOWN)
	rtn = g.String()
	assert.Equal(t, m1[UNKNOWN], rtn)
}

func Test_SimpleFunc(t *testing.T) {
	d := Daemon{}
	d.GetDependencies()
	d.SetLongPressDuration(1)

	m := Manager{}
	m.GetInterfaceName()
	m.SetShortPressDuration(1)
	m.SetEdgeMoveStopDuration(1)
	m.SetInputIgnore("", false)
}
