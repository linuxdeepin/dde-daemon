// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"errors"
	"time"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	dbusServiceName = "org.deepin.dde.Power1"
	dbusPath        = "/org/deepin/dde/Power1"
	dbusInterface   = dbusServiceName
)

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) GetBatteries() (batteries []dbus.ObjectPath, busErr *dbus.Error) {
	m.batteriesMu.Lock()

	batteries = make([]dbus.ObjectPath, len(m.batteries))
	idx := 0
	for _, bat := range m.batteries {
		batteries[idx] = bat.getObjPath()
		idx++
	}

	m.batteriesMu.Unlock()
	return batteries, nil
}

func (m *Manager) refreshBatteries() {
	logger.Debug("RefreshBatteries")
	m.batteriesMu.Lock()
	for _, bat := range m.batteries {
		bat.Refresh()
	}
	m.batteriesMu.Unlock()
}

func (m *Manager) RefreshBatteries() *dbus.Error {
	m.refreshBatteries()
	return nil
}

func (m *Manager) RefreshMains() *dbus.Error {
	logger.Debug("RefreshMains")
	if m.ac == nil {
		return nil
	}

	device := m.ac.newDevice()
	if device == nil {
		logger.Warning("RefreshMains: ac.newDevice failed")
		return nil
	}
	m.refreshAC(device)
	return nil
}

func (m *Manager) Refresh() *dbus.Error {
	err := m.RefreshMains()
	if err != nil {
		return err
	}
	err = m.RefreshBatteries()
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) SetCpuBoost(enabled bool) *dbus.Error {
	err := m.cpus.SetBoostEnabled(enabled)
	if err == nil {
		m.setPropCpuBoost(enabled)
	}
	return dbusutil.ToError(err)
}

func (m *Manager) SetCpuGovernor(governor string) *dbus.Error {
	err := m.cpus.SetGovernor(governor, false)
	if err == nil {
		m.setPropCpuGovernor(governor)
	}
	return dbusutil.ToError(err)
}

func (m *Manager) SetMode(mode string) *dbus.Error {
	if m.Mode == mode {
		return dbusutil.ToError(errors.New("Repeat switch"))
	}

	m.setPropPowerSavingModeAutoWhenBatteryLow(false)
	m.setPropPowerSavingModeAuto(false)

	err := m.doSetMode(mode)
	if err == nil {
		err = m.saveConfig()
	}

	if err != nil {
		logger.Warning(err)
	}

	return dbusutil.ToError(err)
}

func (m *Manager) LockCpuFreq(governor string, lockTime int32) *dbus.Error {

	currentGovernor, err := m.cpus.GetGovernor()
	if err != nil {
		return dbusutil.ToError(err)
	}

	// change cpu governor
	if governor != currentGovernor {
		err = m.cpus.SetGovernor(governor, false)
		if err != nil {
			return dbusutil.ToError(err)
		}

		time.AfterFunc(time.Second*time.Duration(lockTime), func() {
			err := m.cpus.SetGovernor(currentGovernor, false)
			if err != nil {
				logger.Warningf("rewrite cpu scaling_governor file failed:%v", err)
			}
		})
	}

	return nil
}
