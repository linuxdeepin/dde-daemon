// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/go-lib/pulse"
)

// installIsolatedConfigKeeper 用临时目录下的配置文件替换全局配置单例。
//
// ConfigKeeper.SetEnabled 会调用 Save() 把整份配置写回磁盘，若沿用全局单例，
// 跑测试就会覆盖用户真实的 ~/.config/deepin/dde-daemon/audio-config-keeper.json。
func installIsolatedConfigKeeper(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	old := GetConfigKeeper
	GetConfigKeeper = createConfigKeeperSingleton(
		filepath.Join(dir, "audio-config-keeper.json"),
		filepath.Join(dir, "audio-config-keeper-mute.json"),
	)
	t.Cleanup(func() { GetConfigKeeper = old })
}

// mkCard 构造一个测试用 PulseAudio 声卡
func mkCard(coreName, alsaName string, ports ...pulse.CardPortInfo) *pulse.Card {
	return &pulse.Card{
		Name: coreName,
		PropList: map[string]string{
			"alsa.card_name": alsaName,
		},
		Ports: ports,
	}
}

// mkPort 构造一个测试用声卡端口
func mkPort(name, desc string, dir int, prio uint32, avail int) pulse.CardPortInfo {
	return pulse.CardPortInfo{
		PortInfo: pulse.PortInfo{
			Name:        name,
			Description: desc,
			Priority:    prio,
			Available:   avail,
		},
		Direction: dir,
	}
}

// newPortPriorityTestManager 构造一个仅含输出方向、类型优先级可预期的 PriorityManager
func newPortPriorityTestManager() *PriorityManager {
	pm := NewPriorityManager("")
	pm.Output.Types = []int{
		PortTypeBluetooth, PortTypeHeadset, PortTypeUsb, PortTypeBuiltin,
		PortTypeHdmi, PortTypeLineIO, PortTypeMultiChannel, PortTypeUnknown,
	}
	pm.Input.Types = append([]int{}, pm.Output.Types...)
	return pm
}

// 场景：连接有线耳机、HDMI、双端口 USB 耳机，在设备管理中禁用「有线耳机」和
// 「USB 扬声器」，期望优选输出设备为 USB 耳机的另一个端口（耳机）。
//
// USB 声卡切换 profile 时会短暂上报 Available=No 的中间态端口。回归点：
// refreshPorts 不能因为这种瞬时不可用就把已启用端口从优先级队列中删除，
// 否则会持久化回退到 HDMI（对应的实际表现是界面显示 HDMI 但声音仍从耳机输出）。
func Test_RefreshPortsKeepsEnabledPortDuringProfileTransition(t *testing.T) {
	installIsolatedConfigKeeper(t)

	analog := mkCard("alsa_card.pci-0000_00_1f.4", "内置音频",
		mkPort("analog-output-speaker", "扬声器", pulse.DirectionSink, 50, pulse.AvailableTypeYes),
		mkPort("analog-output-headphones", "有线耳机", pulse.DirectionSink, 60, pulse.AvailableTypeYes),
	)
	hdmi := mkCard("alsa_card.pci-0000_01_00.2", "HDMI",
		mkPort("hdmi-output-0", "HDMI / DisplayPort", pulse.DirectionSink, 100, pulse.AvailableTypeYes),
	)
	hdmi.Index = 7
	usb := mkCard("alsa_card.usb-Generic_USB_Audio-01", "USB Audio",
		mkPort("analog-output-speaker", "扬声器", pulse.DirectionSink, 40, pulse.AvailableTypeYes),
		mkPort("analog-output-headphones", "耳机", pulse.DirectionSink, 45, pulse.AvailableTypeNo),
	)

	cards := newCardList([]*pulse.Card{analog, hdmi, usb})

	// 用户禁用：有线耳机 + USB扬声器，两者均从优先级队列中移除
	GetConfigKeeper().SetEnabled(cards[0], "analog-output-headphones", false)
	GetConfigKeeper().SetEnabled(cards[2], "analog-output-speaker", false)

	pm := newPortPriorityTestManager()
	pm.refreshPorts(cards)

	first, pos := pm.Output.GetTheFirstPort()
	if first == nil {
		t.Fatal("expected a preferred output port, got none")
	}
	if first.CardName != usb.Name || first.PortName != "analog-output-headphones" {
		t.Fatalf("expected USB headphones %s/%s as preferred output, got %s/%s (tp=%d)",
			usb.Name, "analog-output-headphones", first.CardName, first.PortName, pos.tp)
	}
}

// 回归点：refreshPorts 只按 Enabled 过滤，不按 Available 过滤，
// 因此暂时不可用但已启用的端口仍应保留在优先级队列中。
func Test_RefreshPortsRetainsUnavailableButEnabledPort(t *testing.T) {
	installIsolatedConfigKeeper(t)

	usb := mkCard("alsa_card.usb-Generic_USB_Audio-02", "USB Audio",
		mkPort("analog-output-headphones", "耳机", pulse.DirectionSink, 45, pulse.AvailableTypeNo),
	)

	cards := newCardList([]*pulse.Card{usb})
	pm := newPortPriorityTestManager()
	pm.refreshPorts(cards)

	for _, tp := range pm.Output.Types {
		for _, p := range pm.Output.Ports[tp] {
			if p.CardName == usb.Name && p.PortName == "analog-output-headphones" {
				return
			}
		}
	}
	t.Fatal("enabled port with transient Available=No was dropped from the priority queue")
}

// 回归点：禁用的端口必须从优先级队列中移除。
func Test_RefreshPortsRemovesDisabledPort(t *testing.T) {
	installIsolatedConfigKeeper(t)

	usb := mkCard("alsa_card.usb-Generic_USB_Audio-03", "USB Audio",
		mkPort("analog-output-headphones", "耳机", pulse.DirectionSink, 45, pulse.AvailableTypeYes),
	)

	cards := newCardList([]*pulse.Card{usb})
	GetConfigKeeper().SetEnabled(cards[0], "analog-output-headphones", false)

	pm := newPortPriorityTestManager()
	pm.refreshPorts(cards)

	for _, tp := range pm.Output.Types {
		for _, p := range pm.Output.Ports[tp] {
			if p.CardName == usb.Name && p.PortName == "analog-output-headphones" {
				t.Fatal("disabled port must not remain in the priority queue")
			}
		}
	}
}

// 回归点：PipeWire 报端口可用，但配置中该端口已被禁用时，自动切换不得选中它。
func Test_CheckAutoSwitchSkipsEnabledConfigMismatch(t *testing.T) {
	installIsolatedConfigKeeper(t)

	headphone := mkCard("alsa_card.pci-0000_00_1f.4", "内置音频",
		mkPort("analog-output-headphones", "有线耳机", pulse.DirectionSink, 60, pulse.AvailableTypeYes),
	)
	hdmi := mkCard("alsa_card.pci-0000_01_00.2", "HDMI",
		mkPort("hdmi-output-0", "HDMI / DisplayPort", pulse.DirectionSink, 100, pulse.AvailableTypeYes),
	)
	hdmi.Index = 7
	cards := newCardList([]*pulse.Card{headphone, hdmi})
	GetConfigKeeper().SetEnabled(cards[0], "analog-output-headphones", false)

	pm := newPortPriorityTestManager()
	pm.Output.Ports[PortTypeHeadset] = []*PriorityPort{
		{CardName: headphone.Name, PortName: "analog-output-headphones", PortType: PortTypeBuiltin},
	}
	pm.Output.Ports[PortTypeHdmi] = []*PriorityPort{
		{CardName: hdmi.Name, PortName: "hdmi-output-0", PortType: PortTypeHdmi},
	}

	// 模拟 profile 切换中间态：优先级队列仍含有线耳机，PipeWire 也报其可用，
	// 但 ConfigKeeper 已记录用户禁用。校验逻辑必须跳过它，选中下一个 HDMI 候选。
	card := cards[0]
	portInfo, err := card.core.Ports.Get("analog-output-headphones", pulse.DirectionSink)
	if err != nil {
		t.Fatal(err)
	}
	if portInfo.Available != pulse.AvailableTypeYes {
		t.Fatal("test setup expects PipeWire to report the disabled port as available")
	}
	if _, portConfig := GetConfigKeeper().GetCardAndPortConfig(card, "analog-output-headphones"); portConfig.Enabled {
		t.Fatal("test setup expects ConfigKeeper to record the port as disabled")
	}

	cardByName, err := cards.getByName(headphone.Name)
	if err != nil {
		t.Fatal(err)
	}
	if cardByName.Name != card.Name || !sameCard(cardByName, card) {
		t.Fatal("expected priority candidate lookup to return the disabled headphone card")
	}

	if isOutputPortSelectable(card, "analog-output-headphones", portInfo) {
		t.Fatal("disabled output port must not be selectable")
	}
	if !isOutputPortSelectable(cards[1], "hdmi-output-0", pulse.CardPortInfo{
		PortInfo:  pulse.PortInfo{Name: "hdmi-output-0", Available: pulse.AvailableTypeYes},
		Direction: pulse.DirectionSink,
	}) {
		t.Fatal("enabled output port must be selectable")
	}
}

func sameCard(left, right *Card) bool {
	return left == right
}
