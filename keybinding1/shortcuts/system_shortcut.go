// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package shortcuts

import (
	"encoding/json"
	"os"
	"path"
	"sync"

	"github.com/adrg/xdg"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

const (
	// Under '/usr/share' or '/usr/local/share'
	systemActionsFile   = "dde-daemon/keybinding/system_actions.json"
	screenshotCmdPrefix = "dbus-send --print-reply --dest=com.deepin.Screenshot /com/deepin/Screenshot com.deepin.Screenshot."
)

type SystemShortcut struct {
	*ShortcutObject
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
	if id == "lockScreen" && _useWayland {
		id = "lockScreen-wayland"
	}
	return defaultSysActionCmdMap[id]
}

// key is id, value is commandline.
var defaultSysActionCmdMap = map[string]string{
	"launcher":      "dbus-send --print-reply --dest=org.deepin.dde.Launcher1 /org/deepin/dde/Launcher1 org.deepin.dde.Launcher1.Toggle",
	"terminal":      "/usr/lib/deepin-daemon/default-terminal",
	"terminalQuake": "dde-am deepin-terminal quake-mode",
	"lockScreen":    "originmap=$(setxkbmap -query | grep option | awk -F ' ' '{print $2}');/usr/bin/setxkbmap -option grab:break_actions&&/usr/bin/xdotool key XF86Ungrab&&dbus-send --print-reply --dest=org.deepin.dde.LockFront1 /org/deepin/dde/LockFront1 org.deepin.dde.LockFront1.Show&&/usr/bin/setxkbmap -option $originmap",
	//wayland不能设置XF86Ungrab，否则会导致Bug-224309
	"lockScreen-wayland":   "originmap=$(setxkbmap -query | grep option | awk -F ' ' '{print $2}');/usr/bin/setxkbmap -option grab:break_actions&&dbus-send --print-reply --dest=org.deepin.dde.LockFront1 /org/deepin/dde/LockFront1 org.deepin.dde.LockFront1.Show&&/usr/bin/setxkbmap -option $originmap",
	"logout":               "dbus-send --print-reply --dest=org.deepin.dde.ShutdownFront1 /org/deepin/dde/ShutdownFront1 org.deepin.dde.ShutdownFront1.Show",
	"deepinScreenRecorder": "dbus-send --print-reply --dest=com.deepin.ScreenRecorder /com/deepin/ScreenRecorder com.deepin.ScreenRecorder.stopRecord",
	"systemMonitor":        "/usr/bin/deepin-system-monitor",
	"colorPicker":          "dbus-send --print-reply --dest=com.deepin.Picker /com/deepin/Picker com.deepin.Picker.Show",
	// screenshot actions:
	"screenshot":             screenshotCmdPrefix + "StartScreenshot",
	"screenshotFullscreen":   screenshotCmdPrefix + "FullscreenScreenshot",
	"screenshotWindow":       screenshotCmdPrefix + "TopWindowScreenshot",
	"screenshotDelayed":      screenshotCmdPrefix + "DelayScreenshot int64:5",
	"screenshotOcr":          screenshotCmdPrefix + "OcrScreenshot",
	"screenshotScroll":       screenshotCmdPrefix + "ScrollScreenshot",
	"fileManager":            "/usr/lib/deepin-daemon/default-file-manager",
	"disableTouchpad":        "dde-dconfig set -a  org.deepin.dde.daemon -r org.deepin.dde.daemon.touchpad -k touchpadEnabled -v false",
	"wmSwitcher":             "dbus-send --type=method_call --dest=org.deepin.dde.WMSwitcher1 /org/deepin/dde/WMSwitcher1 org.deepin.dde.WMSwitcher1.RequestSwitchWM",
	"turnOffScreen":          "sleep 0.5; xset dpms force off",
	"notificationCenter":     "dbus-send --print-reply --dest=org.deepin.dde.Osd1 /org/deepin/dde/shell/notification/center org.deepin.dde.shell.notification.center.Toggle",
	"clipboard":              "dbus-send --print-reply --dest=org.deepin.dde.Clipboard1 /org/deepin/dde/Clipboard1 org.deepin.dde.Clipboard1.Toggle; dbus-send --print-reply --dest=org.deepin.dde.Launcher1 /org/deepin/dde/Launcher1 org.deepin.dde.Launcher1.Hide",
	"globalSearch":           "/usr/libexec/dde-daemon/keybinding/shortcut-dde-grand-search.sh",
	"switch-next-kbd-layout": "dbus-send --print-reply --dest=org.deepin.dde.Keybinding1 /org/deepin/dde/InputDevice1/Keyboard org.deepin.dde.InputDevice1.Keyboard.ToggleNextLayout",
	"switchMonitors":         "/usr/libexec/dde-daemon/keybinding/shortcut-dde-switch-monitors.sh",
	// cmd
	"calculator": "/usr/bin/deepin-calculator",
	"search":     "/usr/libexec/dde-daemon/keybinding/shortcut-dde-grand-search.sh",
	"script":     "/usr/libexec/dde-daemon/keybinding/shortcut-dde-script.sh",
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
	content, err := os.ReadFile(file)
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

	filepath, _ := xdg.SearchDataFile(systemActionsFile)
	return filepath
}
