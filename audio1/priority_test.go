// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	assert.False(t, pr.IsInputTypeAfter(PortTypeBuiltin, PortTypeUsb))

	assert.True(t, pr.IsInputTypeAfter(PortTypeBluetooth, PortTypeHeadset))
	assert.True(t, pr.IsInputTypeAfter(PortTypeBluetooth, PortTypeBuiltin))
	assert.True(t, pr.IsInputTypeAfter(PortTypeBluetooth, PortTypeHdmi))
	assert.True(t, pr.IsInputTypeAfter(PortTypeHeadset, PortTypeBuiltin))
	assert.True(t, pr.IsInputTypeAfter(PortTypeBuiltin, PortTypeHdmi))
	assert.True(t, pr.IsInputTypeAfter(PortTypeUsb, PortTypeBuiltin))
}

func Test_IsOutputTypeAfter(t *testing.T) {
	pr := NewPriorities()
	pr.defaultInit(CardList{})
	assert.False(t, pr.IsOutputTypeAfter(PortTypeHeadset, PortTypeBluetooth))
	assert.False(t, pr.IsOutputTypeAfter(PortTypeBuiltin, PortTypeBluetooth))
	assert.False(t, pr.IsOutputTypeAfter(PortTypeHdmi, PortTypeBluetooth))
	assert.False(t, pr.IsOutputTypeAfter(PortTypeBuiltin, PortTypeHeadset))
	assert.False(t, pr.IsOutputTypeAfter(PortTypeHdmi, PortTypeBuiltin))
	assert.False(t, pr.IsOutputTypeAfter(PortTypeBuiltin, PortTypeUsb))

	assert.True(t, pr.IsOutputTypeAfter(PortTypeBluetooth, PortTypeHeadset))
	assert.True(t, pr.IsOutputTypeAfter(PortTypeBluetooth, PortTypeBuiltin))
	assert.True(t, pr.IsOutputTypeAfter(PortTypeBluetooth, PortTypeHdmi))
	assert.True(t, pr.IsOutputTypeAfter(PortTypeHeadset, PortTypeBuiltin))
	assert.True(t, pr.IsOutputTypeAfter(PortTypeBuiltin, PortTypeHdmi))
	assert.True(t, pr.IsOutputTypeAfter(PortTypeUsb, PortTypeBuiltin))
}

func Test_SetTheFirstPort(t *testing.T) {
	// 测试用例1: 正常情况 - 将端口移到第一个可用端口之前
	t.Run("move port to first available position", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth, PortTypeHeadset, PortTypeBuiltin}

		// 初始化端口列表
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
			{CardName: "card2", PortName: "bt2", PortType: PortTypeBluetooth},
		}
		pp.Ports[PortTypeHeadset] = []*PriorityPort{
			{CardName: "card3", PortName: "headset1", PortType: PortTypeHeadset},
		}
		pp.Ports[PortTypeBuiltin] = []*PriorityPort{
			{CardName: "card4", PortName: "builtin1", PortType: PortTypeBuiltin},
		}

		// card1 和 card3 可用
		available := portList{
			"card1": []string{"bt1"},
			"card3": []string{"headset1"},
		}

		// 将 card3 的端口设为第一优先级
		result := pp.SetTheFirstPort("card3", "headset1", available)

		assert.True(t, result)
		// card3 应该被移到 card1 之前
		firstPort, _ := pp.GetTheFirstPort(available)
		assert.Equal(t, "card3", firstPort.CardName)
		assert.Equal(t, "headset1", firstPort.PortName)
	})

	// 测试用例2: 端口不在可用列表中
	t.Run("port not in available list", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth}
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}

		available := portList{}

		result := pp.SetTheFirstPort("card1", "bt1", available)
		assert.False(t, result)
	})

	// 测试用例3: 端口不存在
	t.Run("port not found", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth}
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}

		available := portList{"card2": []string{"bt2"}}

		result := pp.SetTheFirstPort("card2", "bt2", available)
		assert.False(t, result)
	})

	// 测试用例4: 端口已经是第一个
	t.Run("port already first", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth, PortTypeHeadset}
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
			{CardName: "card2", PortName: "bt2", PortType: PortTypeBluetooth},
		}

		available := portList{
			"card1": []string{"bt1"},
			"card2": []string{"bt2"},
		}

		result := pp.SetTheFirstPort("card1", "bt1", available)

		assert.True(t, result)
		firstPort, _ := pp.GetTheFirstPort(available)
		assert.Equal(t, "card1", firstPort.CardName)
	})

	// 测试用例5: 跨类型移动端口
	t.Run("move port across types", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth, PortTypeHeadset, PortTypeBuiltin}

		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}
		pp.Ports[PortTypeHeadset] = []*PriorityPort{
			{CardName: "card2", PortName: "headset1", PortType: PortTypeHeadset},
		}
		pp.Ports[PortTypeBuiltin] = []*PriorityPort{
			{CardName: "card3", PortName: "builtin1", PortType: PortTypeBuiltin},
		}

		available := portList{
			"card1": []string{"bt1"},
			"card3": []string{"builtin1"},
		}

		// 将 builtin 端口移到第一位
		result := pp.SetTheFirstPort("card3", "builtin1", available)

		assert.True(t, result)
		firstPort, _ := pp.GetTheFirstPort(available)
		assert.Equal(t, "card3", firstPort.CardName)
		assert.Equal(t, "builtin1", firstPort.PortName)
	})

	// 测试用例6: 没有其他可用端口
	t.Run("no other available ports", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth, PortTypeHeadset}

		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}
		pp.Ports[PortTypeHeadset] = []*PriorityPort{
			{CardName: "card2", PortName: "headset1", PortType: PortTypeHeadset},
		}

		// 只有 card1 可用
		available := portList{
			"card1": []string{"bt1"},
		}

		// 尝试将 card1 设为第一（它已经是唯一可用的）
		result := pp.SetTheFirstPort("card1", "bt1", available)

		// 应该返回 false，因为没有其他可用端口
		assert.False(t, result)
		// 但端口应该被放回原位置
		assert.Equal(t, 1, len(pp.Ports[PortTypeBluetooth]))
		assert.Equal(t, "card1", pp.Ports[PortTypeBluetooth][0].CardName)
	})

	// 测试用例7: 同类型内移动端口
	t.Run("move port within same type", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth}

		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
			{CardName: "card2", PortName: "bt2", PortType: PortTypeBluetooth},
			{CardName: "card3", PortName: "bt3", PortType: PortTypeBluetooth},
		}

		available := portList{
			"card1": []string{"bt1"},
			"card2": []string{"bt2"},
			"card3": []string{"bt3"},
		}

		// 将 card3 移到第一位
		result := pp.SetTheFirstPort("card3", "bt3", available)

		assert.True(t, result)
		assert.Equal(t, 3, len(pp.Ports[PortTypeBluetooth]))
		assert.Equal(t, "card3", pp.Ports[PortTypeBluetooth][0].CardName)
		assert.Equal(t, "card1", pp.Ports[PortTypeBluetooth][1].CardName)
		assert.Equal(t, "card2", pp.Ports[PortTypeBluetooth][2].CardName)
	})

	// 测试用例8: 目标端口本身就是第一个可用端口
	t.Run("target port is already the first available", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth, PortTypeHeadset, PortTypeBuiltin}

		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}
		pp.Ports[PortTypeHeadset] = []*PriorityPort{
			{CardName: "card2", PortName: "headset1", PortType: PortTypeHeadset},
			{CardName: "card3", PortName: "headset2", PortType: PortTypeHeadset},
		}

		// 只有 headset 端口可用
		available := portList{
			"card2": []string{"headset1"},
			"card3": []string{"headset2"},
		}

		// 尝试将 card2 设为第一（它已经是第一个可用的）
		result := pp.SetTheFirstPort("card2", "headset1", available)

		// 应该成功，因为还有其他可用端口
		assert.True(t, result)
		// card2 应该仍然是第一个
		firstPort, _ := pp.GetTheFirstPort(available)
		assert.Equal(t, "card2", firstPort.CardName)
		assert.Equal(t, "headset1", firstPort.PortName)
	})
}
