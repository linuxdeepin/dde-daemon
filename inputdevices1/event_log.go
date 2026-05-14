// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"github.com/linuxdeepin/dde-daemon/session/eventlog"
)

// Event IDs for input devices (10-digit numbers)
const (
	EventTidNaturalScroll = 1000610009 // 自然滚动设置
)

// LogTouchpadNaturalScroll logs touchpad natural scroll state
// When touchpad is not present, pass hasTouchpad=false to report empty value
func LogTouchpadNaturalScroll(enabled bool, hasTouchpad bool) {
	value := ""
	if hasTouchpad {
		value = boolToString(enabled)
	}
	data := &eventlog.EventLogData{
		Tid:    EventTidNaturalScroll,
		Target: "natural_scroll",
		Message: map[string]string{
			"touchpad_scroll_native_on": value,
		},
	}
	if eventlog.WriteEventLog(data) {
		logger.Debug("EventLog: touchpad natural scroll:", value)
	}
}

// LogMouseNaturalScroll logs mouse natural scroll state (for runtime changes)
func LogMouseNaturalScroll(enabled bool) {
	data := &eventlog.EventLogData{
		Tid:    EventTidNaturalScroll,
		Target: "natural_scroll",
		Message: map[string]string{
			"mouse_scroll_native_on": boolToString(enabled),
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
