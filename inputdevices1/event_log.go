// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"time"

	"github.com/linuxdeepin/dde-daemon/session/eventlog"
)

// Event IDs for input devices (10-digit numbers)
const (
	// Combined event ID for natural scroll settings on startup
	EventTidNaturalScroll = 1000610009 // 自然滚动设置（触控板和鼠标合并）
)

// LogNaturalScroll logs natural scroll state for both touchpad and mouse in one event
// Used for startup logging to reduce log entries
func LogNaturalScroll(touchpadNaturalScroll, mouseNaturalScroll bool) {
	data := &eventlog.EventLogData{
		Tid:    EventTidNaturalScroll,
		Target: "natural_scroll",
		Message: map[string]string{
			"touchpad_natural_scroll": boolToString(touchpadNaturalScroll),
			"mouse_natural_scroll":    boolToString(mouseNaturalScroll),
		},
	}
	if eventlog.WriteEventLog(data) {
		logger.Debug("EventLog: natural scroll - touchpad:", touchpadNaturalScroll, "mouse:", mouseNaturalScroll)
	}
}

// LogTouchpadNaturalScroll logs touchpad natural scroll state (for runtime changes)
func LogTouchpadNaturalScroll(enabled bool) {
	data := &eventlog.EventLogData{
		Tid:    EventTidNaturalScroll,
		Target: "natural_scroll",
		Message: map[string]string{
			"touchpad_natural_scroll": boolToString(enabled),
		},
	}
	if eventlog.WriteEventLog(data) {
		logger.Debug("EventLog: touchpad natural scroll:", enabled)
	}
}

// LogMouseNaturalScroll logs mouse natural scroll state (for runtime changes)
func LogMouseNaturalScroll(enabled bool) {
	data := &eventlog.EventLogData{
		Tid:    EventTidNaturalScroll,
		Target: "natural_scroll",
		Message: map[string]string{
			"mouse_natural_scroll": boolToString(enabled),
		},
	}
	if eventlog.WriteEventLog(data) {
		logger.Debug("EventLog: mouse natural scroll:", enabled)
	}
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// LogOnStartup logs the current state on startup with a delay
// Both touchpad and mouse natural scroll states are logged in one combined event
func LogOnStartup(touchpadNaturalScroll, mouseNaturalScroll bool) {
	// Delay logging to ensure the event log system is ready
	time.AfterFunc(5*time.Second, func() {
		LogNaturalScroll(touchpadNaturalScroll, mouseNaturalScroll)
	})
}
