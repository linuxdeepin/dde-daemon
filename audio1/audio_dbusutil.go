// Code generated by "dbusutil-gen -type Audio,Sink,SinkInput,Source,Meter -import github.com/godbus/dbus audio.go sink.go sinkinput.go source.go meter.go"; DO NOT EDIT.

package audio

import (
	"github.com/godbus/dbus/v5"
)

func (v *Audio) setPropSinkInputs(value []dbus.ObjectPath) (changed bool) {
	if !objectPathSliceEqual(v.SinkInputs, value) {
		v.SinkInputs = value
		v.emitPropChangedSinkInputs(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedSinkInputs(value []dbus.ObjectPath) error {
	return v.service.EmitPropertyChanged(v, "SinkInputs", value)
}

func (v *Audio) setPropSinks(value []dbus.ObjectPath) (changed bool) {
	if !objectPathSliceEqual(v.Sinks, value) {
		v.Sinks = value
		v.emitPropChangedSinks(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedSinks(value []dbus.ObjectPath) error {
	return v.service.EmitPropertyChanged(v, "Sinks", value)
}

func (v *Audio) setPropSources(value []dbus.ObjectPath) (changed bool) {
	if !objectPathSliceEqual(v.Sources, value) {
		v.Sources = value
		v.emitPropChangedSources(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedSources(value []dbus.ObjectPath) error {
	return v.service.EmitPropertyChanged(v, "Sources", value)
}

func (v *Audio) setPropDefaultSink(value dbus.ObjectPath) (changed bool) {
	if v.DefaultSink != value {
		v.DefaultSink = value
		v.emitPropChangedDefaultSink(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedDefaultSink(value dbus.ObjectPath) error {
	return v.service.EmitPropertyChanged(v, "DefaultSink", value)
}

func (v *Audio) setPropDefaultSource(value dbus.ObjectPath) (changed bool) {
	if v.DefaultSource != value {
		v.DefaultSource = value
		v.emitPropChangedDefaultSource(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedDefaultSource(value dbus.ObjectPath) error {
	return v.service.EmitPropertyChanged(v, "DefaultSource", value)
}

func (v *Audio) setPropCards(value string) (changed bool) {
	if v.Cards != value {
		v.Cards = value
		v.emitPropChangedCards(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedCards(value string) error {
	return v.service.EmitPropertyChanged(v, "Cards", value)
}

func (v *Audio) setPropCardsWithoutUnavailable(value string) (changed bool) {
	if v.CardsWithoutUnavailable != value {
		v.CardsWithoutUnavailable = value
		v.emitPropChangedCardsWithoutUnavailable(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedCardsWithoutUnavailable(value string) error {
	return v.service.EmitPropertyChanged(v, "CardsWithoutUnavailable", value)
}

func (v *Audio) setPropBluetoothAudioMode(value string) (changed bool) {
	if v.BluetoothAudioMode != value {
		v.BluetoothAudioMode = value
		v.emitPropChangedBluetoothAudioMode(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedBluetoothAudioMode(value string) error {
	return v.service.EmitPropertyChanged(v, "BluetoothAudioMode", value)
}

func (v *Audio) setPropBluetoothAudioModeOpts(value []string) (changed bool) {
	if !isStrvEqual(v.BluetoothAudioModeOpts, value) {
		v.BluetoothAudioModeOpts = value
		v.emitPropChangedBluetoothAudioModeOpts(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedBluetoothAudioModeOpts(value []string) error {
	return v.service.EmitPropertyChanged(v, "BluetoothAudioModeOpts", value)
}

func (v *Audio) setPropPausePlayer(value bool) (changed bool) {
	if v.PausePlayer != value {
		v.PausePlayer = value
		v.emitPropChangedPausePlayer(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedPausePlayer(value bool) error {
	return v.service.EmitPropertyChanged(v, "PausePlayer", value)
}

func (v *Audio) setPropReduceNoise(value bool) (changed bool) {
	if v.ReduceNoise != value {
		v.ReduceNoise = value
		v.emitPropChangedReduceNoise(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedReduceNoise(value bool) error {
	return v.service.EmitPropertyChanged(v, "ReduceNoise", value)
}

func (v *Audio) setPropMaxUIVolume(value float64) (changed bool) {
	if v.MaxUIVolume != value {
		v.MaxUIVolume = value
		v.emitPropChangedMaxUIVolume(value)
		return true
	}
	return false
}

func (v *Audio) emitPropChangedMaxUIVolume(value float64) error {
	return v.service.EmitPropertyChanged(v, "MaxUIVolume", value)
}

func (v *Sink) setPropName(value string) (changed bool) {
	if v.Name != value {
		v.Name = value
		v.emitPropChangedName(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedName(value string) error {
	return v.service.EmitPropertyChanged(v, "Name", value)
}

func (v *Sink) setPropDescription(value string) (changed bool) {
	if v.Description != value {
		v.Description = value
		v.emitPropChangedDescription(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedDescription(value string) error {
	return v.service.EmitPropertyChanged(v, "Description", value)
}

func (v *Sink) setPropBaseVolume(value float64) (changed bool) {
	if v.BaseVolume != value {
		v.BaseVolume = value
		v.emitPropChangedBaseVolume(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedBaseVolume(value float64) error {
	return v.service.EmitPropertyChanged(v, "BaseVolume", value)
}

func (v *Sink) setPropMute(value bool) (changed bool) {
	if v.Mute != value {
		v.Mute = value
		v.emitPropChangedMute(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedMute(value bool) error {
	return v.service.EmitPropertyChanged(v, "Mute", value)
}

func (v *Sink) setPropVolume(value float64) (changed bool) {
	if v.Volume != value {
		v.Volume = value
		v.emitPropChangedVolume(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedVolume(value float64) error {
	return v.service.EmitPropertyChanged(v, "Volume", value)
}

func (v *Sink) setPropBalance(value float64) (changed bool) {
	if v.Balance != value {
		v.Balance = value
		v.emitPropChangedBalance(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedBalance(value float64) error {
	return v.service.EmitPropertyChanged(v, "Balance", value)
}

func (v *Sink) setPropSupportBalance(value bool) (changed bool) {
	if v.SupportBalance != value {
		v.SupportBalance = value
		v.emitPropChangedSupportBalance(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedSupportBalance(value bool) error {
	return v.service.EmitPropertyChanged(v, "SupportBalance", value)
}

func (v *Sink) setPropFade(value float64) (changed bool) {
	if v.Fade != value {
		v.Fade = value
		v.emitPropChangedFade(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedFade(value float64) error {
	return v.service.EmitPropertyChanged(v, "Fade", value)
}

func (v *Sink) setPropSupportFade(value bool) (changed bool) {
	if v.SupportFade != value {
		v.SupportFade = value
		v.emitPropChangedSupportFade(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedSupportFade(value bool) error {
	return v.service.EmitPropertyChanged(v, "SupportFade", value)
}

func (v *Sink) setPropPorts(value []Port) (changed bool) {
	if !portsEqual(v.Ports, value) {
		v.Ports = value
		v.emitPropChangedPorts(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedPorts(value []Port) error {
	return v.service.EmitPropertyChanged(v, "Ports", value)
}

func (v *Sink) setPropActivePort(value Port) (changed bool) {
	if v.ActivePort != value {
		v.ActivePort = value
		v.emitPropChangedActivePort(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedActivePort(value Port) error {
	return v.service.EmitPropertyChanged(v, "ActivePort", value)
}

func (v *Sink) setPropCard(value uint32) (changed bool) {
	if v.Card != value {
		v.Card = value
		v.emitPropChangedCard(value)
		return true
	}
	return false
}

func (v *Sink) emitPropChangedCard(value uint32) error {
	return v.service.EmitPropertyChanged(v, "Card", value)
}

func (v *SinkInput) setPropName(value string) (changed bool) {
	if v.Name != value {
		v.Name = value
		v.emitPropChangedName(value)
		return true
	}
	return false
}

func (v *SinkInput) emitPropChangedName(value string) error {
	return v.service.EmitPropertyChanged(v, "Name", value)
}

func (v *SinkInput) setPropIcon(value string) (changed bool) {
	if v.Icon != value {
		v.Icon = value
		v.emitPropChangedIcon(value)
		return true
	}
	return false
}

func (v *SinkInput) emitPropChangedIcon(value string) error {
	return v.service.EmitPropertyChanged(v, "Icon", value)
}

func (v *SinkInput) setPropMute(value bool) (changed bool) {
	if v.Mute != value {
		v.Mute = value
		v.emitPropChangedMute(value)
		return true
	}
	return false
}

func (v *SinkInput) emitPropChangedMute(value bool) error {
	return v.service.EmitPropertyChanged(v, "Mute", value)
}

func (v *SinkInput) setPropVolume(value float64) (changed bool) {
	if v.Volume != value {
		v.Volume = value
		v.emitPropChangedVolume(value)
		return true
	}
	return false
}

func (v *SinkInput) emitPropChangedVolume(value float64) error {
	return v.service.EmitPropertyChanged(v, "Volume", value)
}

func (v *SinkInput) setPropBalance(value float64) (changed bool) {
	if v.Balance != value {
		v.Balance = value
		v.emitPropChangedBalance(value)
		return true
	}
	return false
}

func (v *SinkInput) emitPropChangedBalance(value float64) error {
	return v.service.EmitPropertyChanged(v, "Balance", value)
}

func (v *SinkInput) setPropSupportBalance(value bool) (changed bool) {
	if v.SupportBalance != value {
		v.SupportBalance = value
		v.emitPropChangedSupportBalance(value)
		return true
	}
	return false
}

func (v *SinkInput) emitPropChangedSupportBalance(value bool) error {
	return v.service.EmitPropertyChanged(v, "SupportBalance", value)
}

func (v *SinkInput) setPropFade(value float64) (changed bool) {
	if v.Fade != value {
		v.Fade = value
		v.emitPropChangedFade(value)
		return true
	}
	return false
}

func (v *SinkInput) emitPropChangedFade(value float64) error {
	return v.service.EmitPropertyChanged(v, "Fade", value)
}

func (v *SinkInput) setPropSupportFade(value bool) (changed bool) {
	if v.SupportFade != value {
		v.SupportFade = value
		v.emitPropChangedSupportFade(value)
		return true
	}
	return false
}

func (v *SinkInput) emitPropChangedSupportFade(value bool) error {
	return v.service.EmitPropertyChanged(v, "SupportFade", value)
}

func (v *SinkInput) setPropSinkIndex(value uint32) (changed bool) {
	if v.SinkIndex != value {
		v.SinkIndex = value
		v.emitPropChangedSinkIndex(value)
		return true
	}
	return false
}

func (v *SinkInput) emitPropChangedSinkIndex(value uint32) error {
	return v.service.EmitPropertyChanged(v, "SinkIndex", value)
}

func (v *Source) setPropName(value string) (changed bool) {
	if v.Name != value {
		v.Name = value
		v.emitPropChangedName(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedName(value string) error {
	return v.service.EmitPropertyChanged(v, "Name", value)
}

func (v *Source) setPropDescription(value string) (changed bool) {
	if v.Description != value {
		v.Description = value
		v.emitPropChangedDescription(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedDescription(value string) error {
	return v.service.EmitPropertyChanged(v, "Description", value)
}

func (v *Source) setPropBaseVolume(value float64) (changed bool) {
	if v.BaseVolume != value {
		v.BaseVolume = value
		v.emitPropChangedBaseVolume(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedBaseVolume(value float64) error {
	return v.service.EmitPropertyChanged(v, "BaseVolume", value)
}

func (v *Source) setPropMute(value bool) (changed bool) {
	if v.Mute != value {
		v.Mute = value
		v.emitPropChangedMute(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedMute(value bool) error {
	return v.service.EmitPropertyChanged(v, "Mute", value)
}

func (v *Source) setPropVolume(value float64) (changed bool) {
	if v.Volume != value {
		v.Volume = value
		v.emitPropChangedVolume(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedVolume(value float64) error {
	return v.service.EmitPropertyChanged(v, "Volume", value)
}

func (v *Source) setPropBalance(value float64) (changed bool) {
	if v.Balance != value {
		v.Balance = value
		v.emitPropChangedBalance(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedBalance(value float64) error {
	return v.service.EmitPropertyChanged(v, "Balance", value)
}

func (v *Source) setPropSupportBalance(value bool) (changed bool) {
	if v.SupportBalance != value {
		v.SupportBalance = value
		v.emitPropChangedSupportBalance(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedSupportBalance(value bool) error {
	return v.service.EmitPropertyChanged(v, "SupportBalance", value)
}

func (v *Source) setPropFade(value float64) (changed bool) {
	if v.Fade != value {
		v.Fade = value
		v.emitPropChangedFade(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedFade(value float64) error {
	return v.service.EmitPropertyChanged(v, "Fade", value)
}

func (v *Source) setPropSupportFade(value bool) (changed bool) {
	if v.SupportFade != value {
		v.SupportFade = value
		v.emitPropChangedSupportFade(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedSupportFade(value bool) error {
	return v.service.EmitPropertyChanged(v, "SupportFade", value)
}

func (v *Source) setPropPorts(value []Port) (changed bool) {
	if !portsEqual(v.Ports, value) {
		v.Ports = value
		v.emitPropChangedPorts(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedPorts(value []Port) error {
	return v.service.EmitPropertyChanged(v, "Ports", value)
}

func (v *Source) setPropActivePort(value Port) (changed bool) {
	if v.ActivePort != value {
		v.ActivePort = value
		v.emitPropChangedActivePort(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedActivePort(value Port) error {
	return v.service.EmitPropertyChanged(v, "ActivePort", value)
}

func (v *Source) setPropCard(value uint32) (changed bool) {
	if v.Card != value {
		v.Card = value
		v.emitPropChangedCard(value)
		return true
	}
	return false
}

func (v *Source) emitPropChangedCard(value uint32) error {
	return v.service.EmitPropertyChanged(v, "Card", value)
}

func (v *Meter) setPropVolume(value float64) (changed bool) {
	if v.Volume != value {
		v.Volume = value
		v.emitPropChangedVolume(value)
		return true
	}
	return false
}

func (v *Meter) emitPropChangedVolume(value float64) error {
	return v.service.EmitPropertyChanged(v, "Volume", value)
}
