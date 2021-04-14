package audio

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"pkg.deepin.io/lib/xdg/basedir"
)

type PortConfig struct {
	Name           string
	Enabled        bool
	Volume         float64
	IncreaseVolume bool
	Balance        float64
	ReduceNoise    bool
	Mute           bool
}

type CardConfig struct {
	Name  string
	Ports map[string]*PortConfig // Name => PortConfig
}

type ConfigKeeper struct {
	Cards map[string]*CardConfig // Name => CardConfig

	file string // 配置文件路径
}

// 创建单例
func createConfigKeeperSingleton(path string) func() *ConfigKeeper {
	var ck *ConfigKeeper = nil
	return func() *ConfigKeeper {
		if ck == nil {
			ck = NewConfigKeeper(path)
		}
		return ck
	}
}

// 获取单例
// 由于优先级管理需要在很多个对象中使用，放在Audio对象中需要添加额外参数传递到各个模块很不方便，因此在此创建一个全局的单例
var globalConfigKeeperFile = filepath.Join(basedir.GetUserConfigDir(), "deepin/dde-daemon/audio-config-keeper.json")
var GetConfigKeeper = createConfigKeeperSingleton(globalConfigKeeperFile)

func NewConfigKeeper(path string) *ConfigKeeper {
	return &ConfigKeeper{
		Cards: make(map[string]*CardConfig),
		file:  path,
	}
}

func NewCardConfig(name string) *CardConfig {
	return &CardConfig{
		Name:  name,
		Ports: make(map[string]*PortConfig),
	}
}

func NewPortConfig(name string) *PortConfig {
	return &PortConfig{
		Name:           name,
		Enabled:        true,
		Volume:         0.5,
		IncreaseVolume: false,
		Balance:        0.0,
		ReduceNoise:    false,
		Mute:           false,
	}
}

func (ck *ConfigKeeper) Save() error {
	data, err := json.MarshalIndent(ck.Cards, "", "  ")
	if err != nil {
		logger.Warning(err)
		return err
	}

	return ioutil.WriteFile(ck.file, data, 0644)
}

func (ck *ConfigKeeper) Load() error {
	data, err := ioutil.ReadFile(ck.file)
	if err != nil {
		logger.Warning(err)
		return err
	}

	return json.Unmarshal(data, &ck.Cards)
}

func (ck *ConfigKeeper) Print() {
	data, err := json.MarshalIndent(ck.Cards, "", "  ")
	if err != nil {
		logger.Warning(err)
		return
	}
	logger.Debug(string(data))
}

func (ck *ConfigKeeper) UpdateCardConfig(cardConfig *CardConfig) {
	ck.Cards[cardConfig.Name] = cardConfig
}

func (ck *ConfigKeeper) RemoveCardConfig(cardName string) {
	delete(ck.Cards, cardName)
}

func (ck *ConfigKeeper) GetCardAndPortConfig(cardName string, portName string) (*CardConfig, *PortConfig) {
	card, ok := ck.Cards[cardName]
	if !ok {
		card = NewCardConfig(cardName)
		port := NewPortConfig(portName)
		card.UpdatePortConfig(port)
		ck.UpdateCardConfig(card)
		return card, port
	}

	port, ok := card.Ports[portName]
	if !ok {
		port = NewPortConfig(portName)
		card.UpdatePortConfig(port)
		ck.UpdateCardConfig(card)
	}
	return card, port
}

func (ck *ConfigKeeper) SetEnabled(cardName string, portName string, enabled bool) {
	_, port := ck.GetCardAndPortConfig(cardName, portName)
	port.Enabled = enabled
	ck.Save()
}

func (ck *ConfigKeeper) SetVolume(cardName string, portName string, volume float64) {
	_, port := ck.GetCardAndPortConfig(cardName, portName)
	port.Volume = volume
	ck.Save()
}

func (ck *ConfigKeeper) SetIncreaseVolume(cardName string, portName string, enhance bool) {
	_, port := ck.GetCardAndPortConfig(cardName, portName)
	port.IncreaseVolume = enhance
	ck.Save()
}

func (ck *ConfigKeeper) SetBalance(cardName string, portName string, balance float64) {
	_, port := ck.GetCardAndPortConfig(cardName, portName)
	port.Balance = balance
	ck.Save()
}

func (ck *ConfigKeeper) SetReduceNoise(cardName string, portName string, reduce bool) {
	_, port := ck.GetCardAndPortConfig(cardName, portName)
	port.ReduceNoise = reduce
	ck.Save()
}

func (ck *ConfigKeeper) SetMute(cardName string, portName string, mute bool) {
	_, port := ck.GetCardAndPortConfig(cardName, portName)
	port.Mute = mute
	ck.Save()
}

func (card *CardConfig) UpdatePortConfig(portConfig *PortConfig) {
	card.Ports[portConfig.Name] = portConfig
}

func (card *CardConfig) RemovePortConfig(portName string) {
	delete(card.Ports, portName)
}
