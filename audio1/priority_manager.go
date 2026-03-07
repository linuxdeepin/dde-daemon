// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/linuxdeepin/go-lib/pulse"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

// dconfig默认优先级
var (
	inputDefaultPriorities  = []int{}
	outputDefaultPriorities = []int{}
)

// 优先级策略组，包含输入和输出
type PriorityManager struct {
	Output *PriorityPolicy
	Input  *PriorityPolicy

	file string // 配置文件的路径，私有成员不会被json导出
}

// 创建优先级组
func NewPriorityManager(path string) *PriorityManager {
	return &PriorityManager{
		Output: NewPriorityPolicy(),
		Input:  NewPriorityPolicy(),
		file:   path,
	}
}

// 创建单例
func createPriorityManagerSingleton(path string) func() *PriorityManager {
	var pm *PriorityManager = nil
	return func() *PriorityManager {
		if pm == nil {
			pm = NewPriorityManager(path)
		}
		return pm
	}
}

// 获取单例
// 由于优先级管理需要在很多个对象中使用，放在Audio对象中需要添加额外参数传递到各个模块很不方便，因此在此创建一个全局的单例
var globalPrioritiesFilePath = filepath.Join(basedir.GetUserConfigDir(), "deepin/dde-daemon/priorities.json")
var GetPriorityManager = createPriorityManagerSingleton(globalPrioritiesFilePath)

// 打印优先级列表，用于调试
func (pm *PriorityManager) Print() {
	data, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		logger.Warning(err)
		return
	}

	logger.Debugf("PriorityManager:\n %s", string(data))
}

// 保存配置文件
func (pm *PriorityManager) Save() {
	data, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		logger.Warning(err)
		return
	}

	err = os.WriteFile(pm.file, data, 0644)
	if err != nil {
		logger.Warning(err)
		return
	}
}

// 读取配置文件
func (pm *PriorityManager) Load() bool {
	data, err := os.ReadFile(pm.file)
	if err != nil {
		logger.Warningf("failed to read file '%s': %v", pm.file, err)
		return false
	}

	err = json.Unmarshal(data, pm)
	if err != nil {
		logger.Warningf("failed to parse json of file '%s': %v", pm.file, err)
		return false
	}

	return true
}

// 加载dconfig默认优先级
func (pm *PriorityManager) completeDefaultTypes() {
	for _, i := range inputDefaultPriorities {
		if !pm.Input.Types.hasElement(i) {
			pm.Input.Types = append(pm.Input.Types, i)
			logger.Debugf("input defualt append type %d", i)
		}
	}
	for _, i := range outputDefaultPriorities {
		if !pm.Output.Types.hasElement(i) {
			pm.Output.Types = append(pm.Output.Types, i)
			logger.Debugf("output defualt append type %d", i)
		}
	}
	pm.Output.completeTypes()
	pm.Input.completeTypes()

}

// cleanupMismatchedPorts 检查配置，删除端口类型和队列类型不匹配的问题
func (pm *PriorityManager) cleanupMismatchedPorts() {
	pm.cleanupPolicyMismatchedPorts(pm.Output, "Output")
	pm.cleanupPolicyMismatchedPorts(pm.Input, "Input")
}

// cleanupPolicyMismatchedPorts 清理单个策略中端口类型与队列类型不匹配的端口
func (pm *PriorityManager) cleanupPolicyMismatchedPorts(policy *PriorityPolicy, policyName string) {
	if policy == nil || policy.Ports == nil {
		return
	}

	removedCount := 0
	for queueType, portList := range policy.Ports {
		// 检查队列类型是否有效
		if queueType < 0 || queueType >= PortTypeCount {
			logger.Warningf("Invalid queue type %d in %s policy, removing all ports", queueType, policyName)
			delete(policy.Ports, queueType)
			removedCount += len(portList)
			continue
		}

		// 检查每个端口的类型是否与队列类型匹配
		validPorts := make([]*PriorityPort, 0, len(portList))
		for _, port := range portList {
			if port == nil {
				logger.Warningf("Found nil port in %s policy queue %d, skipping", policyName, queueType)
				removedCount++
				continue
			}

			// 检查端口类型是否与队列类型匹配
			if port.PortType != queueType {
				logger.Warningf("Port type mismatch in %s policy: port %s:%s has type %d but is in queue %d, removing",
					policyName, port.CardName, port.PortName, port.PortType, queueType)
				removedCount++
				continue
			}

			// 检查端口类型是否有效
			if port.PortType < 0 || port.PortType >= PortTypeCount {
				logger.Warningf("Invalid port type %d for port %s:%s in %s policy, removing",
					port.PortType, port.CardName, port.PortName, policyName)
				removedCount++
				continue
			}

			validPorts = append(validPorts, port)
		}

		// 更新队列，只保留有效的端口
		if len(validPorts) != len(portList) {
			policy.Ports[queueType] = validPorts
		}

		// 如果队列为空，删除该队列
		if len(validPorts) == 0 {
			delete(policy.Ports, queueType)
		}
	}

	if removedCount > 0 {
		logger.Infof("Cleaned up %d mismatched ports from %s policy", removedCount, policyName)
	}
}

// 进行初始化
func (pm *PriorityManager) Init(cards CardList) {
	pm.Load()
	// 补充缺少的类型（产品修改了端口类型），并更新端口排序
	pm.completeDefaultTypes()
	// 检查配置，删除端口类型和队列类型不匹配的问题
	pm.cleanupMismatchedPorts()
	pm.refreshPorts(cards)

	pm.Save()
}

func (pm *PriorityManager) refreshPorts(cards CardList) {
	// 收集当前所有可用的端口
	currentPorts := make(map[int]map[string]map[string]bool) // [direction][cardName][portName]
	currentPorts[pulse.DirectionSink] = make(map[string]map[string]bool)
	currentPorts[pulse.DirectionSource] = make(map[string]map[string]bool)

	for _, card := range cards {
		// 先对声卡的端口进行排序和调整
		pm.arrangeCardPorts(card)

		// 收集该声卡的新端口（按方向分组）
		newSinkPorts := make([]pulse.CardPortInfo, 0)
		newSourcePorts := make([]pulse.CardPortInfo, 0)

		for _, port := range card.Ports {
			// 端口不可用或被禁用时，不插入到优先级列表中
			_, pc := GetConfigKeeper().GetCardAndPortConfig(card, port.Name)
			if port.Available == pulse.AvailableTypeNo || !pc.Enabled {
				continue
			}

			// 记录当前可用端口
			if currentPorts[port.Direction][card.core.Name] == nil {
				currentPorts[port.Direction][card.core.Name] = make(map[string]bool)
			}
			currentPorts[port.Direction][card.core.Name][port.Name] = true

			// 检查是否为新端口
			var policy *PriorityPolicy
			if port.Direction == pulse.DirectionSink {
				policy = pm.Output
			} else {
				policy = pm.Input
			}

			pos := policy.FindPort(card.core.Name, port.Name)
			if pos.tp == PortTypeInvalid {
				// 收集新端口
				if port.Direction == pulse.DirectionSink {
					newSinkPorts = append(newSinkPorts, port)
				} else {
					newSourcePorts = append(newSourcePorts, port)
				}
			}
		}

		// 插入该声卡的新端口（倒序插入以保持优先级顺序）
		// 排序后：[默认端口, 高优先级, 中优先级, 低优先级]
		// 倒序插入到队首：低 → 中 → 高 → 默认端口
		// 最终队列：[默认端口, 高优先级, 中优先级, 低优先级] 在队首
		for i := len(newSinkPorts) - 1; i >= 0; i-- {
			pm.Output.InsertPort(card.core, &newSinkPorts[i])
		}
		for i := len(newSourcePorts) - 1; i >= 0; i-- {
			pm.Input.InsertPort(card.core, &newSourcePorts[i])
		}
	}

	// 删除不存在的端口
	pm.removeNonExistentPorts(pm.Output, currentPorts[pulse.DirectionSink])
	pm.removeNonExistentPorts(pm.Input, currentPorts[pulse.DirectionSource])

	pm.Print()
	logger.Debugf("refresh ports completed")
}

// removeNonExistentPorts 删除不存在的端口和声卡
func (pm *PriorityManager) removeNonExistentPorts(policy *PriorityPolicy, currentPorts map[string]map[string]bool) {
	for tp := range policy.Ports {
		newList := make([]*PriorityPort, 0)
		for _, port := range policy.Ports[tp] {
			// 检查声卡是否还存在
			if ports, ok := currentPorts[port.CardName]; ok {
				// 检查端口是否还存在
				if ports[port.PortName] {
					newList = append(newList, port)
				}
			}
			// 如果声卡不存在（!ok），端口会被自动过滤掉
		}
		policy.Ports[tp] = newList
	}
}

// arrangeCardPorts 对声卡的端口列表进行排序和调整
// 1. 按 Priority 从高到低排序（值大的在前）
// 2. 将默认端口移到最前面（倒序插入时会最后插入，获得最高优先级）
func (pm *PriorityManager) arrangeCardPorts(card *Card) {
	if card == nil || len(card.Ports) == 0 {
		return
	}

	// 按 Priority 从高到低排序（Priority 值大的在前）
	sort.SliceStable(card.Ports, func(i, j int) bool {
		return card.Ports[i].Priority > card.Ports[j].Priority
	})

	// 获取默认端口
	defaultOutputPort := GetConfigKeeper().GetDefaultPort(card.core.Name, pulse.DirectionSink)
	defaultInputPort := GetConfigKeeper().GetDefaultPort(card.core.Name, pulse.DirectionSource)

	// 将默认端口移到最前面（倒序插入时会最后插入，获得最高优先级）
	if defaultOutputPort != "" {
		pm.moveCardPortToFirst(card.Ports, defaultOutputPort)
	}
	if defaultInputPort != "" {
		pm.moveCardPortToFirst(card.Ports, defaultInputPort)
	}
}

// moveCardPortToFirst 将指定端口移到声卡端口列表的最前面
func (pm *PriorityManager) moveCardPortToFirst(ports pulse.CardPortInfos, portName string) {
	if len(ports) == 0 {
		return
	}

	// 查找端口的索引
	portIndex := -1
	for i, port := range ports {
		if port.Name == portName {
			portIndex = i
			break
		}
	}

	// 如果找到端口且不在第一个位置，将其移到最前面
	if portIndex > 0 {
		port := ports[portIndex]
		// 将前面的端口向后移动
		copy(ports[1:portIndex+1], ports[0:portIndex])
		// 插入到最前面
		ports[0] = port
	}
}

func (pm *PriorityManager) GetTheFirstPort(direction int) (*PriorityPort, *Position) {
	switch direction {
	case pulse.DirectionSink:
		if len(pm.Output.Ports) > 0 {
			return pm.Output.GetTheFirstPort()
		}
	case pulse.DirectionSource:
		if len(pm.Input.Ports) > 0 {
			return pm.Input.GetTheFirstPort()
		}
	}
	return nil, &Position{tp: PortTypeInvalid, index: -1}
}

func (pm *PriorityManager) SetTheFirstPort(cardName string, portName string, direction int) {
	switch direction {
	case pulse.DirectionSink:
		pm.Output.SetTheFirstPort(cardName, portName)
	case pulse.DirectionSource:
		pm.Input.SetTheFirstPort(cardName, portName)
	default:
		logger.Warningf("unexpected direction %d of port <%s:%s>",
			direction, cardName, portName)
		return
	}

	// 设置为声卡的默认端口（根据方向）
	GetConfigKeeper().SetDefaultPort(cardName, portName, direction)

	// 打印并保存
	pm.Print()
	pm.Save()
}

func (pm *PriorityManager) LoopAvaiablePort(direction int, pos *Position) (*PriorityPort, *Position) {
	if pos == nil {
		return pm.GetTheFirstPort(direction)
	} else {
		switch direction {
		case pulse.DirectionSink:
			if len(pm.Output.Ports) > 0 {
				return pm.Output.GetNextPort(pos)
			}
		case pulse.DirectionSource:
			if len(pm.Input.Ports) > 0 {
				return pm.Input.GetNextPort(pos)
			}
		}
		return nil, &Position{tp: PortTypeInvalid, index: -1}
	}
}
