/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package audio

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	dbus "github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/pulse"
)

type Sink struct {
	audio   *Audio
	service *dbusutil.Service
	PropsMu sync.RWMutex
	index   uint32

	Name        string
	Description string

	// 默认音量值
	BaseVolume float64

	// 是否静音
	Mute bool

	// 当前音量
	Volume     float64
	cVolume    pulse.CVolume
	channelMap pulse.ChannelMap
	// 左右声道平衡值
	Balance float64
	// 是否支持左右声道调整
	SupportBalance bool
	// 前后声道平衡值
	Fade float64
	// 是否支持前后声道调整
	SupportFade bool

	// dbusutil-gen: equal=portsEqual
	// 支持的输出端口
	Ports []Port
	// 当前使用的输出端口
	ActivePort Port
	// 声卡的索引
	Card uint32

	props map[string]string
}

func newSink(sinkInfo *pulse.Sink, audio *Audio) *Sink {
	s := &Sink{
		audio:   audio,
		service: audio.service,
		index:   sinkInfo.Index,
		props:   sinkInfo.PropList,
	}
	s.update(sinkInfo)
	return s
}

// 检测端口是否被禁用
func (s *Sink) CheckPort() *dbus.Error {
	enabled, err := s.audio.IsPortEnabled(s.Card, s.ActivePort.Name)
	if err != nil {
		return err
	}

	if !enabled {
		return dbusutil.ToError(fmt.Errorf("port<%d:%s> is disabled", s.Card, s.ActivePort.Name))
	}

	return nil
}

// 设置音量大小
//
// v: 音量大小
//
// isPlay: 是否播放声音反馈
func (s *Sink) SetVolume(value float64, isPlay bool) *dbus.Error {
	err := s.CheckPort()
	if err != nil {
		return err
	}

	logger.Debugf("set #%d sink %q volume %f", s.index, s.Name, value)
	if !isVolumeValid(value) {
		return dbusutil.ToError(fmt.Errorf("invalid volume value: %v", value))
	}

	if value == 0 {
		value = 0.001
	}
	s.PropsMu.Lock()
	cv := s.cVolume.SetAvg(value)
	s.PropsMu.Unlock()
	s.audio.context().SetSinkVolumeByIndex(s.index, cv)

	GetConfigKeeper().SetVolume(s.audio.getCardNameById(s.Card), s.ActivePort.Name, value)

	if isPlay {
		s.playFeedback()
	}
	return nil
}

// 设置左右声道平衡值
//
// v: 声道平衡值
//
// isPlay: 是否播放声音反馈
func (s *Sink) SetBalance(value float64, isPlay bool) *dbus.Error {
	err := s.CheckPort()
	if err != nil {
		return err
	}

	if value < -1.00 || value > 1.00 {
		return dbusutil.ToError(fmt.Errorf("invalid volume value: %v", value))
	}

	s.PropsMu.RLock()
	cv := s.cVolume.SetBalance(s.channelMap, value)
	s.PropsMu.RUnlock()
	s.audio.context().SetSinkVolumeByIndex(s.index, cv)

	GetConfigKeeper().SetBalance(s.audio.getCardNameById(s.Card), s.ActivePort.Name, value)

	if isPlay {
		s.playFeedback()
	}
	return nil
}

// 设置前后声道平衡值
//
// v: 声道平衡值
//
// isPlay: 是否播放声音反馈
func (s *Sink) SetFade(value float64) *dbus.Error {
	err := s.CheckPort()
	if err != nil {
		return err
	}

	if value < -1.00 || value > 1.00 {
		return dbusutil.ToError(fmt.Errorf("invalid volume value: %v", value))
	}

	s.PropsMu.RLock()
	cv := s.cVolume.SetFade(s.channelMap, value)
	s.PropsMu.RUnlock()
	s.audio.context().SetSinkVolumeByIndex(s.index, cv)
	s.playFeedback()
	return nil
}

// 设置同时设置音量和平衡
//
// v: volume音量值
// b: balance左右平衡值
// f: fade前后平衡值
//
func (s *Sink) setVBF(v, b, f float64) *dbus.Error {
	if !isVolumeValid(v) {
		return dbusutil.ToError(fmt.Errorf("invalid volume value: %v", v))
	}

	if b < -1.00 || b > 1.00 {
		return dbusutil.ToError(fmt.Errorf("invalid balance value: %v", b))
	}

	if f < -1.00 || b > 1.00 {
		return dbusutil.ToError(fmt.Errorf("invalid fade value: %v", f))
	}

	s.PropsMu.RLock()
	cv := s.cVolume.SetAvg(v)
	cv = cv.SetBalance(s.channelMap, b)
	cv = cv.SetFade(s.channelMap, f)
	s.PropsMu.RUnlock()
	s.audio.context().SetSinkVolumeByIndex(s.index, cv)

	return nil
}

// 是否静音
func (s *Sink) SetMute(value bool) *dbus.Error {
	err := s.CheckPort()
	if err != nil {
		return err
	}

	logger.Debugf("Sink #%d SetMute %v", s.index, value)
	s.audio.context().SetSinkMuteByIndex(s.index, value)
	GetConfigKeeper().SetMuteOutput(value)

	if !value {
		s.playFeedback()
	}
	return nil
}

// 设置此设备的当前使用端口
func (s *Sink) SetPort(name string) *dbus.Error {
	s.audio.context().SetSinkPortByIndex(s.index, name)
	return nil
}

func (s *Sink) getPath() dbus.ObjectPath {
	return dbus.ObjectPath(dbusPath + "/Sink" + strconv.Itoa(int(s.index)))
}

func (*Sink) GetInterfaceName() string {
	return dbusInterface + ".Sink"
}

func (s *Sink) update(sinkInfo *pulse.Sink) {
	s.PropsMu.Lock()

	s.Name = sinkInfo.Name
	s.Description = sinkInfo.Description
	s.Card = sinkInfo.Card
	s.BaseVolume = sinkInfo.BaseVolume.ToPercent()
	s.cVolume = sinkInfo.Volume
	s.channelMap = sinkInfo.ChannelMap

	s.setPropMute(sinkInfo.Mute)
	s.setPropVolume(floatPrecision(sinkInfo.Volume.Avg()))

	s.setPropSupportFade(false)
	s.setPropFade(sinkInfo.Volume.Fade(sinkInfo.ChannelMap))
	s.setPropSupportBalance(true)
	s.setPropBalance(sinkInfo.Volume.Balance(sinkInfo.ChannelMap))

	oldActivePort := s.ActivePort
	newActivePort := toPort(sinkInfo.ActivePort)
	var activePortChanged bool

	s.setPropPorts(toPorts(sinkInfo.Ports))
	activePortChanged = s.setPropActivePort(newActivePort)

	s.props = sinkInfo.PropList
	s.PropsMu.Unlock()

	if activePortChanged && s.audio.defaultSinkName == s.Name {
		logger.Debugf("default sink update active port %s", sinkInfo.ActivePort.Name)
		s.audio.resumeSinkConfig(s)
	}

	// TODO(jouyouyun): Sometimes the default sink not in the same card, so the activePortChanged inaccurate.
	// The right way is saved the last default sink active port, then judge whether equal.
	if s.audio.headphoneUnplugAutoPause && activePortChanged {
		logger.Debugf("sink #%d active port changed, old %v, new %v",
			s.index, oldActivePort, newActivePort)
		// old port but has new available state
		oldPort, foundOldPort := getPortByName(s.Ports, oldActivePort.Name)
		var oldPortUnavailable bool
		if !foundOldPort {
			logger.Debug("Sink.update not found old port")
			oldPortUnavailable = true
		} else {
			oldPortUnavailable = int(oldPort.Available) == pulse.AvailableTypeNo
		}
		logger.Debugf("oldPortUnavailable: %v", oldPortUnavailable)

		handleUnplugedEvent(oldActivePort, newActivePort, oldPortUnavailable)
	}
}

func handleUnplugedEvent(oldActivePort, newActivePort Port, oldPortUnavailable bool) {
	logger.Debug("[handleUnplugedEvent] Old port:", oldActivePort.String(), oldPortUnavailable)
	logger.Debug("[handleUnplugedEvent] New port:", newActivePort.String())
	// old active port is headphone or bluetooth
	if isHeadphoneHeadsetOrLineoutPort(oldActivePort.Name) &&
		// old active port available is yes or unknown, not no
		int(oldActivePort.Available) != pulse.AvailableTypeNo &&
		// new port is not headphone and bluetooth
		!isHeadphoneHeadsetOrLineoutPort(newActivePort.Name) && oldPortUnavailable {
		pauseAllPlayers()
	}
}

func isHeadphoneHeadsetOrLineoutPort(portName string) bool {
	name := strings.ToLower(portName)
	return strings.Contains(name, "headphone") || strings.Contains(name, "headset-output") || strings.Contains(name, "lineout")
}

func (s *Sink) GetMeter() (meter dbus.ObjectPath, busErr *dbus.Error) {
	//TODO
	return "/", nil
}

func (s *Sink) playFeedback() {
	s.PropsMu.RLock()
	name := s.Name
	s.PropsMu.RUnlock()
	playFeedbackWithDevice(name)
}

func (s *Sink) setMute(v bool) {
	logger.Debugf("Sink #%d setMute %v", s.index, v)
	s.audio.context().SetSinkMuteByIndex(s.index, v)
}
