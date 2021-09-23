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

package power_manager

import (
	"os"

	"github.com/godbus/dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"pkg.deepin.io/lib/dbusutil"
)

//go:generate dbusutil-gen em -type Manager
type Manager struct {
	service  *dbusutil.Service
	objLogin login1.Manager
}

func newManager(service *dbusutil.Service) (*Manager, error) {
	m := &Manager{
		service: service,
	}
	err := m.init()
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) init() error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return err
	}

	m.objLogin = login1.NewManager(sysBus)
	return nil
}

func (m *Manager) CanShutdown() (can bool, busErr *dbus.Error) {
	str, _ := m.objLogin.CanPowerOff(0)
	return str == "yes", nil
}

func (m *Manager) CanReboot() (can bool, busErr *dbus.Error) {
	str, _ := m.objLogin.CanReboot(0)
	return str == "yes", nil
}

func (m *Manager) CanSuspend() (can bool, busErr *dbus.Error) {
	_, err := os.Stat("/sys/power/mem_sleep")
	if os.IsNotExist(err) {
		return false, nil
	}

	str, _ := m.objLogin.CanSuspend(0)
	return str == "yes", nil
}

func (m *Manager) CanHibernate() (can bool, busErr *dbus.Error) {
	str, _ := m.objLogin.CanHibernate(0)
	return str == "yes", nil
}

func (m *Manager) CanSuspendToHibernate() (can bool, busErr *dbus.Error) {

	if !canSuspendToHibernate() {
		logger.Debug("The system does not support Suspend To Hibernate")
		return false, nil
	}

	return true, nil
}

func (m *Manager) SetSuspendToHibernateTime(timeMinute int32) *dbus.Error {

	err := setSuspendToHibernateTime(timeMinute)
	if err != nil {
		logger.Debug("Set Suspend To HibernateTime fail", err)
		return dbusutil.ToError(err)
	}

	return nil
}
