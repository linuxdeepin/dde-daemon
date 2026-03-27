// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"testing"

	"github.com/linuxdeepin/go-lib/pulse"
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
	// 测试用例1: 正常情况 - 将端口移到第一个位置
	t.Run("move port to first position", func(t *testing.T) {
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

		// 将 card3 的端口设为第一优先级
		result := pp.SetTheFirstPort("card3", "headset1")

		assert.True(t, result)
		// card3 应该被移到第一位
		firstPort, _ := pp.GetTheFirstPort()
		assert.Equal(t, "card3", firstPort.CardName)
		assert.Equal(t, "headset1", firstPort.PortName)
	})

	// 测试用例2: 端口不存在
	t.Run("port not found", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth}
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}

		result := pp.SetTheFirstPort("card2", "bt2")
		assert.False(t, result)
	})

	// 测试用例3: 端口已经是第一个
	t.Run("port already first", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth, PortTypeHeadset}
		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
			{CardName: "card2", PortName: "bt2", PortType: PortTypeBluetooth},
		}

		result := pp.SetTheFirstPort("card1", "bt1")

		assert.True(t, result)
		firstPort, _ := pp.GetTheFirstPort()
		assert.Equal(t, "card1", firstPort.CardName)
	})

	// 测试用例4: 跨类型移动端口
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

		// 将 builtin 端口移到第一位
		result := pp.SetTheFirstPort("card3", "builtin1")

		assert.True(t, result)
		firstPort, _ := pp.GetTheFirstPort()
		assert.Equal(t, "card3", firstPort.CardName)
		assert.Equal(t, "builtin1", firstPort.PortName)
	})

	// 测试用例5: 端口已经在第一位
	t.Run("port already first", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth, PortTypeHeadset}

		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}
		pp.Ports[PortTypeHeadset] = []*PriorityPort{
			{CardName: "card2", PortName: "headset1", PortType: PortTypeHeadset},
		}

		// 尝试将 card1 设为第一（它已经在第一位）
		result := pp.SetTheFirstPort("card1", "bt1")

		// 应该返回 true，因为操作成功（即使端口已经在第一位）
		assert.True(t, result)
		// 端口应该保持在原位置
		assert.Equal(t, 1, len(pp.Ports[PortTypeBluetooth]))
		assert.Equal(t, "card1", pp.Ports[PortTypeBluetooth][0].CardName)
	})

	// 测试用例6: 同类型内移动端口
	t.Run("move port within same type", func(t *testing.T) {
		pp := NewPriorityPolicy()
		pp.Types = []int{PortTypeBluetooth}

		pp.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
			{CardName: "card2", PortName: "bt2", PortType: PortTypeBluetooth},
			{CardName: "card3", PortName: "bt3", PortType: PortTypeBluetooth},
		}

		// 将 card3 移到第一位
		result := pp.SetTheFirstPort("card3", "bt3")

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

		// 尝试将 card2 设为第一（它已经是第一个）
		result := pp.SetTheFirstPort("card2", "headset1")

		// 应该成功
		assert.True(t, result)
		// card2 应该仍然是第一个
		firstPort, _ := pp.GetTheFirstPort()
		assert.Equal(t, "card2", firstPort.CardName)
		assert.Equal(t, "headset1", firstPort.PortName)
	})
}

func Test_InsertPort_Logic(t *testing.T) {
	// 准备模拟数据辅助函数
	mockInsert := func(pp *PriorityPolicy, cardName string, portName string, priority uint32, allPorts []string) {
		card := &pulse.Card{Name: cardName}
		port := &pulse.CardPortInfo{
			PortInfo: pulse.PortInfo{
				Name:     portName,
				Priority: priority,
			},
		}
		allCardPorts := make(pulse.CardPortInfos, len(allPorts))
		for i, name := range allPorts {
			allCardPorts[i] = pulse.CardPortInfo{
				PortInfo: pulse.PortInfo{
					Name: name,
				},
			}
		}
		pp.InsertPort(card, port, allCardPorts)
	}

	t.Run("insert into empty queue", func(t *testing.T) {
		pp := NewPriorityPolicy()
		mockInsert(pp, "card1", "port1", 100, []string{"port1", "port2"})

		assert.Equal(t, 1, len(pp.Ports[PortTypeUnknown])) // DetectPortType 默认为 Unknown
		assert.Equal(t, "port1", pp.Ports[PortTypeUnknown][0].PortName)
	})

	t.Run("card clustering and internal ordering", func(t *testing.T) {
		pp := NewPriorityPolicy()
		// 1. 插入 A1 -> [A1]
		mockInsert(pp, "cardA", "A1", 100, []string{"A1", "A2"})
		// 2. 插入 B1 -> [B1, A1] (B1 是新插入的，置顶)
		mockInsert(pp, "cardB", "B1", 50, []string{"B1"})

		// 3. 插入 A2 -> 应该找到 A1 的位置并在 A 集群内按权重排序
		// A2 权重比 A1 低，应在 A1 后面
		mockInsert(pp, "cardA", "A2", 80, []string{"A1", "A2"})

		assert.Equal(t, 3, len(pp.Ports[PortTypeUnknown]))
		assert.Equal(t, "cardB", pp.Ports[PortTypeUnknown][0].CardName)
		assert.Equal(t, "A1", pp.Ports[PortTypeUnknown][1].PortName)
		assert.Equal(t, "A2", pp.Ports[PortTypeUnknown][2].PortName)
	})

	t.Run("cross-card priority sorting (higher) - Now Prepends", func(t *testing.T) {
		pp := NewPriorityPolicy()
		mockInsert(pp, "cardA", "A1", 100, []string{"A1"})

		// 无论 B1 优先级高还是低，作为新设备都应该置顶
		mockInsert(pp, "cardB", "B1", 200, []string{"B1"})

		assert.Equal(t, "cardB", pp.Ports[PortTypeUnknown][0].CardName)
		assert.Equal(t, "cardA", pp.Ports[PortTypeUnknown][1].CardName)
	})

	t.Run("cross-card priority sorting (lower) - Now Prepends", func(t *testing.T) {
		pp := NewPriorityPolicy()
		mockInsert(pp, "cardA", "A1", 100, []string{"A1"})

		// 即使 C1 优先级更低，作为新插入的设备依然置顶 (LIFO 策略)
		mockInsert(pp, "cardC", "C1", 50, []string{"C1"})

		assert.Equal(t, "cardC", pp.Ports[PortTypeUnknown][0].CardName)
		assert.Equal(t, "cardA", pp.Ports[PortTypeUnknown][1].CardName)
	})

	t.Run("maintain clustering after cluster-aware insert (Stable)", func(t *testing.T) {
		pp := NewPriorityPolicy()
		// 1. 插入 B1 -> [B1]
		mockInsert(pp, "cardB", "B1", 200, []string{"B1"})
		// 2. 插入 A1 -> [A1, B1] (A1 置顶)
		mockInsert(pp, "cardA", "A1", 100, []string{"A1", "A2"})

		// 3. 插入 A2 -> 应该跟随 A1 集群
		mockInsert(pp, "cardA", "A2", 50, []string{"A1", "A2"})

		assert.Equal(t, "cardA", pp.Ports[PortTypeUnknown][0].CardName)
		assert.Equal(t, "A1", pp.Ports[PortTypeUnknown][0].PortName)
		assert.Equal(t, "A2", pp.Ports[PortTypeUnknown][1].PortName)
		assert.Equal(t, "cardB", pp.Ports[PortTypeUnknown][2].CardName)
	})
}
func Test_cleanupMismatchedPorts(t *testing.T) {
	// 测试用例1: 清理端口类型与队列类型不匹配的端口
	t.Run("cleanup mismatched port types", func(t *testing.T) {
		pm := NewPriorityManager("")

		// 创建一个有问题的配置：蓝牙端口被放在了耳机队列中
		pm.Output.Types = []int{PortTypeBluetooth, PortTypeHeadset, PortTypeBuiltin}
		pm.Output.Ports = make(map[int]PriorityPortList)

		// 正确的配置
		pm.Output.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}

		// 错误的配置：蓝牙端口在耳机队列中
		pm.Output.Ports[PortTypeHeadset] = []*PriorityPort{
			{CardName: "card2", PortName: "headset1", PortType: PortTypeHeadset}, // 正确
			{CardName: "card3", PortName: "bt2", PortType: PortTypeBluetooth},    // 错误：蓝牙端口在耳机队列
		}

		// 执行清理
		pm.cleanupMismatchedPorts()

		// 验证结果
		assert.Equal(t, 1, len(pm.Output.Ports[PortTypeBluetooth]))
		assert.Equal(t, "bt1", pm.Output.Ports[PortTypeBluetooth][0].PortName)

		assert.Equal(t, 1, len(pm.Output.Ports[PortTypeHeadset]))
		assert.Equal(t, "headset1", pm.Output.Ports[PortTypeHeadset][0].PortName)
	})

	// 测试用例2: 清理无效的端口类型
	t.Run("cleanup invalid port types", func(t *testing.T) {
		pm := NewPriorityManager("")

		pm.Input.Types = []int{PortTypeBluetooth, PortTypeBuiltin}
		pm.Input.Ports = make(map[int]PriorityPortList)

		// 正确的配置
		pm.Input.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}

		// 无效的端口类型
		pm.Input.Ports[PortTypeBuiltin] = []*PriorityPort{
			{CardName: "card2", PortName: "builtin1", PortType: PortTypeBuiltin}, // 正确
			{CardName: "card3", PortName: "invalid1", PortType: 999},             // 无效类型
			{CardName: "card4", PortName: "invalid2", PortType: -1},              // 无效类型
		}

		// 执行清理
		pm.cleanupMismatchedPorts()

		// 验证结果
		assert.Equal(t, 1, len(pm.Input.Ports[PortTypeBluetooth]))
		assert.Equal(t, 1, len(pm.Input.Ports[PortTypeBuiltin]))
		assert.Equal(t, "builtin1", pm.Input.Ports[PortTypeBuiltin][0].PortName)
	})

	// 测试用例3: 清理无效的队列类型
	t.Run("cleanup invalid queue types", func(t *testing.T) {
		pm := NewPriorityManager("")

		pm.Output.Types = []int{PortTypeBluetooth}
		pm.Output.Ports = make(map[int]PriorityPortList)

		// 正确的队列
		pm.Output.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}

		// 无效的队列类型
		pm.Output.Ports[999] = []*PriorityPort{
			{CardName: "card2", PortName: "invalid1", PortType: 999},
		}
		pm.Output.Ports[-1] = []*PriorityPort{
			{CardName: "card3", PortName: "invalid2", PortType: -1},
		}

		// 执行清理
		pm.cleanupMismatchedPorts()

		// 验证结果：无效队列应该被删除
		assert.Equal(t, 1, len(pm.Output.Ports))
		_, exists999 := pm.Output.Ports[999]
		assert.False(t, exists999)
		_, existsNeg1 := pm.Output.Ports[-1]
		assert.False(t, existsNeg1)

		assert.Equal(t, 1, len(pm.Output.Ports[PortTypeBluetooth]))
	})

	// 测试用例4: 清理 nil 端口
	t.Run("cleanup nil ports", func(t *testing.T) {
		pm := NewPriorityManager("")

		pm.Output.Types = []int{PortTypeBluetooth}
		pm.Output.Ports = make(map[int]PriorityPortList)

		// 包含 nil 端口的队列
		pm.Output.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
			nil, // nil 端口
			{CardName: "card2", PortName: "bt2", PortType: PortTypeBluetooth},
		}

		// 执行清理
		pm.cleanupMismatchedPorts()

		// 验证结果：nil 端口应该被删除
		assert.Equal(t, 2, len(pm.Output.Ports[PortTypeBluetooth]))
		assert.Equal(t, "bt1", pm.Output.Ports[PortTypeBluetooth][0].PortName)
		assert.Equal(t, "bt2", pm.Output.Ports[PortTypeBluetooth][1].PortName)
	})

	// 测试用例5: 清理后空队列被删除
	t.Run("remove empty queues after cleanup", func(t *testing.T) {
		pm := NewPriorityManager("")

		pm.Input.Types = []int{PortTypeBluetooth, PortTypeHeadset}
		pm.Input.Ports = make(map[int]PriorityPortList)

		// 正常队列
		pm.Input.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
		}

		// 全是错误端口的队列
		pm.Input.Ports[PortTypeHeadset] = []*PriorityPort{
			{CardName: "card2", PortName: "bt2", PortType: PortTypeBluetooth}, // 类型不匹配
			{CardName: "card3", PortName: "invalid", PortType: 999},           // 无效类型
		}

		// 执行清理
		pm.cleanupMismatchedPorts()

		// 验证结果：空队列应该被删除
		assert.Equal(t, 1, len(pm.Input.Ports))
		_, existsHeadset := pm.Input.Ports[PortTypeHeadset]
		assert.False(t, existsHeadset)

		assert.Equal(t, 1, len(pm.Input.Ports[PortTypeBluetooth]))
	})

	// 测试用例6: 正常配置不受影响
	t.Run("normal configuration unchanged", func(t *testing.T) {
		pm := NewPriorityManager("")

		pm.Output.Types = []int{PortTypeBluetooth, PortTypeHeadset, PortTypeBuiltin}
		pm.Output.Ports = make(map[int]PriorityPortList)

		// 全部正确的配置
		pm.Output.Ports[PortTypeBluetooth] = []*PriorityPort{
			{CardName: "card1", PortName: "bt1", PortType: PortTypeBluetooth},
			{CardName: "card2", PortName: "bt2", PortType: PortTypeBluetooth},
		}
		pm.Output.Ports[PortTypeHeadset] = []*PriorityPort{
			{CardName: "card3", PortName: "headset1", PortType: PortTypeHeadset},
		}
		pm.Output.Ports[PortTypeBuiltin] = []*PriorityPort{
			{CardName: "card4", PortName: "builtin1", PortType: PortTypeBuiltin},
		}

		// 执行清理
		pm.cleanupMismatchedPorts()

		// 验证结果：所有配置应该保持不变
		assert.Equal(t, 3, len(pm.Output.Ports))
		assert.Equal(t, 2, len(pm.Output.Ports[PortTypeBluetooth]))
		assert.Equal(t, 1, len(pm.Output.Ports[PortTypeHeadset]))
		assert.Equal(t, 1, len(pm.Output.Ports[PortTypeBuiltin]))
	})
}
