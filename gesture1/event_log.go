// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package gesture1

import (
	"os/exec"
	"strings"
	"sync"

	"github.com/linuxdeepin/dde-daemon/session/eventlog"
)

const (
	eventTidKWinMultiTaskView = 1000300000
	eventTidKWinSplitScreen   = 1000300004

	eventLaunchTypeTouchPad    = "3"
	eventLaunchTypeTouchScreen = "4"
)

var (
	kwinVersionOnce sync.Once
	kwinVersion     string
)

func logMultiTaskViewEvent(launchType string) {
	data := &eventlog.EventLogData{
		Tid:    eventTidKWinMultiTaskView,
		Target: "kwin_multitask_view",
		Message: map[string]string{
			"launch_type":  launchType,
			"kwin_version": getKWinVersion(),
		},
	}
	if eventlog.WriteEventLog(data) {
		logger.Debug("EventLog: kwin multitask view launch type:", launchType)
	}
}

func logSplitScreenEvent() {
	data := &eventlog.EventLogData{
		Tid:    eventTidKWinSplitScreen,
		Target: "kwin_split_screen",
		Message: map[string]string{
			"launch_type":  eventLaunchTypeTouchPad,
			"kwin_version": getKWinVersion(),
		},
	}
	if eventlog.WriteEventLog(data) {
		logger.Debug("EventLog: kwin split screen")
	}
}

func getKWinVersion() string {
	kwinVersionOnce.Do(func() {
		kwinVersion = packageVersion("kwin-x11")
	})
	return kwinVersion
}

func packageVersion(packageName string) string {
	packageName = strings.TrimSpace(packageName)
	if packageName == "" || strings.ContainsAny(packageName, "/ \t\n\r") {
		return ""
	}

	out, err := exec.Command("dpkg-query", "-W", "-f=${Version}", packageName).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
