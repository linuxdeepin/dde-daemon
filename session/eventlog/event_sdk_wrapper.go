// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package eventlog

// #cgo CXXFLAGS:-O2 -std=c++11
// #cgo CFLAGS: -W -Wall -fstack-protector-all -fPIC
// #cgo LDFLAGS:-ldl
// #include <stdlib.h>
// #include "event_sdk.h"
import "C"
import (
	"encoding/json"
	"sync"
	"unsafe"
)

var (
	sdkInitMu   sync.Mutex
	sdkInited   bool
	sdkDisabled bool // Cached disabled state to avoid repeated init attempts
)

// EventLogData represents the event log data structure
type EventLogData struct {
	Tid     int64             `json:"tid"`
	Target  string            `json:"target"`
	Message map[string]string `json:"message"`
}

// InitEventSDK initializes the event log SDK.
// It returns true if initialization succeeded or was already done.
func InitEventSDK() bool {
	sdkInitMu.Lock()
	defer sdkInitMu.Unlock()

	if sdkInited {
		return true
	}

	if sdkDisabled {
		return false
	}

	ret := C.InitEventSDK()
	switch ret {
	case C.EVENT_LOG_SUCCESS:
		sdkInited = true
		return true
	case C.EVENT_LOG_DISABLED:
		sdkDisabled = true
		return false
	default:
		logger.Warning("failed to initialize event SDK:", ret)
		return false
	}
}

// CloseEventSDK closes the event log SDK.
func CloseEventSDK() {
	sdkInitMu.Lock()
	defer sdkInitMu.Unlock()

	if sdkInited {
		C.CloseEventLog()
		sdkInited = false
	}
}

// WriteEventLog writes an event log with the given data.
// It returns false if the SDK is not initialized.
func WriteEventLog(data *EventLogData) bool {
	if !InitEventSDK() {
		return false
	}

	content, err := json.Marshal(data)
	if err != nil {
		logger.Warning("failed to marshal event log data:", err)
		return false
	}

	cStr := C.CString(string(content))
	defer C.free(unsafe.Pointer(cStr))

	C.writeEventLog(cStr, 0)
	return true
}

// WriteEventLogSimple writes an event log with tid, target and a single key-value message.
func WriteEventLogSimple(tid int64, target, key, value string) bool {
	return WriteEventLog(&EventLogData{
		Tid:    tid,
		Target: target,
		Message: map[string]string{
			key: value,
		},
	})
}
