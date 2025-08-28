// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package shortcuts

import (
	"github.com/linuxdeepin/go-lib/gettext"
)

func getSystemIdNameMap() map[string]string {
	var idNameMap = map[string]string{
		"launcher":               gettext.Tr("Launcher"),
		"terminal":               gettext.Tr("Terminal"),
		"deepinScreenRecorder":   gettext.Tr("Screen Recorder"),
		"lockScreen":             gettext.Tr("Lock screen"),
		"showDock":               gettext.Tr("Show/Hide the dock"),
		"logout":                 gettext.Tr("Shutdown interface"),
		"terminalQuake":          gettext.Tr("Terminal Quake Window"),
		"screenshot":             gettext.Tr("Screenshot"),
		"screenshotFullscreen":   gettext.Tr("Full screenshot"),
		"screenshotWindow":       gettext.Tr("Window screenshot"),
		"screenshotDelayed":      gettext.Tr("Delay screenshot"),
		"screenshotOcr":          gettext.Tr("OCR (Image to Text)"),
		"screenshotScroll":       gettext.Tr("Scrollshot"),
		"fileManager":            gettext.Tr("File manager"),
		"disableTouchpad":        gettext.Tr("Disable Touchpad"),
		"wmSwitcher":             gettext.Tr("Switch window effects"),
		"turnOffScreen":          gettext.Tr("Fast Screen Off"),
		"systemMonitor":          gettext.Tr("System Monitor"),
		"colorPicker":            gettext.Tr("Deepin Picker"),
		"textToSpeech":           gettext.Tr("Text to Speech"),
		"speechToText":           gettext.Tr("Speech to Text"),
		"clipboard":              gettext.Tr("Clipboard"),
		"translation":            gettext.Tr("Translation"),
		"globalSearch":           gettext.Tr("Grand Search"),
		"notificationCenter":     gettext.Tr("Notification Center"),
		"switch-next-kbd-layout": gettext.Tr("Switch Layout"),
		"switchMonitors":         gettext.Tr("Toggle multiple displays"),
	}
	return idNameMap
}

func getSpecialIdNameMap() map[string]string {
	var idNameMap = map[string]string{
		"switch-kbd-layout": gettext.Tr("Switch Layout"),
	}
	return idNameMap
}

func getWMIdNameMap() map[string]string {
	var idNameMap = map[string]string{
		"switchToWorkspace1":         "Switch to workspace 1",
		"switchToWorkspace2":         "Switch to workspace 2",
		"switchToWorkspace3":         "Switch to workspace 3",
		"switchToWorkspace4":         "Switch to workspace 4",
		"switchToWorkspace5":         "Switch to workspace 5",
		"switchToWorkspace6":         "Switch to workspace 6",
		"switchToWorkspace7":         "Switch to workspace 7",
		"switchToWorkspace8":         "Switch to workspace 8",
		"switchToWorkspace9":         "Switch to workspace 9",
		"switchToWorkspace10":        "Switch to workspace 10",
		"switchToWorkspace11":        "Switch to workspace 11",
		"switchToWorkspace12":        "Switch to workspace 12",
		"switchToWorkspaceLeft":      gettext.Tr("Switch to left workspace"),
		"switchToWorkspaceRight":     gettext.Tr("Switch to right workspace"),
		"switchToWorkspaceUp":        gettext.Tr("Switch to upper workspace"),
		"switchToWorkspaceDown":      gettext.Tr("Switch to lower workspace"),
		"switchToWorkspaceLast":      "Switch to last workspace",
		"switchGroup":                gettext.Tr("Switch similar windows"),
		"switchGroupBackward":        gettext.Tr("Switch similar windows in reverse"),
		"switchApplications":         gettext.Tr("Switch windows"),
		"switchApplicationsBackward": gettext.Tr("Switch windows in reverse"),
		"switchWindows":              "Switch windows",
		"switchWindowsBackward":      "Reverse switch windows",
		"switchPanels":               "Switch system controls",
		"switchPanelsBackward":       "Reverse switch system controls",
		"cycleGroup":                 "Switch windows of an app directly",
		"cycleGroupBackward":         "Reverse switch windows of an app directly",
		"cycleWindows":               "Switch windows directly",
		"cycleWindowsBackward":       "Reverse switch windows directly",
		"cyclePanels":                "Switch system controls directly",
		"cyclePanelsBackward":        "Reverse switch system controls directly",
		"showDesktop":                gettext.Tr("Show desktop"),
		"panelMainMenu":              "Show the activities overview",
		"panelRunDialog":             "Show the run command prompt",
		// Don't use
		// "set-spew-mark":                gettext.Tr(""),
		"activateWindowMenu":        "Activate window menu",
		"toggleFullscreen":          "toggle-fullscreen",
		"toggleMaximized":           "Toggle maximization state",
		"toggleAbove":               "Toggle window always appearing on top",
		"maximize":                  gettext.Tr("Maximize window"),
		"unmaximize":                gettext.Tr("Restore window"),
		"toggleShaded":              "Switch furl state",
		"minimize":                  gettext.Tr("Minimize window"),
		"close":                     gettext.Tr("Close window"),
		"beginMove":                 gettext.Tr("Move window"),
		"beginResize":               gettext.Tr("Resize window"),
		"toggle-on-all-workspaces":  "Toggle window on all workspaces or one",
		"moveToWorkspace1":          "Move to workspace 1",
		"moveToWorkspace2":          "Move to workspace 2",
		"moveToWorkspace3":          "Move to workspace 3",
		"moveToWorkspace4":          "Move to workspace 4",
		"moveToWorkspace5":          "Move to workspace 5",
		"moveToWorkspace6":          "Move to workspace 6",
		"moveToWorkspace7":          "Move to workspace 7",
		"moveToWorkspace8":          "Move to workspace 8",
		"moveToWorkspace9":          "Move to workspace 9",
		"moveToWorkspace10":         "Move to workspace 10",
		"moveToWorkspace11":         "Move to workspace 11",
		"moveToWorkspace12":         "Move to workspace 12",
		"moveToWorkspaceLast":       "Move to last workspace",
		"moveToWorkspaceLeft":       gettext.Tr("Move to left workspace"),
		"moveToWorkspaceRight":      gettext.Tr("Move to right workspace"),
		"moveToWorkspaceUp":         gettext.Tr("Move to upper workspace"),
		"moveToWorkspaceDown":       gettext.Tr("Move to lower workspace"),
		"moveToMonitorLeft":         "Move to left monitor",
		"moveToMonitorRight":        "Move to right monitor",
		"moveToMonitorUp":           "Move to up monitor",
		"moveToMonitorDown":         "Move to down monitor",
		"raise-or-lower":            "Raise window if covered, otherwise lower it",
		"raise":                     "Raise window above other windows",
		"lower":                     "Lower window below other windows",
		"maximizeVertically":        "Maximize window vertically",
		"maximizeHorizontally":      "Maximize window horizontally",
		"move-to-corner-nw":         "Move window to top left corner",
		"move-to-corner-ne":         "Move window to top right corner",
		"move-to-corner-sw":         "Move window to bottom left corner",
		"move-to-corner-se":         "Move window to bottom right corner",
		"move-to-side-n":            "Move window to top edge of screen",
		"move-to-side-s":            "Move window to bottom edge of screen",
		"move-to-side-e":            "Move window to right side of screen",
		"move-to-side-w":            "Move window to left side of screen",
		"move-to-center":            "Move window to center of screen",
		"switchInputSource":         "Binding to select the next input source",
		"switchInputSourceBackward": "Binding to select the previous input source",
		"always-on-top":             "Set or unset window to appear always on top",
		"exposeAllWindows":          gettext.Tr("Display windows of all workspaces"),
		"exposeWindows":             gettext.Tr("Display windows of current workspace"),
		"previewWorkspace":          gettext.Tr("Display workspace"),
		"viewZoomIn":                gettext.Tr("Zoom In"),
		"viewZoomOut":               gettext.Tr("Zoom Out"),
		"viewActualSize":            gettext.Tr("Zoom to Actual Size"),
		"toggleToLeft":              gettext.Tr("Window Quick Tile Left"),
		"toggleToRight":             gettext.Tr("Window Quick Tile Right"),
	}
	return idNameMap
}

func getMediaIdNameMap() map[string]string {
	var idNameMap = map[string]string{
		"messenger":         "Messenger",         // XF86Messenger
		"save":              "Save",              // XF86Save
		"new":               "New",               // XF86New
		"wakeUp":            "WakeUp",            // XF86WakeUp
		"audioRewind":       "AudioRewind",       // XF86AudioRewind
		"audioMute":         "AudioMute",         // XF86AudioMute
		"monBrightnessUp":   "MonBrightnessUp",   // XF86MonBrightnessUp
		"wlan":              "WLAN",              // XF86WLAN
		"audioMedia":        "AudioMedia",        // XF86AudioMedia
		"reply":             "Reply",             // XF86Reply
		"favorites":         "Favorites",         // XF86Favorites
		"audioPlay":         "AudioPlay",         // XF86AudioPlay
		"audioMicMute":      "AudioMicMute",      // XF86AudioMicMute
		"audioPause":        "AudioPause",        // XF86AudioPause
		"audioStop":         "AudioStop",         // XF86AudioStop
		"documents":         "Documents",         // XF86Documents
		"game":              "Game",              // XF86Game
		"search":            "Search",            // XF86Search
		"audioRecord":       "AudioRecord",       // XF86AudioRecord
		"display":           "Display",           // XF86Display
		"reload":            "Reload",            // XF86Reload
		"explorer":          "Explorer",          // XF86Explorer
		"calculator":        "Calculator",        // XF86Calculator
		"calendar":          "Calendar",          // XF86Calendar
		"forward":           "Forward",           // XF86Forward
		"cut":               "Cut",               // XF86Cut
		"monBrightnessDown": "MonBrightnessDown", // XF86MonBrightnessDown
		"copy":              "Copy",              // XF86Copy
		"tools":             "Tools",             // XF86Tools
		"audioRaiseVolume":  "AudioRaiseVolume",  // XF86AudioRaiseVolume
		"mediaClose":        "media-Close",       // XF86Close
		"www":               "WWW",               // XF86WWW
		"homePage":          "HomePage",          // XF86HomePage
		"sleep":             "Sleep",             // XF86Sleep
		"audioLowerVolume":  "AudioLowerVolume",  // XF86AudioLowerVolume
		"audioPrev":         "AudioPrev",         // XF86AudioPrev
		"audioNext":         "AudioNext",         // XF86AudioNext
		"paste":             "Paste",             // XF86Paste
		"open":              "Open",              // XF86Open
		"send":              "Send",              // XF86Send
		"myComputer":        "MyComputer",        // XF86MyComputer
		"mail":              "Mail",              // XF86Mail
		"adjustBrightness":  "BrightnessAdjust",  // XF86BrightnessAdjust
		"logOff":            "LogOff",            // XF86LogOff
		"pictures":          "Pictures",          // XF86Pictures
		"terminal":          "Terminal",          // XF86Terminal
		"video":             "Video",             // XF86Video
		"music":             "Music",             // XF86Music
		"appLeft":           "ApplicationLeft",   // XF86ApplicationLeft
		"appRight":          "ApplicationRight",  // XF86ApplicationRight
		"meeting":           "Meeting",           // XF86Meeting
		"touchpadToggle":    "ToggleTouchpad",    // XF86TouchpadToggle
		"away":              "Away",              // XF86Away
		"webCam":            "Camera",            // XF86WebCam
	}
	return idNameMap
}
