// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"fmt"
	"time"

	dbus "github.com/godbus/dbus/v5"
	soundthemeplayer "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.soundthemeplayer1"
	"github.com/linuxdeepin/go-lib/asound"
)

func (a *Audio) saveConfig() {
	logger.Debug("saveConfig")
	a.saverLocker.Lock()
	if a.isSaving {
		a.saverLocker.Unlock()
		return
	}

	a.isSaving = true
	a.saverLocker.Unlock()

	time.AfterFunc(time.Second*1, func() {
		a.doSaveConfig()

		a.saverLocker.Lock()
		a.isSaving = false
		a.saverLocker.Unlock()
	})
}

func (a *Audio) doSaveConfig() {
	ctx := a.context()
	if ctx == nil {
		logger.Warning("failed to save config, ctx is nil")
		return
	}
	err := a.saveAudioState()
	if err != nil {
		logger.Warning(err)
	}

}

func (a *Audio) saveAudioState() error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	sink := a.getDefaultSink()
	if sink == nil {
		return fmt.Errorf("not found default sink")
	}
	sink.PropsMu.RLock()
	device := sink.props["alsa.device"]
	card := sink.props["alsa.card"]
	mute := sink.Mute
	volume := sink.Volume * 100.0
	sink.PropsMu.RUnlock()

	cardId, err := toALSACardId(card)
	if err != nil {
		return err
	}

	activePlayback := map[string]dbus.Variant{
		"card":   dbus.MakeVariant(cardId),
		"device": dbus.MakeVariant(device),
		"mute":   dbus.MakeVariant(mute),
		"volume": dbus.MakeVariant(volume),
	}

	player := soundthemeplayer.NewSoundThemePlayer(sysBus)
	err = player.SaveAudioState(0, activePlayback)
	return err
}

func toALSACardId(idx string) (cardId string, err error) {
	ctl, err := asound.CTLOpen("hw:"+idx, 0)
	if err != nil {
		return
	}
	defer ctl.Close()

	cardInfo, err := asound.NewCTLCardInfo()
	if err != nil {
		return
	}
	defer cardInfo.Free()

	err = ctl.CardInfo(cardInfo)
	if err != nil {
		return
	}

	cardId = cardInfo.GetID()
	return
}
