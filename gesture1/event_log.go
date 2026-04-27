// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package gesture1

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/linuxdeepin/dde-daemon/session/eventlog"
	"github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/wm/ewmh"
	"github.com/linuxdeepin/go-x11-client/util/wm/icccm"
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
	appName, appVersion := getActiveWindowAppInfo()
	data := &eventlog.EventLogData{
		Tid:    eventTidKWinSplitScreen,
		Target: "kwin_split_screen",
		Message: map[string]string{
			"app_name":     appName,
			"app_version":  appVersion,
			"launch_type":  eventLaunchTypeTouchPad,
			"kwin_version": getKWinVersion(),
		},
	}
	if eventlog.WriteEventLog(data) {
		logger.Debug("EventLog: kwin split screen app:", appName, "version:", appVersion)
	}
}

func getKWinVersion() string {
	kwinVersionOnce.Do(func() {
		kwinVersion = packageVersion("kwin-x11")
	})
	return kwinVersion
}

func getActiveWindowAppInfo() (string, string) {
	if getX11Conn() == nil {
		return "", ""
	}

	win, err := ewmh.GetActiveWindow(xconn).Reply(xconn)
	if err != nil {
		logger.Warning("Failed to get current active window:", err)
		return "", ""
	}

	appName, appVersion := getWindowAppNameAndVersion(win)
	if appName != "" || appVersion != "" {
		return appName, appVersion
	}

	pid, err := ewmh.GetWMPid(xconn, win).Reply(xconn)
	if err != nil {
		logger.Warning("Failed to get current window pid:", err)
		return appName, appVersion
	}

	return getProcessAppInfo(pid)
}

func getWindowAppNameAndVersion(win x.Window) (string, string) {
	wmClass, err := icccm.GetWMClass(xconn, win).Reply(xconn)
	if err == nil {
		if appName, appVersion := getDesktopAppInfo(wmClass.Class); appName != "" || appVersion != "" {
			return appName, appVersion
		}
		if appName, appVersion := getDesktopAppInfo(wmClass.Instance); appName != "" || appVersion != "" {
			return appName, appVersion
		}
	}

	name, err := ewmh.GetWMName(xconn, win).Reply(xconn)
	if err == nil {
		return name, ""
	}

	return "", ""
}

func getDesktopAppInfo(id string) (string, string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ""
	}

	desktopIDs := []string{id}
	if !strings.HasSuffix(id, ".desktop") {
		desktopIDs = append(desktopIDs, id+".desktop")
	}

	dataDirsEnv := os.Getenv("XDG_DATA_DIRS")
	if dataDirsEnv == "" {
		dataDirsEnv = "/usr/local/share:/usr/share"
	}
	dataDirs := append([]string{filepath.Join(os.Getenv("HOME"), ".local/share")}, strings.Split(dataDirsEnv, ":")...)
	for _, dataDir := range dataDirs {
		if dataDir == "" {
			continue
		}
		for _, desktopID := range desktopIDs {
			desktopPath := filepath.Join(dataDir, "applications", desktopID)
			if appName, appVersion := readDesktopAppInfo(desktopPath); appName != "" || appVersion != "" {
				return appName, appVersion
			}
		}
	}

	if appVersion := packageVersion(id); appVersion != "" {
		return id, appVersion
	}
	return "", ""
}

func readDesktopAppInfo(desktopPath string) (string, string) {
	data, err := os.ReadFile(desktopPath)
	if err != nil {
		return "", ""
	}

	var appName, appVersion, packageName string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if appName == "" && strings.HasPrefix(line, "Name=") {
			appName = strings.TrimPrefix(line, "Name=")
		} else if strings.HasPrefix(line, "X-Deepin-AppID=") {
			packageName = strings.TrimPrefix(line, "X-Deepin-AppID=")
		} else if strings.HasPrefix(line, "X-Deepin-PackageName=") {
			packageName = strings.TrimPrefix(line, "X-Deepin-PackageName=")
		}
	}
	if packageName == "" {
		packageName = packageNameByFile(desktopPath)
	}
	if packageName != "" {
		appVersion = packageVersion(packageName)
	}
	return appName, appVersion
}

func getProcessAppInfo(pid uint32) (string, string) {
	cmdline, err := os.ReadFile("/proc/" + strconv.FormatUint(uint64(pid), 10) + "/cmdline")
	if err != nil {
		logger.Warning("Failed to read cmdline:", err)
		return "", ""
	}

	args := strings.Split(string(cmdline), "\x00")
	if len(args) == 0 || args[0] == "" {
		return "", ""
	}

	appName := filepath.Base(args[0])
	return appName, packageVersion(appName)
}

func packageNameByFile(path string) string {
	out, err := exec.Command("dpkg-query", "-S", path).Output()
	if err != nil {
		return ""
	}
	fields := strings.SplitN(strings.TrimSpace(string(out)), ":", 2)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimSpace(fields[0])
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
