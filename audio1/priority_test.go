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

func Test_insertByPriority(t *testing.T) {
	// 测试用例1: 插入到空队列
	t.Run("insert to empty queue", func(t *testing.T) {
		pp := NewPriorityPolicy()
		newPort := &PriorityPort{
			CardName: "card1",
			PortName: "bt1",
			PortType: PortTypeBluetooth,
			Priority: 20000,
		}

		pp.insertByPriority(newPort, PortTypeBluetooth)

		assert.Equal(t, 1, len(pp.Ports[PortTypeBluetooth]))
		assert.Equal(t, "card1", pp.Ports[PortTypeBluetooth][0].CardName)
		assert.Equal(t, uint32(20000), pp.Ports[PortTypeBluetooth][0].Priority)
	})

	// 测试用例2: 新端口权重最大，插入到队头
	t.Run("insert port with highest priority to head", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth, Priority: 20000},
			{CardName: "card1", PortName: "bt2", PortType: PortTypeBluetooth, Priority: 10000},
		}

		newPort := &PriorityPort{
			CardName: "card1",
			PortName: "bt3",
			PortType: PortTypeBluetooth,
			Priority: 25000, // 最高权重
		}

		pp.insertByPriority(newPort, PortTypeBluetooth)

		assert.Equal(t, 3, len(pp.Ports[PortTypeBluetooth]))
		assert.Equal(t, "bt3", pp.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, uint32(25000), pp.Ports[PortTypeBluetooth][0].Priority)
		assert.Equal(t, "bt1", pp.Ports[PortTypeBluetooth][1].PortName)
		assert.Equal(t, "bt2", pp.Ports[PortTypeBluetooth][2].PortName)
	})

	// 测试用例3: 新端口权重最小，插入到最后一个同声卡端口之后
	t.Run("insert port with lowest priority after last same card port", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth, Priority: 20000},
			{CardName: "card1", PortName: "bt2", PortType: PortTypeBluetooth, Priority: 15000},
		}

		newPort := &PriorityPort{
			CardName: "card1",
			PortName: "bt3",
			PortType: PortTypeBluetooth,
			Priority: 10000, // 最低权重
		}

		pp.insertByPriority(newPort, PortTypeBluetooth)

		assert.Equal(t, 3, len(pp.Ports[PortTypeBluetooth]))
		assert.Equal(t, "bt1", pp.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, "bt2", pp.Ports[PortTypeBluetooth][1].PortName)
		assert.Equal(t, "bt3", pp.Ports[PortTypeBluetooth][2].PortName)
		assert.Equal(t, uint32(10000), pp.Ports[PortTypeBluetooth][2].Priority)
	})

	// 测试用例4: 新端口权重居中，插入到中间位置
	t.Run("insert port with middle priority to middle position", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth, Priority: 20000},
			{CardName: "card1", PortName: "bt2", PortType: PortTypeBluetooth, Priority: 10000},
		}

		newPort := &PriorityPort{
			CardName: "card1",
			PortName: "bt3",
			PortType: PortTypeBluetooth,
			Priority: 15000, // 中间权重
		}

		pp.insertByPriority(newPort, PortTypeBluetooth)

		assert.Equal(t, 3, len(pp.Ports[PortTypeBluetooth]))
		assert.Equal(t, "bt1", pp.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, uint32(20000), pp.Ports[PortTypeBluetooth][0].Priority)
		assert.Equal(t, "bt3", pp.Ports[PortTypeBluetooth][1].PortName)
		assert.Equal(t, uint32(15000), pp.Ports[PortTypeBluetooth][1].Priority)
		assert.Equal(t, "bt2", pp.Ports[PortTypeBluetooth][2].PortName)
		assert.Equal(t, uint32(10000), pp.Ports[PortTypeBluetooth][2].Priority)
	})

	// 测试用例5: 权重相等，插入到队头（不能等于）
	t.Run("insert port with equal priority to head", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth, Priority: 20000},
		}

		newPort := &PriorityPort{
			CardName: "card1",
			PortName: "bt2",
			PortType: PortTypeBluetooth,
			Priority: 20000, // 相等权重
		}

		pp.insertByPriority(newPort, PortTypeBluetooth)

		assert.Equal(t, 2, len(pp.Ports[PortTypeBluetooth]))
		// 因为权重相等（不大于），所以插入到队头
		assert.Equal(t, "bt2", pp.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, "bt1", pp.Ports[PortTypeBluetooth][1].PortName)
	})

	// 测试用例6: 不同声卡，插入到队头
	t.Run("insert port from different card to head", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth, Priority: 20000},
			{CardName: "card1", PortName: "bt2", PortType: PortTypeBluetooth, Priority: 10000},
		}

		newPort := &PriorityPort{
			CardName: "card2", // 不同声卡
			PortName: "bt3",
			PortType: PortTypeBluetooth,
			Priority: 15000,
		}

		pp.insertByPriority(newPort, PortTypeBluetooth)

		assert.Equal(t, 3, len(pp.Ports[PortTypeBluetooth]))
		// 不同声卡，插入到队头
		assert.Equal(t, "card2", pp.Ports[PortTypeBluetooth][0].CardName)
		assert.Equal(t, "bt3", pp.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, "card1", pp.Ports[PortTypeBluetooth][1].CardName)
		assert.Equal(t, "card1", pp.Ports[PortTypeBluetooth][2].CardName)
	})

	// 测试用例7: 多个同声卡端口，插入到正确位置
	t.Run("insert port among multiple same card ports", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth, Priority: 30000},
			{CardName: "card1", PortName: "bt2", PortType: PortTypeBluetooth, Priority: 20000},
			{CardName: "card1", PortName: "bt3", PortType: PortTypeBluetooth, Priority: 10000},
		}

		newPort := &PriorityPort{
			CardName: "card1",
			PortName: "bt4",
			PortType: PortTypeBluetooth,
			Priority: 15000, // 介于 20000 和 10000 之间
		}

		pp.insertByPriority(newPort, PortTypeBluetooth)

		assert.Equal(t, 4, len(pp.Ports[PortTypeBluetooth]))
		assert.Equal(t, "bt1", pp.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, uint32(30000), pp.Ports[PortTypeBluetooth][0].Priority)
		assert.Equal(t, "bt2", pp.Ports[PortTypeBluetooth][1].PortName)
		assert.Equal(t, uint32(20000), pp.Ports[PortTypeBluetooth][1].Priority)
		assert.Equal(t, "bt4", pp.Ports[PortTypeBluetooth][2].PortName)
		assert.Equal(t, uint32(15000), pp.Ports[PortTypeBluetooth][2].Priority)
		assert.Equal(t, "bt3", pp.Ports[PortTypeBluetooth][3].PortName)
		assert.Equal(t, uint32(10000), pp.Ports[PortTypeBluetooth][3].Priority)
	})

	// 测试用例8: 混合不同声卡，只考虑同声卡端口
	t.Run("insert port with mixed cards in queue", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth, Priority: 25000},
			{CardName: "card2", PortName: "bt2", PortType: PortTypeBluetooth, Priority: 22000},
			{CardName: "card1", PortName: "bt3", PortType: PortTypeBluetooth, Priority: 15000},
			{CardName: "card2", PortName: "bt4", PortType: PortTypeBluetooth, Priority: 12000},
		}

		newPort := &PriorityPort{
			CardName: "card1",
			PortName: "bt5",
			PortType: PortTypeBluetooth,
			Priority: 20000, // 介于 card1 的 25000 和 15000 之间
		}

		pp.insertByPriority(newPort, PortTypeBluetooth)

		assert.Equal(t, 5, len(pp.Ports[PortTypeBluetooth]))
		// 应该插入到 card1 的 bt1 (25000) 之后
		assert.Equal(t, "bt1", pp.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, "bt5", pp.Ports[PortTypeBluetooth][1].PortName)
		assert.Equal(t, uint32(20000), pp.Ports[PortTypeBluetooth][1].Priority)
	})

	// 测试用例9: 真实场景 - A2DP 和 HSP 端口
	t.Run("real scenario - A2DP and HSP ports", func(t *testing.T) {
		pp := NewPriorityPolicy()

		// 先插入 HSP (低权重)
		hspPort := &PriorityPort{
			CardName: "bluez_card.XX_XX_XX_XX_XX_XX",
			PortName: "headset-head-unit",
			PortType: PortTypeBluetooth,
			Priority: 10000,
		}
		pp.insertByPriority(hspPort, PortTypeBluetooth)

		// 再插入 A2DP (高权重)
		a2dpPort := &PriorityPort{
			CardName: "bluez_card.XX_XX_XX_XX_XX_XX",
			PortName: "headset-output",
			PortType: PortTypeBluetooth,
			Priority: 20000,
		}
		pp.insertByPriority(a2dpPort, PortTypeBluetooth)

		// A2DP 应该在 HSP 之前
		assert.Equal(t, 2, len(pp.Ports[PortTypeBluetooth]))
		assert.Equal(t, "headset-output", pp.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, uint32(20000), pp.Ports[PortTypeBluetooth][0].Priority)
		assert.Equal(t, "headset-head-unit", pp.Ports[PortTypeBluetooth][1].PortName)
		assert.Equal(t, uint32(10000), pp.Ports[PortTypeBluetooth][1].Priority)
	})

	// 测试用例10: 边界情况 - 插入到末尾
	t.Run("boundary case - insert at end", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth, Priority: 30000},
			{CardName: "card1", PortName: "bt2", PortType: PortTypeBluetooth, Priority: 20000},
		}

		newPort := &PriorityPort{
			CardName: "card1",
			PortName: "bt3",
			PortType: PortTypeBluetooth,
			Priority: 5000, // 最低权重
		}

		pp.insertByPriority(newPort, PortTypeBluetooth)

		assert.Equal(t, 3, len(pp.Ports[PortTypeBluetooth]))
		// 应该插入到末尾
		assert.Equal(t, "bt3", pp.Ports[PortTypeBluetooth][2].PortName)
		assert.Equal(t, uint32(5000), pp.Ports[PortTypeBluetooth][2].Priority)
	})
}

func Test_insertByPriority_AllEqualPriority(t *testing.T) {
	// 测试所有端口权重相同的情况
	t.Run("all ports have equal priority", func(t *testing.T) {
		pp := NewPriorityPolicy()

		// 按顺序插入 4 个相同权重的端口
		port1 := &PriorityPort{
			CardName: "card1",
			PortName: "port1",
			PortType: PortTypeBluetooth,
			Priority: 10000,
		}
		pp.insertByPriority(port1, PortTypeBluetooth)

		port2 := &PriorityPort{
			CardName: "card1",
			PortName: "port2",
			PortType: PortTypeBluetooth,
			Priority: 10000,
		}
		pp.insertByPriority(port2, PortTypeBluetooth)

		port3 := &PriorityPort{
			CardName: "card1",
			PortName: "port3",
			PortType: PortTypeBluetooth,
			Priority: 10000,
		}
		pp.insertByPriority(port3, PortTypeBluetooth)

		port4 := &PriorityPort{
			CardName: "card1",
			PortName: "port4",
			PortType: PortTypeBluetooth,
			Priority: 10000,
		}
		pp.insertByPriority(port4, PortTypeBluetooth)

		// 验证结果
		assert.Equal(t, 4, len(pp.Ports[PortTypeBluetooth]))

		// 当前行为：后插入的在前面（LIFO）
		// 顺序应该是: port4, port3, port2, port1
		assert.Equal(t, "port4", pp.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, "port3", pp.Ports[PortTypeBluetooth][1].PortName)
		assert.Equal(t, "port2", pp.Ports[PortTypeBluetooth][2].PortName)
		assert.Equal(t, "port1", pp.Ports[PortTypeBluetooth][3].PortName)

		// 所有端口权重都相同
		for i := 0; i < 4; i++ {
			assert.Equal(t, uint32(10000), pp.Ports[PortTypeBluetooth][i].Priority)
		}
	})

	// 测试混合场景：部分相同权重
	t.Run("mixed equal and different priorities", func(t *testing.T) {
		pp := NewPriorityPolicy()

		// 插入高权重端口
		pp.insertByPriority(&PriorityPort{
			CardName: "card1",
			PortName: "high1",
			PortType: PortTypeBluetooth,
			Priority: 20000,
		}, PortTypeBluetooth)

		// 插入相同低权重端口
		pp.insertByPriority(&PriorityPort{
			CardName: "card1",
			PortName: "low1",
			PortType: PortTypeBluetooth,
			Priority: 10000,
		}, PortTypeBluetooth)

		pp.insertByPriority(&PriorityPort{
			CardName: "card1",
			PortName: "low2",
			PortType: PortTypeBluetooth,
			Priority: 10000,
		}, PortTypeBluetooth)

		pp.insertByPriority(&PriorityPort{
			CardName: "card1",
			PortName: "low3",
			PortType: PortTypeBluetooth,
			Priority: 10000,
		}, PortTypeBluetooth)

		// 验证顺序
		assert.Equal(t, 4, len(pp.Ports[PortTypeBluetooth]))
		assert.Equal(t, "high1", pp.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, uint32(20000), pp.Ports[PortTypeBluetooth][0].Priority)

		// 相同权重的端口按 LIFO 顺序
		assert.Equal(t, "low3", pp.Ports[PortTypeBluetooth][1].PortName)
		assert.Equal(t, "low2", pp.Ports[PortTypeBluetooth][2].PortName)
		assert.Equal(t, "low1", pp.Ports[PortTypeBluetooth][3].PortName)
	})
}
