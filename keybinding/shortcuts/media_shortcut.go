// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package shortcuts

import (
	"github.com/godbus/dbus/v5"
)

type MediaShortcut struct {
	*GSettingsShortcut
}

const (
	mimeTypeBrowser    = "x-scheme-handler/http"
	mimeTypeMail       = "x-scheme-handler/mailto"
	mimeTypeAudioMedia = "audio/mpeg"
	mimeTypeVideoMp4   = "video/mp4"
	mimeTypeDir        = "inode/directory"
	mimeTypeImagePng   = "image/png"
)

const (
	cmdMyComputer = "gio open computer:///"
	cmdDocuments  = "gio open ~/Documents"
	cmdEject      = "eject -r"
	cmdTerminal   = "/usr/lib/deepin-daemon/default-terminal"
	cmdMessenger  = "dbus-send --print-reply --dest=org.deepin.dde.Widgets1 /org/deepin/dde/Widgets1 org.deepin.dde.Widgets1.Toggle"
	cmdLauncher   = "dbus-send --print-reply --dest=org.deepin.dde.Launcher1 /org/deepin/dde/Launcher1 org.deepin.dde.Launcher1.Toggle"
)

const (
	desktopCalculator = "deepin-calculator.desktop"
	desktopCalendar   = "dde-calendar.desktop"
	desktopMeeting    = "deepin-contacts.desktop"
	desktopCamera     = "deepin-camera.desktop"
)

var mediaIdActionMap = map[string]*Action{
	"numlock":  {Type: ActionTypeShowNumLockOSD},
	"capslock": {Type: ActionTypeShowCapsLockOSD},
	// Open MimeType
	"home-page":   NewOpenMimeTypeAction(mimeTypeBrowser),
	"www":         NewOpenMimeTypeAction(mimeTypeBrowser),
	"explorer":    NewOpenMimeTypeAction(mimeTypeDir),
	"mail":        NewOpenMimeTypeAction(mimeTypeMail),
	"audio-media": NewOpenMimeTypeAction(mimeTypeAudioMedia),
	"music":       NewOpenMimeTypeAction(mimeTypeAudioMedia),
	"pictures":    NewOpenMimeTypeAction(mimeTypeImagePng),
	"video":       NewOpenMimeTypeAction(mimeTypeVideoMp4),

	// command
	"my-computer": NewExecCmdAction(cmdMyComputer, false),
	"documents":   NewExecCmdAction(cmdDocuments, false),
	"eject":       NewExecCmdAction(cmdEject, false),
	"calculator":  NewDesktopTypeAction(desktopCalculator),
	"calendar":    NewDesktopTypeAction(desktopCalendar),
	"meeting":     NewDesktopTypeAction(desktopMeeting),
	"terminal":    NewExecCmdAction(cmdTerminal, false),
	"messenger":   NewExecCmdAction(cmdMessenger, false),
	"app-left":    NewExecCmdAction(cmdLauncher, false),
	"app-right":   NewExecCmdAction(cmdLauncher, false),

	// audio control
	"audio-mute":         NewAudioCtrlAction(AudioSinkMuteToggle),
	"audio-raise-volume": NewAudioCtrlAction(AudioSinkVolumeUp),
	"audio-lower-volume": NewAudioCtrlAction(AudioSinkVolumeDown),
	"audio-mic-mute":     NewAudioCtrlAction(AudioSourceMuteToggle),

	// media player control
	"audio-play":    NewMediaPlayerCtrlAction(MediaPlayerPlay),
	"audio-pause":   NewMediaPlayerCtrlAction(MediaPlayerPause),
	"audio-stop":    NewMediaPlayerCtrlAction(MediaPlayerStop),
	"audio-forward": NewMediaPlayerCtrlAction(MediaPlayerForword),
	"audio-rewind":  NewMediaPlayerCtrlAction(MediaPlayerRewind),
	"audio-prev":    NewMediaPlayerCtrlAction(MediaPlayerPrevious),
	"audio-next":    NewMediaPlayerCtrlAction(MediaPlayerNext),
	"audio-repeat":  NewMediaPlayerCtrlAction(MediaPlayerRepeat),
	// TODO audio-random-play audio-cycle-track

	// display control
	"mon-brightness-up":   NewDisplayCtrlAction(MonitorBrightnessUp),
	"mon-brightness-down": NewDisplayCtrlAction(MonitorBrightnessDown),
	"switch-monitors":     NewDisplayCtrlAction(DisplayModeSwitch),
	"display":             NewDisplayCtrlAction(DisplayModeSwitch),
	"adjust-brightness":   NewDisplayCtrlAction(AdjustBrightnessSwitch),

	// kbd light control
	"kbd-light-on-off":    NewKbdBrightnessCtrlAction(KbdLightToggle),
	"kbd-brightness-up":   NewKbdBrightnessCtrlAction(KbdLightBrightnessUp),
	"kbd-brightness-down": NewKbdBrightnessCtrlAction(KbdLightBrightnessDown),

	// touchpad
	"touchpad-toggle": NewTouchpadCtrlAction(TouchpadToggle),
	"touchpad-on":     NewTouchpadCtrlAction(TouchpadOn),
	"touchpad-off":    NewTouchpadCtrlAction(TouchpadOff),

	// power
	"suspend": {Type: ActionTypeSystemSuspend},
	"sleep":   {Type: ActionTypeSystemSuspend},
	"log-off": {Type: ActionTypeSystemLogOff},
	"away":    {Type: ActionTypeSystemAway},

	"web-cam": NewDesktopTypeAction(desktopCamera),

	// We do not need to deal with XF86Wlan key default,
	// but can be specially by 'EnableNetworkController'
	"wlan":  {Type: ActionTypeToggleWireless},
	"tools": {Type: ActionTypeShowControlCenter},
}

func showOSD(signal string) {
	logger.Debug("show OSD", signal)
	sessionDBus, _ := dbus.SessionBus()
	go sessionDBus.Object("org.deepin.dde.Osd1", "/").Call("org.deepin.dde.Osd1.ShowOSD", 0, signal)
}

func (ms *MediaShortcut) GetAction() *Action {
	logger.Debug("MediaShortcut.GetAction", ms.Id)
	if action, ok := mediaIdActionMap[ms.Id]; ok {
		return action
	}
	return ActionNoOp
}

func GetAction(id string) *Action {
	if action, ok := mediaIdActionMap[id]; ok {
		return action
	}

	return ActionNoOp
}
