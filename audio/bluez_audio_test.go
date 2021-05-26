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

	"pkg.deepin.io/lib/pulse"

	"github.com/stretchr/testify/assert"
)

func Test_isBluezAudio(t *testing.T) {
	assert.True(t, isBluezAudio("bluez.abcd.1234"))
	assert.True(t, isBluezAudio("hbc.bluez.1234"))
	assert.True(t, isBluezAudio("BLUEZ.abcd.1234"))
	assert.False(t, isBluezAudio("bluz.abcd.1234"))
	assert.False(t, isBluezAudio("hbc.bluz.1234"))
	assert.False(t, isBluezAudio("BLUZ.abcd.1234"))
}

func Test_createBluezVirtualCardPorts(t *testing.T) {
	cardName := "bluez.test.a2dp"
	portName := "bluez.test.port0"
	desc := "headset"
	var cardPorts pulse.CardPortInfos
	cardPorts = append(cardPorts, pulse.CardPortInfo{
		PortInfo: pulse.PortInfo{
			Name:        portName,
			Description: desc,
			Priority:    0,
			Available:   pulse.AvailableTypeUnknow,
		},
		Direction: pulse.DirectionSink,
		Profiles:  pulse.ProfileInfos2{},
	})
	portinfos := createBluezVirtualCardPorts(cardName, cardPorts)
	assert.Equal(t, len(portinfos), 0)

	cardPorts[0].Profiles = append(cardPorts[0].Profiles, pulse.ProfileInfo2{
		Name:        "a2dp_sink",
		Description: "A2DP",
		Priority:    0,
		NSinks:      1,
		NSources:    0,
		Available:   pulse.AvailableTypeUnknow,
	})
	portinfos = createBluezVirtualCardPorts(cardName, cardPorts)
	assert.Equal(t, len(portinfos), 1)

	cardPorts[0].Profiles = append(cardPorts[0].Profiles, pulse.ProfileInfo2{
		Name:        "headset_head_unit",
		Description: "Headset",
		Priority:    0,
		NSinks:      1,
		NSources:    0,
		Available:   pulse.AvailableTypeUnknow,
	})
	portinfos = createBluezVirtualCardPorts(cardName, cardPorts)
	assert.Equal(t, len(portinfos), 2)
	assert.Equal(t, portinfos[0].Name, portName+"(headset_head_unit)")
	assert.Equal(t, portinfos[1].Name, portName+"(a2dp_sink)")
}

func Test_createBluezVirtualSinkPorts(t *testing.T) {
	name := "bluez.abcd.1234"
	var ports []Port
	ports = append(ports, Port{
		Name:        name,
		Description: "headsert",
		Available:   byte(pulse.AvailableTypeUnknow),
	})

	ret := createBluezVirtualSinkPorts(ports)
	assert.Equal(t, len(ret), 2)
	assert.Equal(t, ret[0].Name, name+"(headset_head_unit)")
	assert.Equal(t, ret[1].Name, name+"(a2dp_sink)")
}

func Test_createBluezVirtualSourcePorts(t *testing.T) {
	name := "bluez.abcd.1234"
	var ports []Port
	ports = append(ports, Port{
		Name:        name,
		Description: "headset",
		Available:   byte(pulse.AvailableTypeUnknow),
	})

	ret := createBluezVirtualSourcePorts(ports)
	assert.Equal(t, len(ret), 1)
	assert.Equal(t, ret[0].Name, name+"(headset_head_unit)")
}
