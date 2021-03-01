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
	"sync"

	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/pulse"
)

type Source struct {
	audio       *Audio
	service     *dbusutil.Service
	PropsMu     sync.RWMutex
	index       uint32
	cVolume     pulse.CVolume
	channelMap  pulse.ChannelMap
	Name        string
	Description string
	// 默认的输入音量
	BaseVolume     float64
	Mute           bool
	Volume         float64
	Balance        float64
	SupportBalance bool
	Fade           float64
	SupportFade    bool
	// dbusutil-gen: equal=portsEqual
	Ports      []Port
	ActivePort Port
	// 声卡的索引
	Card uint32

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
	AudioSourceName1 = "3a_source"                                     				//华为虚拟source映射AudiosourceName2
	AudioSourceName2 = "alsa_input.platform-sound_hi6405.analog-stereo"            	//panguv内置音频声卡的source
	AudioSourceName3 = "alsa_output.platform-sound_hi6405.analog-stereo.monitor"   	//Monitor内置音频,不存在有效输入输出生成
	AudioSourceName4 = "histen_sink.monitor"										//存在histen_sink输出,不存在输入情况生成
)

func newSource(sourceInfo *pulse.Source, audio *Audio) *Source {
	s := &Source{
		audio:   audio,
		index:   sourceInfo.Index,
		service: audio.service,
	}
	s.update(sourceInfo)
	return s
}

//TODO bug55140 write source'volume to audio's gsetting if machine is panguV
func doSetSourceVolumeToGSetting(s *Source, v float64) {
	if s.audio.isPanguV {
		iv := int32(floatPrecision(v) * 100.0)
		if s.ActivePort.Name == "" {
			if s.Name == AudioSourceName3 || s.Name == AudioSourceName4 {
				s.audio.settings.SetInt("onboard-input-volume", iv)
			}
			logger.Debug("doSetVolumeToSetting->Source Get a null activePort")
			return
		}

		//闭包函数,设置音量到audio GSetting
		var setHeadsetVolumeForGSetting = func (v int32, selectPort int) {
			key := "headset-volume"
			vl := s.audio.settings.GetStrv(key)
			if selectPort == 1 {
				vl[0] = strconv.Itoa(int(v))
				s.audio.settings.SetStrv(key, vl)
			} else if selectPort == 2 {
				vl[1] = strconv.Itoa(int(v))
				s.audio.settings.SetStrv(key, vl)
			} else if selectPort == 3 {
				vl[2] = strconv.Itoa(int(v))
				s.audio.settings.SetStrv(key, vl)
			}
		}

		switch s.Name {
		case AudioSourceName1:
			//修改的是huawei虚拟source的音量,需要遍历有效sources里区分不同的激活port和设置port的音量值
			for _, sourceInfo := range s.audio.sources {
				if sourceInfo.Name == AudioSourceName2 {
					if sourceInfo.ActivePort.Available != 1 {
						if sourceInfo.ActivePort.Name == "analog-input-rear-mic-huawei" {
							setHeadsetVolumeForGSetting(iv, 1)
						} else if sourceInfo.ActivePort.Name == "analog-input-headset-mic-huawei" {
							setHeadsetVolumeForGSetting(iv, 2)
						} else if sourceInfo.ActivePort.Name == "analog-input-linein-huawei" {
							setHeadsetVolumeForGSetting(iv, 3)
						}
					}
				}
			}
		case AudioSourceName2:
			if s.ActivePort.Available != 1 {
				if s.ActivePort.Name == "analog-input-rear-mic-huawei" {
					setHeadsetVolumeForGSetting(iv, 1)
				} else if s.ActivePort.Name == "analog-input-headset-mic-huawei" {
					setHeadsetVolumeForGSetting(iv, 2)
				} else if s.ActivePort.Name == "analog-input-linein-huawei" {
					setHeadsetVolumeForGSetting(iv, 3)
				}
			} else {
				s.audio.settings.SetInt("onboard-input-volume", iv)
			}
		default:
			return
		}
	}
}

// 如何反馈输入音量？
func (s *Source) SetVolume(v float64, isPlay bool) *dbus.Error {
	if !isVolumeValid(v) {
		return dbusutil.ToError(fmt.Errorf("invalid volume value: %v", v))
	}

	if v == 0 {
		v = 0.001
	}

	//TODO bug55140
	doSetSourceVolumeToGSetting(s, v)
	logger.Debugf("Set Volume [source:%s] [ActivePort:%v,volume:%v] ", s.Name, s.ActivePort, v)

	s.PropsMu.RLock()
	cv := s.cVolume.SetAvg(v)
	s.PropsMu.RUnlock()
	s.audio.context().SetSourceVolumeByIndex(s.index, cv)

	if isPlay {
		playFeedback()
	}

	return nil
}

func (s *Source) SetBalance(v float64, isPlay bool) *dbus.Error {
	if v < -1.00 || v > 1.00 {
		return dbusutil.ToError(fmt.Errorf("invalid volume value: %v", v))
	}

	s.PropsMu.RLock()
	cv := s.cVolume.SetBalance(s.channelMap, v)
	s.PropsMu.RUnlock()
	s.audio.context().SetSourceVolumeByIndex(s.index, cv)
	if isPlay {
		playFeedback()
	}
	return nil
}

func (s *Source) SetFade(v float64) *dbus.Error {
	if v < -1.00 || v > 1.00 {
		return dbusutil.ToError(fmt.Errorf("invalid volume value: %v", v))
	}

	s.PropsMu.RLock()
	cv := s.cVolume.SetFade(s.channelMap, v)
	s.PropsMu.RUnlock()
	s.audio.context().SetSourceVolumeByIndex(s.index, cv)

	playFeedback()
	return nil
}

func (s *Source) SetMute(v bool) *dbus.Error {
	s.audio.context().SetSourceMuteByIndex(s.index, v)
	if !v {
		playFeedback()
	}
	return nil
}

func (s *Source) SetPort(name string) *dbus.Error {
	s.audio.context().SetSourcePortByIndex(s.index, name)
	return nil
}

func (s *Source) GetMeter() (dbus.ObjectPath, *dbus.Error) {
	id := fmt.Sprintf("source%d", s.index)
	s.audio.mu.Lock()
	m, ok := s.audio.meters[id]
	s.audio.mu.Unlock()
	if ok {
		return m.getPath(), nil
	}

	sourceMeter := pulse.NewSourceMeter(s.audio.ctx, s.index)
	m = newMeter(id, sourceMeter, s.audio)
	meterPath := m.getPath()
	err := s.service.Export(meterPath, m)
	if err != nil {
		return "/", dbusutil.ToError(err)
	}

	s.audio.mu.Lock()
	s.audio.meters[id] = m
	s.audio.mu.Unlock()

	m.core.ConnectChanged(func(v float64) {
		m.PropsMu.Lock()
		m.setPropVolume(v)
		m.PropsMu.Unlock()
	})
	return meterPath, nil
}

func (s *Source) getPath() dbus.ObjectPath {
	return dbus.ObjectPath(dbusPath + "/Source" + strconv.Itoa(int(s.index)))
}

func (*Source) GetInterfaceName() string {
	return dbusInterface + ".Source"
}

//TODO bug55140 只针对panguv，通过读取gsetting配置里保存的音量值与pulseaudio获取的音量值做比较，设置对应source->port的音量
func compareSourceVolumeByGSetting(s *Source, sourceInfo *pulse.Source, sourceVolume *float64) {
	if s.audio.isPanguV {
		if sourceInfo.ActivePort.Name == "" {
			sname := sourceInfo.Name
			if sname == AudioSourceName3 || sname == AudioSourceName4 {
				setvol := s.audio.settings.GetInt("onboard-input-volume")
				*sourceVolume = floatPrecision(float64(setvol) / 100.0)
				logger.Errorf("Failed: Get Source's activePort is Null & return onboard-input-volume: %v", *sourceVolume)
			} else {
				logger.Errorf("Failed: Get Source's activePort is Null & return")
			}
			return
		}

		var setVolome int32
		var sourceVolume_tmp float64

		//闭包函数,从audio GSetting获取音量
		var getHeadsetVolumeFromGSetting = func (selectPort int) int32 {
			var v int
			vl := s.audio.settings.GetStrv("headset-volume")
			if selectPort == 1 {
				v, _ = strconv.Atoi(vl[0])
			} else if selectPort == 2 {
				v, _ = strconv.Atoi(vl[1])
			} else if selectPort == 3 {
				v, _ = strconv.Atoi(vl[2])
			}
			return int32(v)
		}

		switch sourceInfo.Name {
		case AudioSourceName1:
			//存在虚拟3a_source情况下， 只更新内置音频声卡source的音量即可(华为音频算法服务会同步音量到虚拟source上)
			return
		case AudioSourceName2:
			//根据当前激活的port到audio gsetting里取headset-volume/headphone-volume键值数组对应index的值
			if sourceInfo.ActivePort.Available != 1 {
				if sourceInfo.ActivePort.Name == "analog-input-rear-mic-huawei" {
					setVolome = getHeadsetVolumeFromGSetting(1)
				} else if sourceInfo.ActivePort.Name == "analog-input-headset-mic-huawei" {
					setVolome = getHeadsetVolumeFromGSetting(2)
				} else if sourceInfo.ActivePort.Name == "analog-input-linein-huawei" {
					setVolome = getHeadsetVolumeFromGSetting(3)
				}
			} else {
				setVolome = s.audio.settings.GetInt("onboard-input-volume")
			}
		default:
			return
		}

		sourceVolume_tmp = floatPrecision(float64(setVolome) / 100.0)
		if *sourceVolume != sourceVolume_tmp {
			*sourceVolume = sourceVolume_tmp
			if sourceInfo.ActivePort.Available != 1 {
				cv := sourceInfo.Volume.SetAvg(sourceVolume_tmp)
				s.audio.context().SetSourceVolumeByIndex(sourceInfo.Index, cv)
				logger.Debugf("Update Source->ActivePort :%s need change volume: %v -> %v", sourceInfo.ActivePort.Name, *sourceVolume, sourceVolume_tmp)
			} else {
				logger.Debugf("Update Source->ActivePort :%s but is UnAvailable(Not Update source volume)", sourceInfo.ActivePort.Name)
			}
		}
	}
}

func (s *Source) update(sourceInfo *pulse.Source) {
	s.PropsMu.Lock()

	s.cVolume = sourceInfo.Volume
	s.channelMap = sourceInfo.ChannelMap
	s.Name = sourceInfo.Name
	s.Description = sourceInfo.Description
	s.Card = sourceInfo.Card
	s.BaseVolume = sourceInfo.BaseVolume.ToPercent()

	var source_volume float64
	source_volume = floatPrecision(sourceInfo.Volume.Avg())
	logger.Debugf("Update Source name :%s [ActivePort:%v,volume:%v]", s.Name, sourceInfo.ActivePort, source_volume)

	//TODO bug55140
	compareSourceVolumeByGSetting(s, sourceInfo, &source_volume)

	s.setPropMute(sourceInfo.Mute)
	s.setPropVolume(source_volume)

	//TODO: handle this
	s.setPropSupportFade(false)
	s.setPropFade(sourceInfo.Volume.Fade(sourceInfo.ChannelMap))
	s.setPropSupportBalance(true)
	s.setPropBalance(sourceInfo.Volume.Balance(sourceInfo.ChannelMap))

	s.setPropActivePort(toPort(sourceInfo.ActivePort))

	var ports []Port
	for _, p := range sourceInfo.Ports {
		ports = append(ports, toPort(p))
	}
	s.setPropPorts(ports)

	s.PropsMu.Unlock()
}
