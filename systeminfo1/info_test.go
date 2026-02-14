// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package systeminfo

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCPUInfo(t *testing.T) {
	cpu, err := GetCPUInfo("testdata/cpuinfo")
	assert.Equal(t, cpu,
		"Intel(R) Core(TM) i3 CPU M 330 @ 2.13GHz")
	assert.NoError(t, err)

	cpu, err = GetCPUInfo("testdata/sw-cpuinfo")
	assert.Equal(t, cpu, "sw 1.40GHz")
	assert.NoError(t, err)

	cpu, err = GetCPUInfo("testdata/arm-cpuinfo")
	assert.Equal(t, cpu, "NANOPI2")
	assert.NoError(t, err)

	cpu, err = GetCPUInfo("testdata/hw_kirin-cpuinfo")
	assert.Equal(t, cpu, "HUAWEI Kirin 990")
	assert.NoError(t, err)
}

func TestMemInfo(t *testing.T) {
	mem, err := getMemoryFromFile("testdata/meminfo")
	assert.Equal(t, mem, uint64(4005441536))
	assert.NoError(t, err)
}

func TestVersion(t *testing.T) {
	lang := os.Getenv("LANGUAGE")
	os.Setenv("LANGUAGE", "en_US")
	defer os.Setenv("LANGUAGE", lang)

	deepin, err := getVersionFromDeepin("testdata/deepin-version")
	assert.Equal(t, deepin, "2015 Desktop Alpha1")
	assert.NoError(t, err)

	lsb, err := getVersionFromLSB("testdata/lsb-release")
	assert.Equal(t, lsb, "2014.3")
	assert.NoError(t, err)
}

func TestDistro(t *testing.T) {
	lang := os.Getenv("LANGUAGE")
	os.Setenv("LANGUAGE", "en_US")
	defer os.Setenv("LANGUAGE", lang)

	distroId, distroDesc, distroVer, err := getDistroFromLSB("testdata/lsb-release")
	assert.Equal(t, distroId, "Deepin")
	assert.Equal(t, distroDesc, "Deepin 2014.3")
	assert.Equal(t, distroVer, "2014.3")
	assert.NoError(t, err)
}

func TestSystemBit(t *testing.T) {
	v := systemBit()
	if v != "32" {
		assert.Equal(t, v, "64")
	}

	if v != "64" {
		assert.Equal(t, v, "32")
	}
}

func TestIsFloatEqual(t *testing.T) {
	assert.Equal(t, isFloatEqual(0.001, 0.0), false)
	assert.Equal(t, isFloatEqual(0.001, 0.001), true)
}

func TestParseInfoFile(t *testing.T) {
	v, err := parseInfoFile("testdata/lsb-release", "=")
	assert.NoError(t, err)
	assert.Equal(t, v["DISTRIB_ID"], "Deepin")
	assert.Equal(t, v["DISTRIB_RELEASE"], "2014.3")
	assert.Equal(t, v["DISTRIB_DESCRIPTION"], strconv.Quote("Deepin 2014.3"))
}

func TestGetCPUMaxMHzByLscpu(t *testing.T) {
	ret, err := parseInfoFile("testdata/lsCPU", ":")
	assert.NoError(t, err)
	v, err := getCPUMaxMHzByLscpu(ret)
	assert.NoError(t, err)
	assert.Equal(t, v, 3600.0000)
}

func TestGetProcessorByLscpuu(t *testing.T) {
	ret, err := parseInfoFile("testdata/lsCPU", ":")
	assert.NoError(t, err)
	v, err := getProcessorByLscpu(ret)
	assert.NoError(t, err)
	assert.Equal(t, v, "Intel(R) Core(TM) i5-4570 CPU @ 3.20GHz")
}

func TestDoReadCache(t *testing.T) {
	// 创建测试数据
	expectedInfo := &SystemInfo{
		Version:      "20 专业版",
		DistroID:     "uos",
		DistroDesc:   "UnionTech OS 20",
		DistroVer:    "20",
		Processor:    "Intel(R) Core(TM) i5-4570 CPU @ 3.20GHz",
		DiskCap:      uint64(500107862016),
		MemoryCap:    uint64(8280711168),
		SystemType:   int64(64),
		CPUMaxMHz:    float64(3600),
		CurrentSpeed: uint64(0),
	}

	// 创建临时缓存文件
	tmpFile := t.TempDir() + "/systeminfo.cache"
	err := doSaveCache(expectedInfo, tmpFile)
	assert.NoError(t, err)

	// 读取并验证
	ret, err := doReadCache(tmpFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedInfo.Version, ret.Version)
	assert.Equal(t, expectedInfo.DistroID, ret.DistroID)
	assert.Equal(t, expectedInfo.DistroDesc, ret.DistroDesc)
	assert.Equal(t, expectedInfo.DistroVer, ret.DistroVer)
	assert.Equal(t, expectedInfo.Processor, ret.Processor)
	assert.Equal(t, expectedInfo.DiskCap, ret.DiskCap)
	assert.Equal(t, expectedInfo.MemoryCap, ret.MemoryCap)
	assert.Equal(t, expectedInfo.SystemType, ret.SystemType)
	assert.Equal(t, expectedInfo.CPUMaxMHz, ret.CPUMaxMHz)
	assert.Equal(t, expectedInfo.CurrentSpeed, ret.CurrentSpeed)
}

func TestSupplementProcessorFrequency(t *testing.T) {
	// Test case 1: Processor already contains GHz frequency
	result := supplementProcessorFrequency("Intel(R) Core(TM) i5-4570 CPU @ 3.20GHz", 0, 0)
	assert.Equal(t, "Intel(R) Core(TM) i5-4570 CPU @ 3.20GHz", result)

	// Test case 2: Processor already contains MHz frequency
	result = supplementProcessorFrequency("Intel Xeon 1400MHz", 0, 0)
	assert.Equal(t, "Intel Xeon 1400MHz", result)

	// Test case 3: Processor already contains GHz without @ symbol
	result = supplementProcessorFrequency("sw 1.40GHz", 0, 0)
	assert.Equal(t, "sw 1.40GHz", result)

	// Test case 4: Processor already contains frequency with space
	result = supplementProcessorFrequency("AMD Ryzen 3.5 GHz", 0, 0)
	assert.Equal(t, "AMD Ryzen 3.5 GHz", result)

	// Test case 5: Add frequency from CurrentSpeed (priority)
	result = supplementProcessorFrequency("Intel Core i7", 2300, 3600)
	assert.Equal(t, "Intel Core i7 @ 2.30GHz", result)

	// Test case 6: Add frequency from CPUMaxMHz (fallback)
	result = supplementProcessorFrequency("Intel Core i7", 0, 3600)
	assert.Equal(t, "Intel Core i7 @ 3.60GHz", result)

	// Test case 7: Empty processor string
	result = supplementProcessorFrequency("", 2300, 3600)
	assert.Equal(t, "", result)

	// Test case 8: No frequency information available
	result = supplementProcessorFrequency("Unknown CPU", 0, 0)
	assert.Equal(t, "Unknown CPU", result)

	// Test case 9: Processor with THz frequency
	result = supplementProcessorFrequency("Future CPU 2.5THz", 0, 0)
	assert.Equal(t, "Future CPU 2.5THz", result)

	// Test case 10: CPUMaxMHz is very small (near zero)
	result = supplementProcessorFrequency("Low Power CPU", 0, 0.0000001)
	assert.Equal(t, "Low Power CPU", result)

	// Test case 11: Add frequency with decimal CurrentSpeed
	result = supplementProcessorFrequency("ARM Cortex", 1500, 0)
	assert.Equal(t, "ARM Cortex @ 1.50GHz", result)
}
