// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_IsPstate(t *testing.T) {
	driverData, _ := os.ReadFile("./testdata/scaling_driver_intel_pstate")
	driver := string(driverData)
	assert.Equal(t, driverHasPstate(driver), true)

	driverData, _ = os.ReadFile("./testdata/scaling_driver_amd_pstate")
	driver = string(driverData)
	assert.Equal(t, driverHasPstate(driver), true)

	driverData, _ = os.ReadFile("./testdata/scaling_driver_intel_cpufreq")
	driver = string(driverData)
	assert.Equal(t, driverHasPstate(driver), false)
}

func Test_GetGovernor(t *testing.T) {
	expectGovernor := "performance"
	cpu := CpuHandler{}
	_, err := cpu.GetGovernor(false)
	assert.Nil(t, err)

	cpu.path = "./testdata1"
	_, err = cpu.GetGovernor(true)
	assert.NotNil(t, err)

	cpu.path = "./testdata"
	governor, err := cpu.GetGovernor(true)
	assert.Nil(t, err)
	assert.Equal(t, expectGovernor, governor)
}

func Test_SetGovernor(t *testing.T) {
	cpu := CpuHandler{}
	cpu.path = "./testdata/setGovernor"
	err := cpu.SetGovernor("scaling_governor", false)
	assert.Nil(t, err)

	cpu.path = "./testdata/setGovernor1"
	err = cpu.SetGovernor("scaling_governor", false)
	assert.NotNil(t, err)
}

func Test_GetGovernor1(t *testing.T) {
	cpus := CpuHandlers{}
	cpus.GetGovernor()

	expectGovernor := "performance"
	cpu1 := CpuHandler{
		path:     "./testdata",
		governor: "scaling_governor",
	}

	cpu2 := CpuHandler{
		path:     "./testdata",
		governor: "scaling_governor",
	}
	cpus = CpuHandlers{
		cpu1,
		cpu2,
	}

	governor, err := cpus.GetGovernor()
	assert.Equal(t, expectGovernor, governor)
	assert.Nil(t, err)

	cpu2 = CpuHandler{
		path:     "./setGovernor2",
		governor: "scaling_governor",
	}
	cpus = CpuHandlers{
		cpu1,
		cpu2,
	}
	_, err = cpus.GetGovernor()
	assert.NotNil(t, err)
}

func Test_SetGovernor1(t *testing.T) {
	cpu1 := CpuHandler{
		path:     "./testdata/setGovernor",
		governor: "scaling_governor",
	}

	cpu2 := CpuHandler{
		path:     "./testdata/setGovernor",
		governor: "scaling_governor",
	}
	cpus := CpuHandlers{
		cpu1,
		cpu2,
	}

	err := cpus.SetGovernor("performance", false)
	assert.Nil(t, err)

	err = cpus.SetGovernor("scaling_governor", false)
	assert.Nil(t, err)
}

func Test_getScalingAvailableGovernors(t *testing.T) {
	cpuGovernors := getScalingAvailableGovernors()
	assert.NotEqual(t, len(cpuGovernors), 0)
}

func Test_getScalingBalanceAvailableGovernors(t *testing.T) {
	cpuGovernors := getScalingBalanceAvailableGovernors()
	assert.NotEqual(t, len(cpuGovernors), 0)
	assert.Equal(t, len(cpuGovernors), 4)
}

func Test_getSupportGovernors(t *testing.T) {
	cpuGovernors := getSupportGovernors()
	assert.Equal(t, len(cpuGovernors), 0)
}

func Test_getLocalAvailableGovernors(t *testing.T) {
	cpuGovernors := getLocalAvailableGovernors()
	assert.NotEqual(t, len(cpuGovernors), 10)
}

func Test_isSystemSupportMode(t *testing.T) {
	assert.Equal(t, isSystemSupportMode("powersave"), false)
}

func Test_setLocalAvailableGovernors(t *testing.T) {
	var governors []string = []string{"aaaa", "bbbb", "cccc"}
	setLocalAvailableGovernors(governors)
	cpuGovernors := getLocalAvailableGovernors()
	assert.Equal(t, len(cpuGovernors), 3)
	assert.Equal(t, cpuGovernors[0], governors[0])
	assert.Equal(t, cpuGovernors[1], governors[1])
	assert.Equal(t, cpuGovernors[2], governors[2])
}

func Test_setSupportGovernors(t *testing.T) {
	var lines = []string{"1", "2", "3"}
	assert.NotEqual(t, len(setSupportGovernors(lines)), 10)
}

func Test_getAvailableGovernors(t *testing.T) {
	cpus := CpuHandlers{}
	govs, _ := cpus.getAvailableGovernors(true)
	assert.NotEqual(t, govs, "/a/b/c/d")
}

func Test_getAvailableArrGovernors(t *testing.T) {
	cpus := CpuHandlers{}
	govs := cpus.getAvailableArrGovernors(true)
	assert.NotEqual(t, len(govs), 10)
}

func Test_trySetBalanceCpuGovernor(t *testing.T) {
	err, target := trySetBalanceCpuGovernor("powersave")
	assert.NotEqual(t, err, nil)
	assert.NotEqual(t, target, "powersave")
}

func Test_getCpuGovernorPath(t *testing.T) {
	cpus := CpuHandlers{}

	assert.NotEqual(t, cpus.getCpuGovernorPath(false), "/a/b/c/d")
}

func Test_IsBoostFileExist(t *testing.T) {
	cpus := CpuHandlers{}

	cpus.IsBoostFileExist()
}

func Test_SetBoostEnabled(t *testing.T) {
	cpus := CpuHandlers{}

	err := cpus.SetBoostEnabled(false)
	assert.NotNil(t, err)

	err = cpus.SetBoostEnabled(false)
	assert.NotNil(t, err)
}

func Test_SimpleFunc(t *testing.T) {
	b := Battery{}

	b.GetInterfaceName()
	b.getObjPath()
	b.setRefreshDoneCallback(func() {
	})
	b.resetUpdateInterval(1)
	b.startLoopUpdate(1)
	b.destroy()
	b.destroy()
	b.GetExportedMethods()

	m := Manager{}
	m.GetExportedMethods()
}
