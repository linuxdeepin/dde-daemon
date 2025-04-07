// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package shortcuts

import (
	"github.com/godbus/dbus/v5"
)

type MediaShortcut struct {
	*ShortcutObject
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
	cmdCalculator = "dde-am deepin-calculator"
	cmdCalendar   = "dde-am dde-calendar"
	cmdMeeting    = "deepin-contacts"
	cmdTerminal   = "/usr/lib/deepin-daemon/default-terminal"
	cmdMessenger  = "dbus-send --print-reply --dest=org.deepin.dde.Osd1 /org/deepin/dde/Notification1 org.deepin.dde.Notification1.Toggle"
	cmdLauncher   = "dbus-send --print-reply --dest=org.deepin.dde.Launcher1 /org/deepin/dde/Launcher1 org.deepin.dde.Launcher1.Toggle"
	cmdCamera     = "/usr/share/dde-daemon/keybinding/cameraSwitch.sh"
)

var mediaIdActionMap = map[string]*Action{
	"numlock":  &Action{Type: ActionTypeShowNumLockOSD},
	"capslock": &Action{Type: ActionTypeShowCapsLockOSD},
	// Open MimeType
	"homePage":   NewOpenMimeTypeAction(mimeTypeBrowser),
	"www":        NewOpenMimeTypeAction(mimeTypeBrowser),
	"explorer":   NewOpenMimeTypeAction(mimeTypeDir),
	"mail":       NewOpenMimeTypeAction(mimeTypeMail),
	"audioMedia": NewOpenMimeTypeAction(mimeTypeAudioMedia),
	"music":      NewOpenMimeTypeAction(mimeTypeAudioMedia),
	"pictures":   NewOpenMimeTypeAction(mimeTypeImagePng),
	"video":      NewOpenMimeTypeAction(mimeTypeVideoMp4),

	// command
	"myComputer": NewExecCmdAction(cmdMyComputer, false),
	"documents":  NewExecCmdAction(cmdDocuments, false),
	"eject":      NewExecCmdAction(cmdEject, false),
	"calculator": NewExecCmdAction(cmdCalculator, false),
	"calendar":   NewExecCmdAction(cmdCalendar, false),
	"meeting":    NewExecCmdAction(cmdMeeting, false),
	"terminal":   NewExecCmdAction(cmdTerminal, false),
	"messenger":  NewExecCmdAction(cmdMessenger, false),
	"appLeft":    NewExecCmdAction(cmdLauncher, false),
	"appRight":   NewExecCmdAction(cmdLauncher, false),

	// audio control
	"audioMute":        NewAudioCtrlAction(AudioSinkMuteToggle),
	"audioRaiseVolume": NewAudioCtrlAction(AudioSinkVolumeUp),
	"audioLowerVolume": NewAudioCtrlAction(AudioSinkVolumeDown),
	"audioMicMute":     NewAudioCtrlAction(AudioSourceMuteToggle),

	// media player control
	"audioPlay":    NewMediaPlayerCtrlAction(MediaPlayerPlay),
	"audioPause":   NewMediaPlayerCtrlAction(MediaPlayerPause),
	"audioStop":    NewMediaPlayerCtrlAction(MediaPlayerStop),
	"audioForward": NewMediaPlayerCtrlAction(MediaPlayerForword),
	"audioRewind":  NewMediaPlayerCtrlAction(MediaPlayerRewind),
	"audioPrev":    NewMediaPlayerCtrlAction(MediaPlayerPrevious),
	"audioNext":    NewMediaPlayerCtrlAction(MediaPlayerNext),
	"audioRepeat":  NewMediaPlayerCtrlAction(MediaPlayerRepeat),
	// TODO audio-random-play audio-cycle-track

	// display control
	"monBrightnessUp":   NewDisplayCtrlAction(MonitorBrightnessUp),
	"monBrightnessDown": NewDisplayCtrlAction(MonitorBrightnessDown),
	"display":           NewDisplayCtrlAction(DisplayModeSwitch),
	"adjustBrightness":  NewDisplayCtrlAction(AdjustBrightnessSwitch),

	// kbd light control
	"kbdLightOnOff":     NewKbdBrightnessCtrlAction(KbdLightToggle),
	"kbdBrightnessUp":   NewKbdBrightnessCtrlAction(KbdLightBrightnessUp),
	"kbdBrightnessDown": NewKbdBrightnessCtrlAction(KbdLightBrightnessDown),

	// touchpad
	"touchpadToggle": NewTouchpadCtrlAction(TouchpadToggle),
	"touchpadOn":     NewTouchpadCtrlAction(TouchpadOn),
	"touchpadOff":    NewTouchpadCtrlAction(TouchpadOff),

	// power
	"suspend": &Action{Type: ActionTypeSystemSuspend},
	"sleep":   &Action{Type: ActionTypeSystemSuspend},
	"logOff":  &Action{Type: ActionTypeSystemLogOff},
	"away":    &Action{Type: ActionTypeSystemAway},

	"webCam": NewExecCmdAction(cmdCamera, false),

	// We do not need to deal with XF86Wlan key default,
	// but can be specially by 'EnableNetworkController'
	"wlan":  &Action{Type: ActionTypeToggleWireless},
	"tools": &Action{Type: ActionTypeShowControlCenter},
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
