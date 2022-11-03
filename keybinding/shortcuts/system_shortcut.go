// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package shortcuts

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"sync"

	dutils "github.com/linuxdeepin/go-lib/utils"
)

const (
	// Under '/usr/share' or '/usr/local/share'
	systemActionsFile   = "dde-daemon/keybinding/system_actions.json"
	screenshotCmdPrefix = "dbus-send --print-reply --dest=com.deepin.Screenshot /com/deepin/Screenshot com.deepin.Screenshot."
)

type SystemShortcut struct {
	*GSettingsShortcut
	arg *ActionExecCmdArg
}

func (ss *SystemShortcut) SetName(name string) error {
	return ErrOpNotSupported
}

func (ss *SystemShortcut) GetAction() *Action {
	return &Action{
		Type: ActionTypeExecCmd,
		Arg:  ss.arg,
	}
}

func (ss *SystemShortcut) SetAction(newAction *Action) error {
	if newAction == nil {
		return ErrNilAction
	}
	if newAction.Type != ActionTypeExecCmd {
		return ErrInvalidActionType
	}

	arg, ok := newAction.Arg.(*ActionExecCmdArg)
	if !ok {
		return ErrTypeAssertionFail
	}
	ss.arg = arg
	return nil
}

var loadSysActionsFileOnce sync.Once
var actionsCache *actionHandler

func GetSystemActionCmd(id string) string {
	return getSystemActionCmd(id)
}

func getSystemActionCmd(id string) string {
	loadSysActionsFileOnce.Do(func() {
		file := getSystemActionsFile()
		actions, err := loadSystemActionsFile(file)
		if err != nil {
			logger.Warning("failed to load system actions file:", err)
			return
		}
		actionsCache = actions
	})

	if actionsCache != nil {
		if cmd, ok := actionsCache.getCmd(id); ok {
			return cmd
		}
	}
	return defaultSysActionCmdMap[id]
}

// key is id, value is commandline.
var defaultSysActionCmdMap = map[string]string{
	"launcher":               "dbus-send --print-reply --dest=com.deepin.dde.Launcher /com/deepin/dde/Launcher com.deepin.dde.Launcher.Toggle",
	"terminal":               "/usr/lib/deepin-daemon/default-terminal",
	"terminal-quake":         "deepin-terminal --quake-mode",
	"lock-screen":            "originmap=$(setxkbmap -query | grep option | awk -F ' ' '{print $2}');/usr/bin/setxkbmap -option grab:break_actions&&/usr/bin/xdotool key XF86Ungrab&&dbus-send --print-reply --dest=com.deepin.dde.lockFront /com/deepin/dde/lockFront com.deepin.dde.lockFront.Show&&/usr/bin/setxkbmap -option; setxkbmap -option $originmap",
	"logout":                 "dbus-send --print-reply --dest=com.deepin.dde.shutdownFront /com/deepin/dde/shutdownFront com.deepin.dde.shutdownFront.Show",
	"deepin-screen-recorder": "dbus-send --print-reply --dest=com.deepin.ScreenRecorder /com/deepin/ScreenRecorder com.deepin.ScreenRecorder.stopRecord",
	"system-monitor":         "/usr/bin/deepin-system-monitor",
	"color-picker":           "dbus-send --print-reply --dest=com.deepin.Picker /com/deepin/Picker com.deepin.Picker.Show",
	// screenshot actions:
	"screenshot":             screenshotCmdPrefix + "StartScreenshot",
	"screenshot-fullscreen":  screenshotCmdPrefix + "FullscreenScreenshot",
	"screenshot-window":      screenshotCmdPrefix + "TopWindowScreenshot",
	"screenshot-delayed":     screenshotCmdPrefix + "DelayScreenshot int64:5",
	"screenshot-ocr":         screenshotCmdPrefix + "OcrScreenshot",
	"screenshot-scroll":      screenshotCmdPrefix + "ScrollScreenshot",
	"file-manager":           "/usr/lib/deepin-daemon/default-file-manager",
	"disable-touchpad":       "gsettings set com.deepin.dde.touchpad touchpad-enabled false",
	"wm-switcher":            "dbus-send --type=method_call --dest=com.deepin.WMSwitcher /com/deepin/WMSwitcher com.deepin.WMSwitcher.RequestSwitchWM",
	"turn-off-screen":        "sleep 0.5; xset dpms force off",
	"notification-center":    "dbus-send --print-reply --dest=com.deepin.dde.osd /org/freedesktop/Notifications com.deepin.dde.Notification.Toggle",
	"clipboard":              "dbus-send --print-reply --dest=com.deepin.dde.Clipboard /com/deepin/dde/Clipboard com.deepin.dde.Clipboard.Toggle",
	"global-search":          "/usr/libexec/dde-daemon/keybinding/shortcut-dde-grand-search.sh",
	"switch-next-kbd-layout": "dbus-send --print-reply --dest=com.deepin.daemon.Keybinding /com/deepin/daemon/InputDevice/Keyboard com.deepin.daemon.InputDevice.Keyboard.ToggleNextLayout",
	// cmd
	"calculator": "/usr/bin/deepin-calculator",
	"search":     "/usr/libexec/dde-daemon/keybinding/shortcut-dde-grand-search.sh",
}

type actionHandler struct {
	Actions []struct {
		Key    string `json:"Key"`
		Action string `json:"Action"`
	} `json:"Actions"`
}

func (a *actionHandler) getCmd(id string) (cmd string, ok bool) {
	for _, v := range a.Actions {
		if v.Key == id {
			return v.Action, true
		}
	}
	return "", false
}

func loadSystemActionsFile(file string) (*actionHandler, error) {
	logger.Debug("load system action file:", file)

	// #nosec G304
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var handler actionHandler
	err = json.Unmarshal(content, &handler)
	if err != nil {
		return nil, err
	}

	return &handler, nil
}

func getSystemActionsFile() string {
	var file = path.Join("/usr/local/share", systemActionsFile)
	if dutils.IsFileExist(file) {
		return file
	}

	file = path.Join("/usr/share", systemActionsFile)
	if dutils.IsFileExist(file) {
		return file
	}

	return ""
}
