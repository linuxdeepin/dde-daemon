/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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

package shortcuts

import (
	"github.com/godbus/dbus"
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
	cmdCalculator = "deepin-calculator"
	cmdCalendar   = "dde-calendar"
	cmdMeeting    = "deepin-contacts"
	cmdTerminal   = "/usr/lib/deepin-daemon/default-terminal"
	cmdMessenger  = "dbus-send --print-reply --dest=com.deepin.dde.osd /com/deepin/dde/Notification com.deepin.dde.Notification.Toggle"
	cmdLauncher   = "dbus-send --print-reply --dest=com.deepin.dde.Launcher /com/deepin/dde/Launcher com.deepin.dde.Launcher.Toggle"
	cmdCamera     = "deepin-camera"
)

var mediaIdActionMap = map[string]*Action{
	"numlock":  &Action{Type: ActionTypeShowNumLockOSD},
	"capslock": &Action{Type: ActionTypeShowCapsLockOSD},
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
	"calculator":  NewExecCmdAction(cmdCalculator, false),
	"calendar":    NewExecCmdAction(cmdCalendar, false),
	"meeting":     NewExecCmdAction(cmdMeeting, false),
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
	"suspend": &Action{Type: ActionTypeSystemSuspend},
	"sleep":   &Action{Type: ActionTypeSystemSuspend},
	"log-off": &Action{Type: ActionTypeSystemLogOff},
	"away":    &Action{Type: ActionTypeSystemAway},

	"web-cam": NewExecCmdAction(cmdCamera, false),

	// We do not need to deal with XF86Wlan key default,
	// but can be specially by 'EnableNetworkController'
	"wlan":  &Action{Type: ActionTypeToggleWireless},
	"tools": &Action{Type: ActionTypeShowControlCenter},
}

func showOSD(signal string) {
	logger.Debug("show OSD", signal)
	sessionDBus, _ := dbus.SessionBus()
	go sessionDBus.Object("com.deepin.dde.osd", "/").Call("com.deepin.dde.osd.ShowOSD", 0, signal)
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
