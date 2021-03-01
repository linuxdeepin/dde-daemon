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

	dbus "pkg.deepin.io/lib/dbus1"
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

	methods *struct {
		SetVolume  func() `in:"value,isPlay"`
		SetBalance func() `in:"value,isPlay"`
		SetFade    func() `in:"value"`
		SetMute    func() `in:"value"`
		SetPort    func() `in:"name"`
		GetMeter   func() `out:"meter"`
	}
}

//TODO bug55140
var (
	AudioSinkName1 = "histen_sink"                                               //虚拟sink映射AudioSinkName2
	AudioSinkName2 = "alsa_output.platform-sound_hi6405.analog-stereo"           //panguv内置音频声卡的sink
)

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

//TODO bug55140 write sink'volume to audio's gsetting if machine is panguV
func doSetSinkVolumeToGSetting(s *Sink, v float64) {
	if s.audio.isPanguV {
		iv := int32(floatPrecision(v) * 100.0)
		if s.ActivePort.Name == "" {
			logger.Debug("doSetVolumeToSetting->Sink Get a null activePort")
			return
		}

		//闭包函数,设置音量到audio GSetting
		var setHeadphoneVolumeForGSetting = func (v int32, selectPort int) {
			key := "headphone-volume"
			vl := s.audio.settings.GetStrv(key)
			if selectPort == 1 {
				vl[0] = strconv.Itoa(int(v))
				s.audio.settings.SetStrv(key, vl)
			} else if selectPort == 2 {
				vl[1] = strconv.Itoa(int(v))
				s.audio.settings.SetStrv(key, vl)
			}
		}

		switch s.Name {
		case AudioSinkName1:
			//修改的是huawei虚拟sink的音量,需要遍历有效sinks里区分不同的激活port和设置port的音量值
			for _, sinkInfo := range s.audio.sinks {
				if sinkInfo.Name == AudioSinkName2 {
					if sinkInfo.ActivePort.Available != 1 {
						if sinkInfo.ActivePort.Name == "analog-output-headphones-huawei" {
							setHeadphoneVolumeForGSetting(iv, 1)
						} else if sinkInfo.ActivePort.Name == "analog-output-headphones-huawei-2" {
							setHeadphoneVolumeForGSetting(iv, 2)
						}
					}
				}
			}
		case AudioSinkName2:
			if s.ActivePort.Available != 1 {
				if s.ActivePort.Name == "analog-output-headphones-huawei" {
					setHeadphoneVolumeForGSetting(iv, 1)
				} else if s.ActivePort.Name == "analog-output-headphones-huawei-2" {
					setHeadphoneVolumeForGSetting(iv, 2)
				}
			} else {
				s.audio.settings.SetInt("onboard-output-volume", iv)
			}
		default:
			return
		}
	}
}

// 设置音量大小
//
// v: 音量大小
//
// isPlay: 是否播放声音反馈
func (s *Sink) SetVolume(v float64, isPlay bool) *dbus.Error {
	if !isVolumeValid(v) {
		return dbusutil.ToError(fmt.Errorf("invalid volume value: %v", v))
	}

	if v == 0 {
		v = 0.001
	}

	//TODO bug55140
	doSetSinkVolumeToGSetting(s, v)
	logger.Debugf("Set Volume: [sink:%s] [ActivePort:%v,volume:%v] ", s.Name, s.ActivePort, v)

	s.PropsMu.Lock()
	cv := s.cVolume.SetAvg(v)
	s.PropsMu.Unlock()
	s.audio.context().SetSinkVolumeByIndex(s.index, cv)

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
func (s *Sink) SetBalance(v float64, isPlay bool) *dbus.Error {
	if v < -1.00 || v > 1.00 {
		return dbusutil.ToError(fmt.Errorf("invalid volume value: %v", v))
	}

	s.PropsMu.RLock()
	cv := s.cVolume.SetBalance(s.channelMap, v)
	s.PropsMu.RUnlock()
	s.audio.context().SetSinkVolumeByIndex(s.index, cv)
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
func (s *Sink) SetFade(v float64) *dbus.Error {
	if v < -1.00 || v > 1.00 {
		return dbusutil.ToError(fmt.Errorf("invalid volume value: %v", v))
	}

	s.PropsMu.RLock()
	cv := s.cVolume.SetFade(s.channelMap, v)
	s.PropsMu.RUnlock()
	s.audio.context().SetSinkVolumeByIndex(s.index, cv)
	s.playFeedback()
	return nil
}

// 是否静音
func (s *Sink) SetMute(v bool) *dbus.Error {
	logger.Debugf("Sink #%d SetMute %v", s.index, v)
	s.audio.context().SetSinkMuteByIndex(s.index, v)
	if !v {
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

//TODO bug55140 只针对panguv，通过读取gsetting配置里保存的音量值与pulseaudio获取的音量值做比较，设置对应sink->port的音量
func compareSinkVolumeByGSetting(s *Sink, sinkInfo *pulse.Sink, sinkVolume *float64) {
	if s.audio.isPanguV {
		if sinkInfo.ActivePort.Name == "" {
			logger.Debug("Failed: Get Sink's activePort is Null & return")
			return
		}

		var setVolome int32
		var sinkVolume_tmp float64

		//闭包函数,从audio GSetting获取音量
		var getHeadphoneVolumeFromGSetting = func (selectPort int) int32 {
			var v int
			vl := s.audio.settings.GetStrv("headphone-volume")
			if selectPort == 1 {
				v, _ = strconv.Atoi(vl[0])
			} else if selectPort == 2 {
				v, _ = strconv.Atoi(vl[1])
			}
			return int32(v)
		}

		switch sinkInfo.Name {
		case AudioSinkName1:
			//存在histen_sink情况下， 只更新内置音频声卡sink的音量即可(华为音频算法服务会同步音量到虚拟sink上)
			return
		case AudioSinkName2:
			//根据当前激活的port到audio gsetting里取headset-volume/headphone-volume键值数组对应index的值
			if sinkInfo.ActivePort.Available != 1 {
				if sinkInfo.ActivePort.Name == "analog-output-headphones-huawei" {
					setVolome = getHeadphoneVolumeFromGSetting(1)
				} else if sinkInfo.ActivePort.Name == "analog-output-headphones-huawei-2" {
					setVolome = getHeadphoneVolumeFromGSetting(2)
				}
			} else {
				setVolome = s.audio.settings.GetInt("onboard-output-volume")
			}
		default:
			return
		}

		sinkVolume_tmp = floatPrecision(float64(setVolome) / 100.0)
		if *sinkVolume != sinkVolume_tmp {
			*sinkVolume = sinkVolume_tmp
			if sinkInfo.ActivePort.Available != 1 {
				cv := sinkInfo.Volume.SetAvg(sinkVolume_tmp)
				s.audio.context().SetSinkVolumeByIndex(sinkInfo.Index, cv)
				logger.Debugf("Update Sink->ActivePort :%s need change volume: %v -> %v", sinkInfo.ActivePort.Name, *sinkVolume, sinkVolume_tmp)
			} else {
				logger.Debugf("Update Sink->ActivePort :%s but is UnAvailable(Not Update sink volume)", sinkInfo.ActivePort.Name)
			}
		}
	}
}

func (s *Sink) update(sinkInfo *pulse.Sink) {
	s.PropsMu.Lock()

	s.Name = sinkInfo.Name
	s.Description = sinkInfo.Description
	s.Card = sinkInfo.Card
	s.BaseVolume = sinkInfo.BaseVolume.ToPercent()
	s.cVolume = sinkInfo.Volume
	s.channelMap = sinkInfo.ChannelMap

	var sink_volume float64
	sink_volume = floatPrecision(sinkInfo.Volume.Avg())
	logger.Debugf("Update Sink name :%s [ActivePort:%v,volume:%v]", s.Name, sinkInfo.ActivePort, sink_volume)

	//TODO bug55140
	compareSinkVolumeByGSetting(s, sinkInfo, &sink_volume)

	s.setPropMute(sinkInfo.Mute)
	s.setPropVolume(sink_volume)

	s.setPropSupportFade(false)
	s.setPropFade(sinkInfo.Volume.Fade(sinkInfo.ChannelMap))
	s.setPropSupportBalance(true)
	s.setPropBalance(sinkInfo.Volume.Balance(sinkInfo.ChannelMap))

	oldActivePort := s.ActivePort
	newActivePort := toPort(sinkInfo.ActivePort)
	activePortChanged := s.setPropActivePort(newActivePort)

	s.setPropPorts(toPorts(sinkInfo.Ports))
	s.props = sinkInfo.PropList
	s.PropsMu.Unlock()

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

	//PanguV Machine check Unplug event: old active port is headphone, new active port is headphone too
	//differents value： Available
	newPortName := strings.ToLower(newActivePort.Name)
	isHeadphtone_NoActive := (newActivePort.Available==byte(1)) &&
					         (strings.Contains(newPortName, "headphone") || strings.Contains(newPortName, "headset-output"));

	// old active port is headphone or bluetooth
	if isHeadphoneOrHeadsetPort(oldActivePort.Name) &&
		// old active port available is yes or unknown, not no
		int(oldActivePort.Available) != pulse.AvailableTypeNo &&
		// new port is not headphone and bluetooth
		(!isHeadphoneOrHeadsetPort(newActivePort.Name) || isHeadphtone_NoActive) &&
		oldPortUnavailable {
			pauseAllPlayers()
	}
}

func isHeadphoneOrHeadsetPort(portName string) bool {
	name := strings.ToLower(portName)
	return strings.Contains(name, "headphone") || strings.Contains(name, "headset-output")
}

func (s *Sink) GetMeter() (dbus.ObjectPath, *dbus.Error) {
	//TODO
	return "/", nil
}

func (s *Sink) playFeedback() {
	s.PropsMu.RLock()
	name := s.Name
	s.PropsMu.RUnlock()
	playFeedbackWithDevice(name)
}
