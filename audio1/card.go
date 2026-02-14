// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/linuxdeepin/go-lib/pulse"
	"github.com/linuxdeepin/go-lib/strv"
)

type Card struct {
	Id            uint32
	Name          string
	ActiveProfile *Profile
	Profiles      ProfileList
	Ports         pulse.CardPortInfos
	core          *pulse.Card
}

type CardExport struct {
	Id    uint32
	Name  string
	Ports []CardPortExport
}

type CardPortExport struct {
	Name        string
	Enabled     bool
	Bluetooth   bool
	Description string
	Direction   int
	PortType    int
}

func newCard(card *pulse.Card) *Card {
	var info = new(Card)
	info.core = card
	info.update(card)
	return info
}

func getCardName(card *pulse.Card) (name string) {
	propAlsaCardName := card.PropList["alsa.card_name"]
	propDeviceApi := card.PropList["device.api"]
	propDeviceDesc := card.PropList["device.description"]
	if propDeviceApi == "bluez" && propDeviceDesc != "" {
		name = propDeviceDesc
	} else if propAlsaCardName != "" {
		name = propAlsaCardName
	} else {
		name = card.Name
	}
	return
}

var (
	portFilterList = []string{"phone-input", "phone-output"}
)

func (a *Audio) getCardNameById(cardId uint32) string {
	if !a.isCardIdValid(cardId) {
		// 出现这个报错通常是非常严重的问题，说明PulseAudio数据同步更新的重构没有完全实现，
		// 出现此问题务必要清理掉
		// 注意：有一种情况下属于正常现象，那就是调用IsPortEnabled的时候，但是不建议调用这个接口
		if cardId != math.MaxUint32 { // 云平台没有card是正常的
			logger.Warningf("invalid card ID %d", cardId)
		}
		return ""
	}
	// 虚拟的输出cardId都是MaxUint32
	if cardId == math.MaxUint32 {
		card, err := a.cards.get(cardId)
		if err != nil {
			logger.Warning(err)
			return ""
		}

		return card.Name
	}
	card, err := a.ctx.GetCard(cardId)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	return card.Name
}

func (c *Card) update(card *pulse.Card) {
	c.Id = card.Index
	c.Name = getCardName(card)
	c.ActiveProfile = newProfile(card.ActiveProfile)
	sort.Sort(card.Profiles)
	// profile是排序过的
	c.Profiles = newProfileList(card.Profiles)
	filterList := strv.Strv(portFilterList)
	// 清空端口列表，避免端口列表中存在旧的端口信息
	c.Ports = make(pulse.CardPortInfos, 0)
	isBlue := isBluezAudio(card.Name)
	for _, port := range card.Ports {
		if filterList.Contains(port.Name) {
			logger.Debug("filter port", port.Name)
			port.Available = pulse.AvailableTypeNo
		} else {
			/* 蓝牙声卡的端口需要过滤 */
			if isBlue {
				if c.BluezMode() == bluezModeA2dp && port.Direction == pulse.DirectionSource {
					// a2dp模式过滤输入端口
					logger.Debugf("filter bluez input port %s", port.Name)
					port.Available = pulse.AvailableTypeNo
				}
				switch portBluezMode(&port) {
				case bluezModeA2dp:
					port.Priority = PriorityA2dp
				case bluezModeHeadset:
					port.Priority = PriorityHandset
				case bluezModeHandsfree:
					port.Priority = PriorityHandfree
				}
			}
		}
		c.Ports = append(c.Ports, port)
	}

	// 根据端口优先级排序，顺序由低到高
	c.sortPortsByPriority(card)
}

func portBluezMode(port *pulse.CardPortInfo) string {
	for _, profile := range port.Profiles {
		if strings.Contains(strings.ToLower(profile.Name), bluezModeA2dp) {
			return bluezModeA2dp
		}
		if strings.Contains(strings.ToLower(profile.Name), bluezModeHeadset) {
			return bluezModeHeadset
		}
		if strings.Contains(strings.ToLower(profile.Name), bluezModeHandsfree) {
			return bluezModeHeadset
		}
	}
	return ""
}

// sortPortsByPriority 根据端口的 Priority 属性排序，Priority 值小的在前（由低到高）
func (c *Card) sortPortsByPriority(card *pulse.Card) {
	sort.SliceStable(c.Ports, func(i, j int) bool {
		return c.Ports[i].Priority < c.Ports[j].Priority
	})
}

func (c *Card) tryGetProfileByPort(portName string) (string, error) {
	profile, _ := c.Ports.TrySelectProfile(portName)
	if len(profile) == 0 {
		return "", fmt.Errorf("not found profile for port '%s'", portName)
	}
	return profile, nil
}

func (c *Card) filterProfile(card *pulse.Card) {
	var profiles ProfileList
	blacklist := profileBlacklist()
	for _, p := range c.Profiles {
		// skip unavailable and blacklisted profiles
		if p.Available == 0 || blacklist.Contains(p.Name) {
			// TODO : p.Available == 0 ?
			continue
		}
		profiles = append(profiles, p)
	}
	c.Profiles = profiles
}

func (c *Card) getPortByName(name string) (pulse.CardPortInfo, error) {
	for _, port := range c.Ports {
		if port.Name == name {
			return port, nil
		}
	}

	return pulse.CardPortInfo{}, fmt.Errorf("port<%s,%s> not found", c.core.Name, name)
}

type CardList []*Card

func newCardList(cards []*pulse.Card) CardList {
	var result CardList
	for _, v := range cards {
		result = append(result, newCard(v))
	}
	return result
}

func (cards CardList) string() string {
	var list []CardExport
	for _, cardInfo := range cards {
		var ports []CardPortExport
		for _, portInfo := range cardInfo.Ports {
			_, portConfig := GetConfigKeeper().GetCardAndPortConfig(cardInfo, portInfo.Name)
			ports = append(ports, CardPortExport{
				Name:        portInfo.Name,
				Enabled:     portConfig.Enabled,
				Bluetooth:   isBluezAudio(cardInfo.core.Name),
				Description: portInfo.Description,
				Direction:   portInfo.Direction,
				PortType:    GetIconPortType(cardInfo.Name, portInfo.Name),
			})
		}

		list = append(list, CardExport{
			Id:    cardInfo.Id,
			Name:  cardInfo.Name,
			Ports: ports,
		})
	}
	return toJSON(list)
}

func (cards CardList) stringWithoutUnavailable() string {
	var list []CardExport
	for _, cardInfo := range cards {
		var ports []CardPortExport
		for _, portInfo := range cardInfo.Ports {
			if portInfo.Available == pulse.AvailableTypeNo {
				logger.Debugf("port '%s(%s)' is unavailable", portInfo.Name, portInfo.Description)
				continue
			}
			_, portConfig := GetConfigKeeper().GetCardAndPortConfig(cardInfo, portInfo.Name)
			ports = append(ports, CardPortExport{
				Name:        portInfo.Name,
				Enabled:     portConfig.Enabled,
				Bluetooth:   isBluezAudio(cardInfo.core.Name),
				Description: portInfo.Description,
				Direction:   portInfo.Direction,
				PortType:    GetIconPortType(cardInfo.Name, portInfo.Name),
			})
		}

		list = append(list, CardExport{
			Id:    cardInfo.Id,
			Name:  cardInfo.Name,
			Ports: ports,
		})
	}

	return toJSON(list)
}

func (cards CardList) get(id uint32) (*Card, error) {
	for _, info := range cards {
		if info.Id == id {
			return info, nil
		}
	}
	return nil, fmt.Errorf("invalid card id: %v", id)
}

func (cards CardList) getByName(cardName string) (*Card, error) {
	for _, info := range cards {
		if info.core.Name == cardName {
			return info, nil
		}
	}
	return nil, fmt.Errorf("invalid card name: %v", cardName)
}

func (cards CardList) add(info *Card) (CardList, bool) {
	card, _ := cards.get(info.Id)
	if card != nil {
		return cards, false
	}

	return append(cards, info), true
}

func (cards CardList) delete(id uint32) (CardList, bool) {
	var (
		ret     CardList
		deleted bool
	)
	for _, info := range cards {
		if info.Id == id {
			deleted = true
			continue
		}
		ret = append(ret, info)
	}
	return ret, deleted
}

func portAvailForCompare(v int) int {
	switch v {
	case pulse.AvailableTypeNo:
		return 0
	case pulse.AvailableTypeUnknow:
		return 1
	case pulse.AvailableTypeYes:
		return 2
	default:
		return -1
	}
}

func (cards CardList) getPassablePort(direction int) (cardId uint32,
	port *pulse.CardPortInfo) {

	for _, card := range cards {
		p := getPassablePort(card.Ports, direction)
		if p == nil {
			continue
		}

		if port == nil ||
			portAvailForCompare(p.Available) > portAvailForCompare(port.Available) ||
			(p.Available == port.Available && p.Priority > port.Priority) {
			cardId = card.Id
			port = p
		}
	}
	return
}

func getPassablePort(ports pulse.CardPortInfos, direction int) (port *pulse.CardPortInfo) {
	for idx := range ports {
		p := &ports[idx]
		if p.Direction != direction {
			continue
		}

		if port == nil ||
			portAvailForCompare(p.Available) > portAvailForCompare(port.Available) ||
			(p.Available == port.Available && p.Priority > port.Priority) {
			port = p
		}
	}
	return port
}

type ChangeType uint32

const (
	NoChange       ChangeType = 0         // 无任何变化
	ProfileChanged ChangeType = 1 << iota // 配置文件变化 (0x1)
	PortChanged                           // 端口变化 (0x2)
)

func (card *Card) doDiff(oldCard *Card, autoPause bool) ChangeType {
	var changed ChangeType = NoChange
	pc := card.core
	// 检查配置文件变化
	if card.ActiveProfile != nil {
		if pc.ActiveProfile.Name != card.ActiveProfile.Name {
			logger.Infof("card %s profile changed from %v to %v",
				card.Name, pc.ActiveProfile.Name, card.ActiveProfile.Name)
			changed |= ProfileChanged
		}
	}
	// 检查配置文件列表变化
	if len(oldCard.Profiles) != len(card.Profiles) {
		logger.Infof("card %s profiles changed", card.Name)
		changed |= ProfileChanged
	}

	// 如果删除的是当前正在使用的声卡，暂停播放
	first, _ := GetPriorityManager().GetTheFirstPort(pulse.DirectionSink)
	if first == nil {
		return changed
	}
	// 检查端口变化
	if len(oldCard.Ports) != len(card.Ports) {
		logger.Infof("card %s Ports changed", card.Name)
		if autoPause && first.CardName == card.Name {
			for _, p := range card.Ports {
				if p.Name == first.PortName && p.Available != pulse.AvailableTypeNo {
					logger.Infof("card <%s> changed:%v", card.Name, changed)
					return changed
				}
			}
			go pauseAllPlayers()
		}
		changed |= PortChanged
	} else {
		// 检查端口可用性变化
		for _, oldPort := range oldCard.Ports {
			newPort, err := card.Ports.Get(oldPort.Name, oldPort.Direction)
			if err != nil {
				logger.Warning(err)
				continue
			}

			if newPort.Available != oldPort.Available {
				logger.Debugf("card port available changed, %v.%v from %v to %v",
					oldCard.Name, oldPort.Name, oldPort.Available, newPort.Available)
				changed |= PortChanged

				// 处理端口禁用通知
				_, portConfig := GetConfigKeeper().GetCardAndPortConfig(card, oldPort.Name)
				if !portConfig.Enabled && newPort.Available != pulse.AvailableTypeNo {
					logger.Debugf("port<%s,%s> notify", card.Name, oldPort.Name)
					notifyPortDisabled(card.Id, oldPort)
				}
				if first != nil && first.CardName == card.core.Name && first.PortName == newPort.Name {
					if newPort.Available == pulse.AvailableTypeNo {
						// 当前使用的端口被禁用，暂停播放
						if autoPause {
							go pauseAllPlayers()
						}
					}
				}
			}
		}
	}
	if changed != NoChange {
		logger.Infof("card <%s> changed:%v", card.Name, changed)
	}
	return changed
}
