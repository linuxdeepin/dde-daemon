/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package power

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetAvailableGovernors(t *testing.T) {
	expect := map[string]bool{
		"performance": true,
		"powersave":   true,
	}

	cpu := CpuHandler{}
	_, err := cpu.GetAvailableGovernors(false)
	assert.Nil(t, err)

	cpu.path = "./testdata1"
	_, err = cpu.GetAvailableGovernors(true)
	assert.NotNil(t, err)

	cpu.path = "./testdata"
	availableGovernors, err := cpu.GetAvailableGovernors(true)
	assert.Equal(t, expect["performance"], availableGovernors["performance"])
	assert.Equal(t, expect["powersave"], availableGovernors["powersave"])
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
	err := cpu.SetGovernor("scaling_governor")
	assert.NotNil(t, err)

	cpu.availableGovernors = map[string]bool{
		"scaling_governor": true,
	}
	cpu.path = "./testdata/setGovernor"
	err = cpu.SetGovernor("scaling_governor")
	assert.Nil(t, err)

	cpu.path = "./testdata/setGovernor1"
	err = cpu.SetGovernor("scaling_governor")
	assert.NotNil(t, err)
}

func Test_GetAvailableGovernors1(t *testing.T) {
	cpus := CpuHandlers{}
	cpus.GetAvailableGovernors()

	expect := map[string]bool{
		"performance": true,
		"powersave":   true,
	}

	availableGovernors1 := map[string]bool{
		"performance": true,
		"powersave":   true,
		"invalid":     false,
	}

	availableGovernors2 := map[string]bool{
		"performance": true,
		"powersave":   true,
	}

	cpu1 := CpuHandler{
		path:               "./testdata",
		availableGovernors: availableGovernors1,
		governor:           "scaling_governor",
	}

	cpu2 := CpuHandler{
		path:               "./testdata",
		availableGovernors: availableGovernors2,
		governor:           "scaling_governor",
	}
	cpus = CpuHandlers{
		cpu1,
		cpu2,
	}
	availableGovernors, err := cpus.GetAvailableGovernors()
	assert.Nil(t, err)
	assert.Equal(t, expect["performance"], availableGovernors["performance"])
	assert.Equal(t, expect["powersave"], availableGovernors["powersave"])
}

func Test_GetGovernor1(t *testing.T) {
	cpus := CpuHandlers{}
	cpus.GetGovernor()

	expectGovernor := "performance"
	availableGovernors1 := map[string]bool{
		"performance": true,
		"powersave":   true,
		"invalid":     false,
	}

	availableGovernors2 := map[string]bool{
		"performance": true,
		"powersave":   true,
	}

	cpu1 := CpuHandler{
		path:               "./testdata",
		availableGovernors: availableGovernors1,
		governor:           "scaling_governor",
	}

	cpu2 := CpuHandler{
		path:               "./testdata",
		availableGovernors: availableGovernors2,
		governor:           "scaling_governor",
	}
	cpus = CpuHandlers{
		cpu1,
		cpu2,
	}

	governor, err := cpus.GetGovernor()
	assert.Equal(t, expectGovernor, governor)
	assert.Nil(t, err)

	cpu2 = CpuHandler{
		path:               "./setGovernor2",
		availableGovernors: availableGovernors2,
		governor:           "scaling_governor",
	}
	cpus = CpuHandlers{
		cpu1,
		cpu2,
	}
	_, err = cpus.GetGovernor()
	assert.NotNil(t, err)
}

func Test_SetGovernor1(t *testing.T) {
	availableGovernors1 := map[string]bool{
		"performance": true,
		"powersave":   true,
		"invalid":     false,
	}

	availableGovernors2 := map[string]bool{
		"performance": true,
		"powersave":   true,
	}

	cpu1 := CpuHandler{
		path:               "./testdata/setGovernor",
		availableGovernors: availableGovernors1,
		governor:           "scaling_governor",
	}

	cpu2 := CpuHandler{
		path:               "./testdata/setGovernor",
		availableGovernors: availableGovernors2,
		governor:           "scaling_governor",
	}
	cpus := CpuHandlers{
		cpu1,
		cpu2,
	}

	err := cpus.SetGovernor("performance")
	assert.Nil(t, err)

	err = cpus.SetGovernor("scaling_governor")
	assert.NotNil(t, err)
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
