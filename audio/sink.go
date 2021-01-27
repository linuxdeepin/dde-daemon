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
	AudioSinkName2 = "alsa_output.platform-sound_hi6405.analog-stereo"           //panguv物理声卡的sink
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
func (s *Sink) doSetVolumeToSetting( v float64) {
	if s.audio.isPanguV && s.audio.createAudioObjFinish {
		//存在激活port才有效
		iv := int32(floatPrecision(v) * 100.0)
		activePort := s.ActivePort.Name
		portAvai := s.ActivePort.Available

		if s.Name == AudioSinkName1 || s.Name == AudioSinkName2 {
			if activePort == "" {
				s.audio.settings.SetInt("physical-output-volume", iv)
				return
			}

			if portAvai == 0 || portAvai == 2 {
				s.audio.settings.SetInt("headphone-volume", iv)
			} else {
				s.audio.settings.SetInt("physical-output-volume", iv)
			}
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
	s.doSetVolumeToSetting(v)
	logger.Debugf("Set Volume [sink:%s] %v ", s.Name, v)

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

//TODO bug55140 只针对panguv，通过读取gsetting配置里保存的音量值设置对应sink的音量
func (s *Sink) setAndCheckSinkVolumeByGsetting(sinkInfo *pulse.Sink, sinkVolume *float64) {
	if s.audio.isPanguV && s.audio.createAudioObjFinish {
		activePort := sinkInfo.ActivePort.Name
		if activePort == "" {
			logger.Debug("Failed: Get Sink's activePort is Null & return")
			return
		}

		var setVolome int32
		var sinkVolume_tmp float64

		switch sinkInfo.Name {
		case AudioSinkName1, AudioSinkName2:
			portAvai := sinkInfo.ActivePort.Available
			if portAvai == 0 || portAvai == 2 {
				setVolome = s.audio.settings.GetInt("headphone-volume")
			} else {
				setVolome = s.audio.settings.GetInt("physical-output-volume")
			}
		default:
			return
		}

		sinkVolume_tmp = floatPrecision(float64(setVolome) / 100.0)
		if *sinkVolume != sinkVolume_tmp {
			logger.Debugf("Sink update need change volume:%v to setting's volume:%v", *sinkVolume, sinkVolume_tmp)
			*sinkVolume = sinkVolume_tmp
			cv := sinkInfo.Volume.SetAvg(sinkVolume_tmp)
			s.audio.context().SetSinkVolumeByIndex(sinkInfo.Index, cv)
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
	s.setAndCheckSinkVolumeByGsetting(sinkInfo, &sink_volume)

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
