// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package shortcuts

import (
	"encoding/json"
	"io/ioutil"
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
	"launcher":               "dbus-send --print-reply --dest=org.deepin.dde.Launcher1 /org/deepin/dde/Launcher1 org.deepin.dde.Launcher1.Toggle",
	"terminal":               "/usr/lib/deepin-daemon/default-terminal",
	"terminal-quake":         "dde-am deepin-terminal quake-mode",
	"lock-screen":            "/usr/lib/deepin-daemon/dde-lock.sh",
	"logout":                 "dbus-send --print-reply --dest=org.deepin.dde.ShutdownFront1 /org/deepin/dde/ShutdownFront1 org.deepin.dde.ShutdownFront1.Show",
	"deepin-screen-recorder": "dbus-send --print-reply --dest=com.deepin.ScreenRecorder /com/deepin/ScreenRecorder com.deepin.ScreenRecorder.stopRecord",
	"system-monitor":         "dde-am deepin-system-monitor",
	"color-picker":           "dbus-send --print-reply --dest=com.deepin.Picker /com/deepin/Picker com.deepin.Picker.Show",
	// screenshot actions:1
	"screenshot":            screenshotCmdPrefix + "StartScreenshot",
	"screenshot-fullscreen": screenshotCmdPrefix + "FullscreenScreenshot",
	"screenshot-window":     screenshotCmdPrefix + "TopWindowScreenshot",
	"screenshot-delayed":    screenshotCmdPrefix + "DelayScreenshot int64:5",
	"screenshot-ocr":        screenshotCmdPrefix + "OcrScreenshot",
	"screenshot-scroll":     screenshotCmdPrefix + "ScrollScreenshot",
	"file-manager":          "/usr/lib/deepin-daemon/default-file-manager",
	"disable-touchpad":      "gsettings set com.deepin.dde.touchpad touchpad-enabled false",
	"wm-switcher":           "dbus-send --type=method_call --dest=org.deepin.dde.WMSwitcher1 /org/deepin/dde/WMSwitcher1 org.deepin.dde.WMSwitcher1.RequestSwitchWM",
	"turn-off-screen":       "sleep 0.5; xset dpms force off",
	"notification-center":   "dbus-send --print-reply --dest=org.deepin.dde.Widgets1 /org/deepin/dde/Widgets1 org.deepin.dde.Widgets1.Toggle",
	"clipboard":             "dbus-send --print-reply --dest=org.deepin.dde.Clipboard1 /org/deepin/dde/Clipboard1 org.deepin.dde.Clipboard1.Toggle",
	"global-search":         "/usr/libexec/dde-daemon/keybinding/shortcut-dde-grand-search.sh",
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

	filepath, _ := xdg.SearchDataFile(systemActionsFile)
	return filepath
}
