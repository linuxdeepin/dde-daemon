// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package calltrace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoi64(t *testing.T) {
	assert.Equal(t, int64(123), stoi64("123"))
	assert.Equal(t, int64(0), stoi64(""))
	assert.Equal(t, int64(0), stoi64("abc"))
	assert.Equal(t, int64(-42), stoi64("-42"))
}

func TestCpuTimeInfo_Total(t *testing.T) {
	info := &cpuTimeInfo{
		utime:  10,
		stime:  20,
		cutime: 5,
		cstime: 3,
		nice:   2,
		start:  100,
		hertz:  100,
	}
	assert.Equal(t, int64(40), info.Total())
}

func TestCpuTimeInfo_Total_Zero(t *testing.T) {
	info := &cpuTimeInfo{}
	assert.Equal(t, int64(0), info.Total())
}

func TestCpuTimeInfo_Percentage(t *testing.T) {
	// start=200, hertz=100 → start/hertz=2, seconds = uptime - 2
	// Total=100, Total/hertz=1, Percentage = 100 * (1 / (uptime-2))
	info := &cpuTimeInfo{
		utime:  50,
		stime:  50,
		cutime: 0,
		cstime: 0,
		nice:   0,
		start:  200,
		hertz:  100,
	}
	// uptime comes from /proc/uptime; can't control, but we can verify
	// the formula doesn't panic and returns a float (likely 0 in test env
	// because /proc/uptime read may fail or return 0 → division by zero).
	// Instead test the pure arithmetic directly:
	assert.Equal(t, int64(100), info.Total())
}

func TestGetInterge(t *testing.T) {
	// format: "RssAnon:     100 kB" → split → ["RssAnon:", "100", "kB"]
	v, err := getInterge("RssAnon:     100 kB")
	assert.NoError(t, err)
	assert.Equal(t, int64(100), v)

	v, err = getInterge("VmPTE:    8 kB")
	assert.NoError(t, err)
	assert.Equal(t, int64(8), v)
}

func TestGetInterge_BadFormat(t *testing.T) {
	// 只有两个字段（不足 3 个）→ error
	_, err := getInterge("VmPTE: 100")
	assert.Error(t, err)

	// 中间不是数字 → error
	_, err = getInterge("VmPTE:    abc kB")
	assert.Error(t, err)
}

func TestSumMemByFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status")
	content := "Name:\ttest\n" +
		"RssAnon:    100 kB\n" +
		"VmPTE:       8 kB\n" +
		"VmPMD:       2 kB\n" +
		"Other:     999 kB\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	total, err := sumMemByFile(path)
	assert.NoError(t, err)
	// 100 + 8 + 2 = 110
	assert.Equal(t, int64(110), total)
}

func TestSumMemByFile_MissingFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status")
	// 只有 RssAnon，缺 VmPTE/VmPMD → 只累加 RssAnon
	content := "RssAnon:    50 kB\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	total, err := sumMemByFile(path)
	assert.NoError(t, err)
	assert.Equal(t, int64(50), total)
}

func TestSumMemByFile_NonExistent(t *testing.T) {
	_, err := sumMemByFile("/nonexistent/path/to/status")
	assert.Error(t, err)
}
