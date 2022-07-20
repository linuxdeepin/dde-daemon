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
	"os/exec"

	"github.com/godbus/dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

//go:generate dbusutil-gen em -type Manager
type Manager struct {
	service  *dbusutil.Service
	objLogin login1.Manager

	VirtualMachineName string
}

func newManager(service *dbusutil.Service) (*Manager, error) {
	m := &Manager{
		service: service,
	}
	err := m.init()
	if err != nil {
		return nil, err
	}

	name, err := detectVirtualMachine()
	if err != nil {
		logger.Warning(err)
	}

	m.setPropVirtualMachineName(name)

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
	// 虚拟机屏蔽待机
	if m.VirtualMachineName != "" {
		return false, nil
	}
	_, err := os.Stat("/sys/power/mem_sleep")
	if os.IsNotExist(err) {
		return false, nil
	}

	str, _ := m.objLogin.CanSuspend(0)
	return str == "yes", nil
}

func (m *Manager) CanHibernate() (can bool, busErr *dbus.Error) {
	// 虚拟机屏蔽休眠
	if m.VirtualMachineName != "" {
		return false, nil
	}
	str, _ := m.objLogin.CanHibernate(0)
	return str == "yes", nil
}

var autoConfigTargets = []string{
	"suspend.target",
	"sleep.target",
	"suspend-then-hibernate.target",
	"hibernate.target",
	"hybrid-sleep.target",
}

func (m *Manager) maskOnVM(enable bool) {
	var oper string
	if enable && m.VirtualMachineName != "" {
		oper = "mask"
	} else {
		oper = "unmask"
	}

	for _, target := range autoConfigTargets {
		logger.Debug("auto mask on virt")
		exec.Command("systemctl", oper, target).Run()
	}
}
