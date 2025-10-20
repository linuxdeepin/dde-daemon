// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package audio

import (
	"testing"

	"github.com/linuxdeepin/go-lib/pulse"
	"github.com/stretchr/testify/assert"
)

func Test_getCardName(t *testing.T) {
	type args struct {
		card *pulse.Card
	}
	tests := []struct {
		name     string
		args     args
		wantName string
	}{
		{
			name: "getCardName",
			args: args{
				card: &pulse.Card{
					Name: "abcd",
					PropList: map[string]string{
						"alsa.card_name":     "",
						"device.api":         "fdf",
						"device.description": "",
					},
				},
			},
			wantName: "abcd",
		},
		{
			name: "getCardName with alsa card name",
			args: args{
				card: &pulse.Card{
					Name: "abcd",
					PropList: map[string]string{
						"alsa.card_name":     "cdcd",
						"device.api":         "fdf",
						"device.description": "",
					},
				},
			},
			wantName: "cdcd",
		},
		{
			name: "getCardName bluez",
			args: args{
				card: &pulse.Card{
					Name: "abcd",
					PropList: map[string]string{
						"alsa.card_name":     "",
						"device.api":         "bluez",
						"device.description": "xxxxxx",
					},
				},
			},
			wantName: "xxxxxx",
		},
		{
			name: "getCardName bluez no description",
			args: args{
				card: &pulse.Card{
					Name: "abcd",
					PropList: map[string]string{
						"alsa.card_name":     "vvvvvv",
						"device.api":         "bluez",
						"device.description": "",
					},
				},
			},
			wantName: "vvvvvv",
		},
		{
			name: "getCardName bluez no description and alsa card name",
			args: args{
				card: &pulse.Card{
					Name: "abcd",
					PropList: map[string]string{
						"alsa.card_name":     "",
						"device.api":         "bluez",
						"device.description": "",
					},
				},
			},
			wantName: "abcd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName := getCardName(tt.args.card)
			assert.Equal(t, tt.wantName, gotName)
		})
	}
}

func Test_sortPortsByPriority(t *testing.T) {
	// 测试用例1: 按 Priority 值从小到大排序
	t.Run("sort ports by priority descending", func(t *testing.T) {
		pulseCard := &pulse.Card{
			Name: "test-card",
		}

		card := &Card{
			Ports: pulse.CardPortInfos{
				{
					PortInfo:  pulse.PortInfo{Name: "port1", Priority: 100},
					Direction: pulse.DirectionSink,
				},
				{
					PortInfo:  pulse.PortInfo{Name: "port2", Priority: 500},
					Direction: pulse.DirectionSink,
				},
				{
					PortInfo:  pulse.PortInfo{Name: "port3", Priority: 300},
					Direction: pulse.DirectionSink,
				},
			},
		}

		card.sortPortsByPriority(pulseCard)

		// 验证排序结果：100 < 300 < 500
		assert.Equal(t, "port1", card.Ports[0].Name)
		assert.Equal(t, uint32(100), card.Ports[0].Priority)
		assert.Equal(t, "port3", card.Ports[1].Name)
		assert.Equal(t, uint32(300), card.Ports[1].Priority)
		assert.Equal(t, "port2", card.Ports[2].Name)
		assert.Equal(t, uint32(500), card.Ports[2].Priority)
	})

	// 测试用例2: 相同 Priority 值保持稳定排序
	t.Run("stable sort for equal priorities", func(t *testing.T) {
		pulseCard := &pulse.Card{
			Name: "test-card",
		}

		card := &Card{
			Ports: pulse.CardPortInfos{
				{
					PortInfo:  pulse.PortInfo{Name: "port1", Priority: 100},
					Direction: pulse.DirectionSink,
				},
				{
					PortInfo:  pulse.PortInfo{Name: "port2", Priority: 100},
					Direction: pulse.DirectionSink,
				},
				{
					PortInfo:  pulse.PortInfo{Name: "port3", Priority: 100},
					Direction: pulse.DirectionSink,
				},
			},
		}

		card.sortPortsByPriority(pulseCard)

		// 相同优先级保持原顺序
		assert.Equal(t, "port1", card.Ports[0].Name)
		assert.Equal(t, "port2", card.Ports[1].Name)
		assert.Equal(t, "port3", card.Ports[2].Name)
	})

	// 测试用例3: 混合输入输出端口
	t.Run("sort mixed input and output ports", func(t *testing.T) {
		pulseCard := &pulse.Card{
			Name: "test-card",
		}

		card := &Card{
			Ports: pulse.CardPortInfos{
				{
					PortInfo:  pulse.PortInfo{Name: "output1", Priority: 200},
					Direction: pulse.DirectionSink,
				},
				{
					PortInfo:  pulse.PortInfo{Name: "input1", Priority: 300},
					Direction: pulse.DirectionSource,
				},
				{
					PortInfo:  pulse.PortInfo{Name: "output2", Priority: 400},
					Direction: pulse.DirectionSink,
				},
			},
		}

		card.sortPortsByPriority(pulseCard)

		// 按 Priority 排序，不区分方向：200 < 300 < 400
		assert.Equal(t, "output1", card.Ports[0].Name)
		assert.Equal(t, uint32(200), card.Ports[0].Priority)
		assert.Equal(t, "input1", card.Ports[1].Name)
		assert.Equal(t, uint32(300), card.Ports[1].Priority)
		assert.Equal(t, "output2", card.Ports[2].Name)
		assert.Equal(t, uint32(400), card.Ports[2].Priority)
	})

	// 测试用例4: 空端口列表
	t.Run("empty ports list", func(t *testing.T) {
		pulseCard := &pulse.Card{
			Name: "test-card",
		}

		card := &Card{
			Ports: pulse.CardPortInfos{},
		}

		// 不应该崩溃
		card.sortPortsByPriority(pulseCard)
		assert.Equal(t, 0, len(card.Ports))
	})
}
