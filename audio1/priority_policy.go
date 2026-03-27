// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"strings"

	"github.com/linuxdeepin/go-lib/pulse"
)

const (
	PortTypeBluetooth    int = iota // 蓝牙音频
	PortTypeHeadset                 // 3.5mm 耳麦
	PortTypeUsb                     // USB
	PortTypeBuiltin                 // 内置扬声器和话筒
	PortTypeHdmi                    // HDMI
	PortTypeLineIO                  // 线缆输入输出
	PortTypeMultiChannel            // 多声道
	PortTypeUnknown                 // 其他类型

	PortTypeCount        // 有效的端口类型个数
	PortTypeInvalid = -1 // 表示无效的类型
)

// 判断一组字符串中，是否存在其中一个字符串A，使得substr是A的子字符串，不区分大小写
func hasKeyword(stringList []string, substr string) bool {
	for _, str := range stringList {
		if strings.Contains(strings.ToLower(str), strings.ToLower(substr)) {
			return true
		}
	}

	return false
}

// 检测端口的类型
func DetectPortType(card *pulse.Card, port *pulse.CardPortInfo) int {
	// 需要判断的字段列表
	stringList := []string{
		card.Name,
		port.Name,
		card.PropList["alsa.card_name"],
		card.PropList["alsa.long_card_name"],
	}

	if hasKeyword(stringList, "multichannel") {
		return PortTypeMultiChannel
	}

	if hasKeyword(stringList, "bluez") ||
		hasKeyword(stringList, "bluetooth") {
		return PortTypeBluetooth
	}

	if hasKeyword(stringList, "linein") ||
		hasKeyword(stringList, "lineout") {
		return PortTypeLineIO
	}

	if hasKeyword(stringList, "rear-mic") ||
		hasKeyword(stringList, "front-mic") ||
		hasKeyword(stringList, "headphone") ||
		hasKeyword(stringList, "headset") {
		return PortTypeHeadset
	}

	if hasKeyword(stringList, "usb") {
		return PortTypeUsb
	}

	if hasKeyword(stringList, "hdmi") {
		return PortTypeHdmi
	}

	if hasKeyword(stringList, "speaker") ||
		hasKeyword(stringList, "input-mic") {
		return PortTypeBuiltin
	}

	return PortTypeUnknown
}

type Position struct {
	tp    int // 所在队列
	index int // 在队列中的索引
}

// 优先级中使用的端口（注意：用于表示端口的结构体有好几个，不要弄混）
type PriorityPort struct {
	CardName string
	PortName string
	PortType int    // 部分声卡需要在Property里判断类型，只有CardName和PortName不足以用来判断PortType，因此添加此项
	Priority uint32 // 端口权重，用于排序
}

// 端口实例优先级列表
type PriorityPortList []*PriorityPort

// 判断端口实例优先级列表中是否包含某个值，判断时只考虑CardName和PortName，忽略PortType
func (portList *PriorityPortList) hasElement(port *PriorityPort) bool {
	for _, p := range *portList {
		if p.CardName == port.CardName && p.PortName == port.PortName {
			return true
		}
	}

	return false
}

// 端口类型优先级列表
type PriorityTypeList []int

// 判断端口类型优先级列表中是否包含某个值
func (typeList *PriorityTypeList) hasElement(value int) bool {
	for _, v := range *typeList {
		if value == v {
			return true
		}
	}

	return false
}

// 管理一组实例和类型的优先级
type PriorityPolicy struct {
	Ports map[int]PriorityPortList
	Types PriorityTypeList
}

// 新建一个PriorityPolicy
func NewPriorityPolicy() *PriorityPolicy {
	return &PriorityPolicy{
		Ports: make(map[int]PriorityPortList, 0),
		Types: make(PriorityTypeList, 0),
	}
}

// 读取配置文件获得的类型优先级中类型的数量少于PortTypeCount时
// 将缺少的类型补充完整
// 通常发生在增加了新的端口类型的时候
// 也可以用于初始化空的优先级列表
func (pp *PriorityPolicy) completeTypes() {
	for i := 0; i < PortTypeCount; i++ {
		if !pp.Types.hasElement(i) {
			pp.Types = append(pp.Types, i)
			logger.Debugf("append type %d", i)
		}
	}
}

// 获取端口数量
func (pp *PriorityPolicy) CountPort() int {
	return len(pp.Ports)
}

// 比较端口类型的优先级
func (pp *PriorityPolicy) GetPreferType(type1 int, type2 int) int {
	for _, t := range pp.Types {
		if t == type1 || t == type2 {
			return t
		}
	}

	return -1
}

func (pp *PriorityPolicy) FindPort(cardName string, portName string) *Position {
	for tp, pList := range pp.Ports {
		for i, p := range pList {
			if p.CardName == cardName && p.PortName == portName {
				return &Position{tp: tp, index: i}
			}
		}
	}
	return &Position{tp: PortTypeInvalid, index: -1}
}

func (pp *PriorityPolicy) InsertPort(card *pulse.Card, port *pulse.CardPortInfo, allCardPorts pulse.CardPortInfos) {
	if pos := pp.FindPort(card.Name, port.Name); pos.tp != PortTypeInvalid {
		return
	}

	tp := DetectPortType(card, port)
	newPort := &PriorityPort{
		CardName: card.Name, PortName: port.Name,
		PortType: tp, Priority: port.Priority,
	}
	queue := pp.Ports[tp]

	// 1. 构建权重 Map，显式处理“未记录端口”的权重
	orderMap := make(map[string]int, len(allCardPorts))
	for i, p := range allCardPorts {
		orderMap[p.Name] = i
	}
	myWeight, ok := orderMap[port.Name]
	if !ok {
		// 防御性处理：如果端口名不在声卡列表中，默认排在该卡集群的末尾
		myWeight = len(allCardPorts)
	}

	// 2. 寻找插入点
	insertIdx := 0 // 默认为新卡置顶逻辑
	foundSameCard := false
	for i, p := range queue {
		if p.CardName == card.Name {
			if !foundSameCard {
				foundSameCard = true
				insertIdx = i // 找到集群后，默认插在集群首位
			}
			// 在声卡集群内部，依据 orderMap 权重进行排序
			if otherWeight, ok := orderMap[p.PortName]; ok && myWeight > otherWeight {
				insertIdx = i + 1
			}
		}
	}

	// 3. 执行切片插入
	if insertIdx >= len(queue) {
		pp.Ports[tp] = append(queue, newPort)
	} else {
		pp.Ports[tp] = append(queue[:insertIdx], append([]*PriorityPort{newPort}, queue[insertIdx:]...)...)
	}
}

func (pp *PriorityPolicy) GetTheFirstPort() (*PriorityPort, *Position) {
	for _, tp := range pp.Types {
		for i, port := range pp.Ports[tp] {
			return port, &Position{tp: tp, index: i}
		}
	}
	return nil, &Position{tp: PortTypeInvalid, index: -1}
}

func (pp *PriorityPolicy) GetNextPort(pos *Position) (*PriorityPort, *Position) {
	if pos.tp == PortTypeInvalid {
		return nil, &Position{tp: PortTypeInvalid, index: -1}
	}

	// 找到当前类型在 Types 队列中的位置
	currentTypeIndex := -1
	for i, tp := range pp.Types {
		if tp == pos.tp {
			currentTypeIndex = i
			break
		}
	}

	if currentTypeIndex == -1 {
		return nil, &Position{tp: PortTypeInvalid, index: -1}
	}

	// 从当前类型开始遍历
	for i := currentTypeIndex; i < len(pp.Types); i++ {
		tp := pp.Types[i]
		startIndex := 0
		if tp == pos.tp {
			// 如果是当前类型，从下一个索引开始
			startIndex = pos.index + 1
		}
		for j := startIndex; j < len(pp.Ports[tp]); j++ {
			port := pp.Ports[tp][j]
			return port, &Position{tp: tp, index: j}
		}
	}
	return nil, &Position{tp: PortTypeInvalid, index: -1}
}

func (pp *PriorityPolicy) SetTheFirstPort(cardName string, portName string) bool {
	// 找到目标端口
	pos := pp.FindPort(cardName, portName)
	if pos.tp == PortTypeInvalid {
		logger.Warning("set first failed: port not found, card,port", cardName, portName)
		return false
	}

	// 将目标端口的类型移到类型列表的最前面
	if pp.Types[0] != pos.tp {
		newTypes := make(PriorityTypeList, 0, len(pp.Types))
		newTypes = append(newTypes, pos.tp)
		for _, tp := range pp.Types {
			if tp != pos.tp {
				newTypes = append(newTypes, tp)
			}
		}
		pp.Types = newTypes
	}

	// 在原队列中将目标端口移到最前面
	if pos.index > 0 {
		targetPort := pp.Ports[pos.tp][pos.index]
		pp.Ports[pos.tp] = append(pp.Ports[pos.tp][:pos.index], pp.Ports[pos.tp][pos.index+1:]...)
		pp.Ports[pos.tp] = append([]*PriorityPort{targetPort}, pp.Ports[pos.tp]...)
	}

	return true
}
