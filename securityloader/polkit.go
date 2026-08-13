// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package securityloader

import (
	"errors"

	"github.com/godbus/dbus/v5"
	polkit "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.policykit1"
)

// CheckPolkitAuth checks whether the caller on the given system bus connection
// is authorized for the specified Polkit action. Returns true if authorized,
// false if denied, and any error encountered.
//
// This is a shared helper that replaces duplicated per-package auth functions.
// Callers should pass their existing service.Conn() to avoid re-establishing
// a system bus connection on every call.
func CheckPolkitAuth(conn *dbus.Conn, sysBusName, actionID string) (bool, error) {
	authority := polkit.NewAuthority(conn)
	subject := polkit.MakeSubject(polkit.SubjectKindSystemBusName)
	subject.SetDetail("name", sysBusName)
	result, err := authority.CheckAuthorization(0, subject, actionID,
		nil, polkit.CheckAuthorizationFlagsAllowUserInteraction, "")
	if err != nil {
		return false, err
	}
	return result.IsAuthorized, nil
}

// AuthorizeWithPolkit applies the caller registry and falls back to Polkit
// only when the caller is not registered by security-loader.
func AuthorizeWithPolkit(registry *AllowCallerRegistry, scope string, sender dbus.Sender, conn *dbus.Conn, actionID string) error {
	result, err := registry.Authorize(scope, sender)
	switch result {
	case AuthError:
		return err
	case AuthPolkit:
		ok, err := CheckPolkitAuth(conn, string(sender), actionID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("access denied")
		}
		return nil
	case AuthOK:
		return nil
	default:
		return errors.New("unknown security-loader authorization result")
	}
}
