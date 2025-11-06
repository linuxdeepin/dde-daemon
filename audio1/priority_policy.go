// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
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
	PortType int // 部分声卡需要在Property里判断类型，只有CardName和PortName不足以用来判断PortType，因此添加此项
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

func (pp *PriorityPolicy) InsertPort(card *pulse.Card, port *pulse.CardPortInfo) {
	pos := pp.FindPort(card.Name, port.Name)
	if pos.tp != PortTypeInvalid {
		// 已存在，无需插入
		return
	}
	tp := DetectPortType(card, port)
	newPort := &PriorityPort{
		CardName: card.Name,
		PortName: port.Name,
		PortType: tp,
	}
	// 新端口添加到队列最前
	pp.Ports[tp] = append([]*PriorityPort{newPort}, pp.Ports[tp]...)
}

func (pp *PriorityPolicy) GetTheFirstPort(available portList) (*PriorityPort, *Position) {
	for _, tp := range pp.Types {
		for i, port := range pp.Ports[tp] {
			if available.isExists(port.CardName, port.PortName) {
				return port, &Position{tp: tp, index: i}
			}
		}
	}
	return nil, &Position{tp: PortTypeInvalid, index: -1}
}

func (pp *PriorityPolicy) GetNextPort(available portList, pos *Position) (*PriorityPort, *Position) {
	if pos.tp == PortTypeInvalid {
		return nil, &Position{tp: PortTypeInvalid, index: -1}
	}
	for tp := pos.tp; tp < PortTypeCount; tp++ {
		for i := pos.index + 1; i < len(pp.Ports[tp]); i++ {
			port := pp.Ports[tp][i]
			if ports, ok := available[port.CardName]; ok {
				if ports.Contains(port.PortName) {
					return port, &Position{tp: tp, index: i}
				}
			}
		}
	}
	return nil, &Position{tp: PortTypeInvalid, index: -1}
}

func (pp *PriorityPolicy) SetTheFirstPort(cardName string, portName string, available portList) bool {
	if !available.isExists(cardName, portName) {
		logger.Warningf("port <%v:%v> not avaiable", cardName, portName)
		return false
	}

	// 第一步：找到并删除指定的端口
	var targetPort *PriorityPort
	var targetType int
	for _, tp := range pp.Types {
		for i, p := range pp.Ports[tp] {
			if p.CardName == cardName && p.PortName == portName {
				// 保存端口信息
				targetPort = &PriorityPort{
					CardName: p.CardName,
					PortName: p.PortName,
					PortType: p.PortType,
				}
				targetType = tp
				// 从列表中删除这个端口
				pp.Ports[tp] = append(pp.Ports[tp][:i], pp.Ports[tp][i+1:]...)
				break
			}
		}
		if targetPort != nil {
			break
		}
	}

	if targetPort == nil {
		logger.Warning("set first failed: port not found, card,port", cardName, portName)
		return false
	}
	// 第二步：找到第一个可用端口的位置，并在其之前插入目标端口
	for _, tp := range pp.Types {
		for i, p := range pp.Ports[tp] {
			if available.isExists(p.CardName, p.PortName) {
				// 找到第一个可用端口，在它之前插入目标端口
				// 创建新的切片以避免切片操作问题
				newList := make([]*PriorityPort, 0, len(pp.Ports[tp])+1)
				newList = append(newList, pp.Ports[tp][:i]...)
				newList = append(newList, targetPort)
				newList = append(newList, pp.Ports[tp][i:]...)
				pp.Ports[tp] = newList
				return true
			}
		}
	}

	// 如果没有找到任何可用端口，将目标端口放回原来的类型列表的开头
	pp.Ports[targetType] = append([]*PriorityPort{targetPort}, pp.Ports[targetType]...)
	logger.Warning("set first failed: no available port found, card,port", cardName, portName)
	return false
}
