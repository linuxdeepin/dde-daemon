// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices1

import (
	"errors"

	"github.com/godbus/dbus/v5"
	polkit "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.policykit1"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (m *InputDevices) SetWakeupDevices(sender dbus.Sender, path string, value string) *dbus.Error {
	err := m.setWakeupDevices(path, value)
	return dbusutil.ToError(err)
}

// checkAuthorization verifies that the caller identified by sysBusName is
// allowed to perform the polkit action identified by actionId. It mirrors
// the pattern used by system/airplane_mode1 so that active local users are
// allowed without an authentication dialog (allow_active: yes).
func checkAuthorization(actionId string, sysBusName string) error {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	authority := polkit.NewAuthority(systemBus)
	subject := polkit.MakeSubject(polkit.SubjectKindSystemBusName)
	subject.SetDetail("name", sysBusName)

	ret, err := authority.CheckAuthorization(0, subject, actionId,
		nil, polkit.CheckAuthorizationFlagsAllowUserInteraction, "")
	if err != nil {
		return err
	}
	if !ret.IsAuthorized {
		return errors.New("not authorized")
	}
	return nil
}
