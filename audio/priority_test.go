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

package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_contains(t *testing.T) {
	assert.True(t, contains("hbc.abcd.1234", "world.abcd.1234", "hbc"))
	assert.True(t, contains("hello.abcd.1234", "hbc.abcd.1234", "hbc"))
	assert.True(t, contains("HBC.abcd.1234", "world.abcd.1234", "hbc"))
	assert.True(t, contains("hello.abcd.1234", "HBC.abcd.1234", "hbc"))
	assert.False(t, contains("hello.abcd.1234", "world.abcd.1234", "hbc"))
}

func Test_GetPortType(t *testing.T) {
	assert.Equal(t, GetPortType("hbc.abcd.1234", "world.abcd.1234"), PortTypeUnknown)
	assert.Equal(t, GetPortType("bluez.abcd.1234", "world.abcd.1234"), PortTypeBluetooth)
	assert.Equal(t, GetPortType("hbc.abcd.1234", "bluez.abcd.1234"), PortTypeBluetooth)
	assert.Equal(t, GetPortType("usb.abcd.1234", "world.abcd.1234"), PortTypeHeadset)
	assert.Equal(t, GetPortType("hbc.abcd.1234", "usb.abcd.1234"), PortTypeHeadset)
	assert.Equal(t, GetPortType("hello.abcd.speaker", "world.abcd.1234"), PortTypeBuiltin)
	assert.Equal(t, GetPortType("hdmi.abcd.speaker", "world.abcd.1234"), PortTypeHdmi)
}

func Test_IsInputTypeAfter(t *testing.T) {
	pr := NewPriorities()
	pr.defaultInit(CardList{})
	assert.False(t, pr.IsInputTypeAfter(PortTypeHeadset, PortTypeBluetooth))
	assert.False(t, pr.IsInputTypeAfter(PortTypeBuiltin, PortTypeBluetooth))
	assert.False(t, pr.IsInputTypeAfter(PortTypeHdmi, PortTypeBluetooth))
	assert.False(t, pr.IsInputTypeAfter(PortTypeBuiltin, PortTypeHeadset))
	assert.False(t, pr.IsInputTypeAfter(PortTypeHdmi, PortTypeBuiltin))

	assert.True(t, pr.IsInputTypeAfter(PortTypeBluetooth, PortTypeHeadset))
	assert.True(t, pr.IsInputTypeAfter(PortTypeBluetooth, PortTypeBuiltin))
	assert.True(t, pr.IsInputTypeAfter(PortTypeBluetooth, PortTypeHdmi))
	assert.True(t, pr.IsInputTypeAfter(PortTypeHeadset, PortTypeBuiltin))
	assert.True(t, pr.IsInputTypeAfter(PortTypeBuiltin, PortTypeHdmi))
}

func Test_IsOutputTypeAfter(t *testing.T) {
	pr := NewPriorities()
	pr.defaultInit(CardList{})
	assert.False(t, pr.IsOutputTypeAfter(PortTypeHeadset, PortTypeBluetooth))
	assert.False(t, pr.IsOutputTypeAfter(PortTypeBuiltin, PortTypeBluetooth))
	assert.False(t, pr.IsOutputTypeAfter(PortTypeHdmi, PortTypeBluetooth))
	assert.False(t, pr.IsOutputTypeAfter(PortTypeBuiltin, PortTypeHeadset))
	assert.False(t, pr.IsOutputTypeAfter(PortTypeHdmi, PortTypeBuiltin))

	assert.True(t, pr.IsOutputTypeAfter(PortTypeBluetooth, PortTypeHeadset))
	assert.True(t, pr.IsOutputTypeAfter(PortTypeBluetooth, PortTypeBuiltin))
	assert.True(t, pr.IsOutputTypeAfter(PortTypeBluetooth, PortTypeHdmi))
	assert.True(t, pr.IsOutputTypeAfter(PortTypeHeadset, PortTypeBuiltin))
	assert.True(t, pr.IsOutputTypeAfter(PortTypeBuiltin, PortTypeHdmi))
}

func Test_NewPriorityManager(t *testing.T) {
	pm := NewPriorityManager("testdata/priorities.json")
	assert.NotNil(t, pm)
}

func Test_DefaultInit(t *testing.T) {
	pm := NewPriorityManager("testdata/priorities.json")
	pm.Init(CardList{})

	for t1 := 0; t1 < PortTypeCount; t1++ {
		for t2 := t1 + 1; t2 < PortTypeCount; t2++ {
			assert.Equal(t, t1, pm.Output.GetPreferType(t1, t2))
			assert.Equal(t, t1, pm.Output.GetPreferType(t2, t1))
			assert.Equal(t, t1, pm.Input.GetPreferType(t1, t2))
			assert.Equal(t, t1, pm.Input.GetPreferType(t2, t1))
		}
	}
}

func Test_SetFirstType(t *testing.T) {
	pm := NewPriorityManager("testdata/priorities.json")
	pm.Init(CardList{})

	pm.Output.SetTheFirstType(PortTypeMultiChannel)
	for t1 := 0; t1 < PortTypeCount; t1++ {
		assert.Equal(t, PortTypeMultiChannel, pm.Output.GetPreferType(t1, PortTypeMultiChannel))
		assert.Equal(t, PortTypeMultiChannel, pm.Output.GetPreferType(PortTypeMultiChannel, t1))
	}

	pm.Output.SetTheFirstType(PortTypeHdmi)
	for t1 := 0; t1 < PortTypeCount; t1++ {
		assert.Equal(t, PortTypeHdmi, pm.Output.GetPreferType(t1, PortTypeHdmi))
		assert.Equal(t, PortTypeHdmi, pm.Output.GetPreferType(PortTypeHdmi, t1))
	}
}

func Test_AddPort(t *testing.T) {
	pm := NewPriorityManager("testdata/priorities.json")
	pm.Init(CardList{})

	port1 := PriorityPort{
		CardName: "test-card1",
		PortName: "test-port1",
		PortType: PortTypeBluetooth,
	}

	port2 := PriorityPort{
		CardName: "test-card2",
		PortName: "test-port2",
		PortType: PortTypeHdmi,
	}

	port3 := PriorityPort{
		CardName: "test-card3",
		PortName: "test-port3",
		PortType: PortTypeBluetooth,
	}

	assert.Equal(t, 0, pm.Output.AddPort(&port1))
	assert.Equal(t, 1, pm.Output.AddPort(&port2))
	assert.Equal(t, 0, pm.Output.AddPort(&port3))
}
