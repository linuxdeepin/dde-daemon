// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"strconv"
	"sync"
	"time"

	"github.com/linuxdeepin/dde-daemon/session/eventlog"
)

// Event ID for display screen (10-digit number)
const EventTidDisplayScreen = 1000610004

// Debounce delay for display screen events
const displayEventDebounceDelay = 1 * time.Second

// displayEventLogger manages debounced event logging
type displayEventLogger struct {
	mu       sync.Mutex
	timer    *time.Timer
	lastData struct {
		screenCount    int
		displayMode    byte
		primaryMonitor string
	}
}

var displayLogger = &displayEventLogger{}

// getDisplayModeString converts DisplayMode to string for logging
func getDisplayModeString(mode byte, monitorName string) string {
	switch mode {
	case DisplayModeMirror:
		return "MERGE"
	case DisplayModeExtend:
		return "EXTEND"
	case DisplayModeOnlyOne:
		// 单屏模式，返回显示器名称
		if monitorName != "" {
			return monitorName
		}
		return "single"
	default:
		return "custom"
	}
}

// LogDisplayScreen logs display screen information with debounce
func LogDisplayScreen(screenCount int, displayMode byte, primaryMonitor string) {
	displayLogger.mu.Lock()
	defer displayLogger.mu.Unlock()

	// Store the latest data
	displayLogger.lastData.screenCount = screenCount
	displayLogger.lastData.displayMode = displayMode
	displayLogger.lastData.primaryMonitor = primaryMonitor

	// If timer exists, reset it; otherwise create a new one
	if displayLogger.timer != nil {
		displayLogger.timer.Reset(displayEventDebounceDelay)
		return
	}

	displayLogger.timer = time.AfterFunc(displayEventDebounceDelay, func() {
		displayLogger.mu.Lock()
		data := displayLogger.lastData
		displayLogger.mu.Unlock()

		// Actually write the log
		message := map[string]string{
			"screen_count": strconv.Itoa(data.screenCount),
			"display_mode": getDisplayModeString(data.displayMode, data.primaryMonitor),
		}

		eventData := &eventlog.EventLogData{
			Tid:     EventTidDisplayScreen,
			Target:  "display_screen",
			Message: message,
		}

		if eventlog.WriteEventLog(eventData) {
			logger.Debug("EventLog: display screen - count:", data.screenCount, "mode:", data.displayMode)
		}
	})
}

// LogOnStartup logs the current display state on startup with a delay
func LogOnStartup(screenCount int, displayMode byte, primaryMonitor string) {
	// Delay logging to ensure the event log system is ready
	// The debounce mechanism in LogDisplayScreen will handle duplicate logs
	time.AfterFunc(5*time.Second, func() {
		LogDisplayScreen(screenCount, displayMode, primaryMonitor)
	})
}
