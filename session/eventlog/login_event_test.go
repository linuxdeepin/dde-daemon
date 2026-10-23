// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package eventlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	previousRebootRecord = "reboot   system boot  4.19.0-amd64-des 2022-08-15T14:13:35+08:00 - 2022-08-15T17:03:29+08:00  (02:49)"
	bogusRebootRecord    = "not a reboot record"
)

func Test_writeLoginLog(t *testing.T) {
	t.Run("Test writeLoginLog", func(t *testing.T) {
		c := new(loginEventCollector)
		assert.NoError(t, c.writeLoginLog(&loginTimeInfo{Tid: SystemBootTid}))
	})
}

func Test_selectSecondRebootRecord(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{name: "no records", content: "", wantErr: true},
		{name: "one record", content: previousRebootRecord + "\n", wantErr: true},
		{name: "two records", content: bogusRebootRecord + "\n" + previousRebootRecord + "\n", want: previousRebootRecord},
		{name: "second record without trailing newline", content: bogusRebootRecord + "\n" + previousRebootRecord, want: previousRebootRecord},
		{name: "second of three records", content: "first\n" + previousRebootRecord + "\nthird\n", want: previousRebootRecord},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectSecondRebootRecord([]byte(tt.content))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// installFakeLast puts an executable named "last" first in PATH. It records
// its arguments and echoes the configured output, exercising the real process
// invocation directly.
func installFakeLast(t *testing.T, output string) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$LAST_LOG"
if [ -n "$LAST_EXIT" ]; then
	exit "$LAST_EXIT"
fi
printf '%s' "$LAST_OUTPUT"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "last"), []byte(script), 0755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LAST_LOG", logPath)
	t.Setenv("LAST_OUTPUT", output)
	t.Setenv("LAST_EXIT", "")
	return logPath
}

func Test_collectLoginTimeInfoSelectsSecondRecord(t *testing.T) {
	// The existing parser interprets the fixture's offset in time.Local.
	local := time.Local
	time.Local = time.FixedZone("UTC+8", 8*60*60)
	t.Cleanup(func() { time.Local = local })

	logPath := installFakeLast(t, bogusRebootRecord+"\n"+previousRebootRecord+"\nlast footer\n")

	c := new(loginEventCollector)
	var written []string
	c.writeEventLogFn = func(content string) {
		written = append(written, content)
	}

	require.NoError(t, c.collectLoginTimeInfo())
	require.Len(t, written, 1)

	args, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Equal(t, "-x reboot --time-format iso\n", string(args))

	var info loginTimeInfo
	require.NoError(t, json.Unmarshal([]byte(written[0]), &info))
	assert.Equal(t, SystemBootTid, info.Tid)

	recordStart, err := time.Parse(time.RFC3339, "2022-08-15T14:13:35+08:00")
	require.NoError(t, err)
	recordEnd, err := time.Parse(time.RFC3339, "2022-08-15T17:03:29+08:00")
	require.NoError(t, err)
	assert.Equal(t, recordStart.UnixMilli(), info.ShutdownTime)
	assert.Equal(t, recordEnd.UnixMilli(), info.BootTime)
}

func Test_collectLoginTimeInfoErrors(t *testing.T) {
	t.Run("no records", func(t *testing.T) {
		installFakeLast(t, "")
		c := new(loginEventCollector)
		written := false
		c.writeEventLogFn = func(string) { written = true }
		assert.Error(t, c.collectLoginTimeInfo())
		assert.False(t, written)
	})

	t.Run("one record", func(t *testing.T) {
		installFakeLast(t, previousRebootRecord+"\n")
		c := new(loginEventCollector)
		c.writeEventLogFn = func(string) {
			t.Fatal("should not write without a previous reboot record")
		}
		assert.Error(t, c.collectLoginTimeInfo())
	})

	t.Run("second record is not a reboot", func(t *testing.T) {
		installFakeLast(t, previousRebootRecord+"\n"+bogusRebootRecord+"\n")
		c := new(loginEventCollector)
		c.writeEventLogFn = func(string) {
			t.Fatal("should not write invalid reboot data")
		}
		assert.Error(t, c.collectLoginTimeInfo())
	})

	t.Run("last command fails", func(t *testing.T) {
		installFakeLast(t, previousRebootRecord+"\n"+previousRebootRecord+"\n")
		t.Setenv("LAST_EXIT", "1")
		c := new(loginEventCollector)
		c.writeEventLogFn = func(string) {
			t.Fatal("should not write when last fails")
		}
		assert.Error(t, c.collectLoginTimeInfo())
	})
}

func Test_parseLastRebootCmdOutPut(t *testing.T) {
	t.Run("Test parseLastRebootCmdOutPut", func(t *testing.T) {
		_, err := parseLastRebootCmdOutPut([]byte(previousRebootRecord))
		assert.NoError(t, err)
	})
}
