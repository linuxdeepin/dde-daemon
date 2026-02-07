// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/linuxdeepin/go-lib/pulse"
	"github.com/linuxdeepin/go-lib/strv"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

// dconfig默认优先级
var (
	inputDefaultPriorities  = []int{}
	outputDefaultPriorities = []int{}
)

type portList map[string]strv.Strv

// 优先级策略组，包含输入和输出
type PriorityManager struct {
	availablePort portList // 可用端口列表，key=cardName，value=portName
	Output        *PriorityPolicy
	Input         *PriorityPolicy

	file string // 配置文件的路径，私有成员不会被json导出
}

func (pl portList) isExists(cardName string, portName string) bool {
	if ports, ok := pl[cardName]; ok {
		return ports.Contains(portName)
	}
	return false
}

// 创建优先级组
func NewPriorityManager(path string) *PriorityManager {
	return &PriorityManager{
		availablePort: make(portList),
		Output:        NewPriorityPolicy(),
		Input:         NewPriorityPolicy(),
		file:          path,
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

// 进行初始化
func (pm *PriorityManager) Init(cards CardList) {
	pm.Load()
	// 补充缺少的类型（产品修改了端口类型），并更新端口排序
	pm.completeDefaultTypes()
	pm.refreshPorts(cards)

	pm.Save()
}

func (pm *PriorityManager) refreshPorts(cards CardList) {
	pm.availablePort = make(map[string]strv.Strv)
	for _, card := range cards {
		plist := make([]string, 0)
		for _, port := range card.Ports {
			switch port.Direction {
			case pulse.DirectionSink:
				pm.Output.InsertPort(card.core, &port)
			case pulse.DirectionSource:
				pm.Input.InsertPort(card.core, &port)
			}
			// 端口不可用或被禁用时，不插入插入到优先级列表中
			_, pc := GetConfigKeeper().GetCardAndPortConfig(card, port.Name)
			if port.Available == pulse.AvailableTypeNo || !pc.Enabled {
				continue
			} else {
				plist = append(plist, port.Name)
			}
		}
		pm.availablePort[card.core.Name] = plist
	}
	logger.Debugf("refresh avaiable Ports: %+v", pm.availablePort)
}

func (pm *PriorityManager) GetTheFirstPort(direction int) (*PriorityPort, *Position) {
	switch direction {
	case pulse.DirectionSink:
		if len(pm.Output.Ports) > 0 {
			return pm.Output.GetTheFirstPort(pm.availablePort)
		}
	case pulse.DirectionSource:
		if len(pm.Input.Ports) > 0 {
			return pm.Input.GetTheFirstPort(pm.availablePort)
		}
	}
	return nil, &Position{tp: PortTypeInvalid, index: -1}
}

func (pm *PriorityManager) SetTheFirstPort(cardName string, portName string, direction int) {
	switch direction {
	case pulse.DirectionSink:
		pm.Output.SetTheFirstPort(cardName, portName, pm.availablePort)
	case pulse.DirectionSource:
		pm.Input.SetTheFirstPort(cardName, portName, pm.availablePort)
	default:
		logger.Warningf("unexpected direction %d of port <%s:%s>",
			direction, cardName, portName)
	}

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
				return pm.Output.GetNextPort(pm.availablePort, pos)
			}
		case pulse.DirectionSource:
			if len(pm.Input.Ports) > 0 {
				return pm.Input.GetNextPort(pm.availablePort, pos)
			}
		}
		return nil, &Position{tp: PortTypeInvalid, index: -1}
	}
}
