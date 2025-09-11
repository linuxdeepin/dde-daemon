// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"encoding/json"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	soundthemeplayer "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.soundthemeplayer1"
)

const (
	gsKeyAudioVolumeChange       = "audio-volume-change"
	gsKeyCameraShutter           = "camera-shutter"
	gsKeyCompleteCopy            = "complete-copy"
	gsKeyCompletePrint           = "complete-print"
	gsKeyDesktopLogin            = "desktop-login"
	gsKeyDesktopLogout           = "desktop-logout"
	gsKeyDeviceAdded             = "device-added"
	gsKeyDeviceRemoved           = "device-removed"
	gsKeyDialogErrorCritical     = "dialog-error-critical"
	gsKeyDialogError             = "dialog-error"
	gsKeyDialogErrorSerious      = "dialog-error-serious"
	gsKeyMessage                 = "message"
	gsKeyPowerPlug               = "power-plug"
	gsKeyPowerUnplug             = "power-unplug"
	gsKeyPowerUnplugBatteryLow   = "power-unplug-battery-low"
	gsKeyScreenCaptureComplete   = "screen-capture-complete"
	gsKeyScreenCapture           = "screen-capture"
	gsKeySuspendResume           = "suspend-resume"
	gsKeySystemShutdown          = "system-shutdown"
	gsKeyTrashEmpty              = "trash-empty"
	gsKeyXDeepinAppSentToDesktop = "x-deepin-app-sent-to-desktop"
)

type syncSoundEffect struct {
	Enabled                 bool `json:"enabled"`
	AudioVolumeChange       bool `json:"audio_volume_change"`
	CameraShutter           bool `json:"camera_shutter"`
	CompleteCopy            bool `json:"complete_copy"`
	CompletePrint           bool `json:"complete_print"`
	DesktopLogin            bool `json:"desktop_login"`
	DesktopLogout           bool `json:"desktop_logout"`
	DeviceAdded             bool `json:"device_added"`
	DeviceRemoved           bool `json:"device_removed"`
	DialogErrorCritical     bool `json:"dialog_error_critical"`
	DialogError             bool `json:"dialog_error"`
	DialogErrorSerious      bool `json:"dialog_error_serious"`
	Message                 bool `json:"message"`
	PowerPlug               bool `json:"power_plug"`
	PowerUnplug             bool `json:"power_unplug"`
	PowerUnplugBatteryLow   bool `json:"power_unplug_battery_low"`
	ScreenCaptureComplete   bool `json:"screen_capture_complete"`
	ScreenCapture           bool `json:"screen_capture"`
	SuspendResume           bool `json:"suspend_resume"`
	SystemShutdown          bool `json:"system_shutdown"`
	TrashEmpty              bool `json:"trash_empty"`
	XDeepinAppSentToDesktop bool `json:"x_deepin_app_sent_to_desktop"`
}

type syncData struct {
	Version     string           `json:"version"`
	SoundEffect *syncSoundEffect `json:"soundeffect"`
}

type syncConfig struct {
	a *Audio
}

const (
	syncVersion = "1.0"
)

func (sc *syncConfig) Get() (interface{}, error) {
	s, err := dconfig.NewDConfig(dconfigDaemonAppId, dconfigSoundEffectId, "")
	if err != nil {
		logger.Warning(err)
	}

	getConfigBoolOrDefault := func(key string, defaultValue bool) bool {
		value, err := s.GetValueBool(key)
		if err != nil {
			// 记录日志或处理错误
			logger.Warning("Failed to get boolean for key %s: %v, using default %v", key, err, defaultValue)
			return defaultValue
		}
		return value
	}

	return &syncData{
		Version: syncVersion,
		SoundEffect: &syncSoundEffect{
			Enabled:                 getConfigBoolOrDefault(gsKeyEnabled, true),
			AudioVolumeChange:       getConfigBoolOrDefault(gsKeyAudioVolumeChange, true),
			CameraShutter:           getConfigBoolOrDefault(gsKeyCameraShutter, true),
			CompleteCopy:            getConfigBoolOrDefault(gsKeyCompleteCopy, true),
			CompletePrint:           getConfigBoolOrDefault(gsKeyCompletePrint, true),
			DesktopLogin:            getConfigBoolOrDefault(gsKeyDesktopLogin, true),
			DesktopLogout:           getConfigBoolOrDefault(gsKeyDesktopLogout, true),
			DeviceAdded:             getConfigBoolOrDefault(gsKeyDeviceAdded, true),
			DeviceRemoved:           getConfigBoolOrDefault(gsKeyDeviceRemoved, true),
			DialogErrorCritical:     getConfigBoolOrDefault(gsKeyDialogErrorCritical, true),
			DialogError:             getConfigBoolOrDefault(gsKeyDialogError, true),
			DialogErrorSerious:      getConfigBoolOrDefault(gsKeyDialogErrorSerious, true),
			Message:                 getConfigBoolOrDefault(gsKeyMessage, true),
			PowerPlug:               getConfigBoolOrDefault(gsKeyPowerPlug, true),
			PowerUnplug:             getConfigBoolOrDefault(gsKeyPowerUnplug, true),
			PowerUnplugBatteryLow:   getConfigBoolOrDefault(gsKeyPowerUnplugBatteryLow, true),
			ScreenCaptureComplete:   getConfigBoolOrDefault(gsKeyScreenCaptureComplete, true),
			ScreenCapture:           getConfigBoolOrDefault(gsKeyScreenCapture, true),
			SuspendResume:           getConfigBoolOrDefault(gsKeySuspendResume, true),
			SystemShutdown:          getConfigBoolOrDefault(gsKeySystemShutdown, true),
			TrashEmpty:              getConfigBoolOrDefault(gsKeyTrashEmpty, true),
			XDeepinAppSentToDesktop: getConfigBoolOrDefault(gsKeyXDeepinAppSentToDesktop, true),
		},
	}, nil
}

func (sc *syncConfig) Set(data []byte) error {
	var info syncData
	err := json.Unmarshal(data, &info)
	if err != nil {
		return err
	}
	soundEffect := info.SoundEffect
	if soundEffect != nil {
		s, err := dconfig.NewDConfig(dconfigDaemonAppId, dconfigSoundEffectId, "")
		if err != nil {
			return err
		}
		s.SetValue(gsKeyEnabled, soundEffect.Enabled)
		s.SetValue(gsKeyAudioVolumeChange, soundEffect.AudioVolumeChange)
		s.SetValue(gsKeyCameraShutter, soundEffect.CameraShutter)
		s.SetValue(gsKeyCompleteCopy, soundEffect.CompleteCopy)
		s.SetValue(gsKeyCompletePrint, soundEffect.CompletePrint)
		s.SetValue(gsKeyDesktopLogin, soundEffect.DesktopLogin)
		err = sc.syncConfigToSoundThemePlayer(soundEffect.DesktopLogin)
		if err != nil {
			logger.Warning(err)
		}
		s.SetValue(gsKeyDesktopLogout, soundEffect.DesktopLogout)
		s.SetValue(gsKeyDeviceAdded, soundEffect.DeviceAdded)
		s.SetValue(gsKeyDeviceRemoved, soundEffect.DeviceRemoved)
		s.SetValue(gsKeyDialogErrorCritical, soundEffect.DialogErrorCritical)
		s.SetValue(gsKeyDialogError, soundEffect.DialogError)
		s.SetValue(gsKeyDialogErrorSerious, soundEffect.DialogErrorSerious)
		s.SetValue(gsKeyMessage, soundEffect.Message)
		s.SetValue(gsKeyPowerPlug, soundEffect.PowerPlug)
		s.SetValue(gsKeyPowerUnplug, soundEffect.PowerUnplug)
		s.SetValue(gsKeyPowerUnplugBatteryLow, soundEffect.PowerUnplugBatteryLow)
		s.SetValue(gsKeyScreenCaptureComplete, soundEffect.ScreenCaptureComplete)
		s.SetValue(gsKeyScreenCapture, soundEffect.ScreenCapture)
		s.SetValue(gsKeySuspendResume, soundEffect.SuspendResume)
		s.SetValue(gsKeySystemShutdown, soundEffect.SystemShutdown)
		s.SetValue(gsKeyTrashEmpty, soundEffect.TrashEmpty)
		s.SetValue(gsKeyXDeepinAppSentToDesktop, soundEffect.XDeepinAppSentToDesktop)
	}
	return nil
}

func (sc *syncConfig) syncConfigToSoundThemePlayer(enabled bool) error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	player := soundthemeplayer.NewSoundThemePlayer(sysBus)
	return player.EnableSoundDesktopLogin(0, enabled)
}
