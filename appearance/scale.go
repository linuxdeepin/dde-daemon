// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"sync/atomic"

	"github.com/godbus/dbus"
	notifications "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.notifications"
	"github.com/linuxdeepin/go-lib/gettext"
)

func (m *Manager) getScaleFactor() float64 {
	scaleFactor, err := m.xSettings.GetScaleFactor(dbus.FlagNoAutoStart)
	if err != nil {
		logger.Warning("failed to get scale factor:", err)
		scaleFactor = 1.0
	}
	return scaleFactor
}

func (m *Manager) setScaleFactor(scale float64) error {
	err := m.xSettings.SetScaleFactor(dbus.FlagNoAutoStart, scale)
	if err != nil {
		logger.Warning("failed to set scale factor:", err)
	}
	return err
}

var notifyId uint32

const icon = "dialog-window-scale"

func handleSetScaleFactorDone() {
	const (
		expireTimeout = 15 * 1000
		requestLogout = "dbus-send,--type=method_call,--dest=com.deepin.SessionManager,/com/deepin/SessionManager,com.deepin.SessionManager.RequestLogout"
	)
	body := gettext.Tr("Log out for display scaling settings to take effect")
	summary := gettext.Tr("Set successfully")
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}
	nid := atomic.LoadUint32(&notifyId)
	notifier := notifications.NewNotifications(sessionBus)
	nid, err = notifier.Notify(0, "dde-control-center", nid,
		icon, summary, body,
		[]string{"_logout", gettext.Tr("Log Out Now"), "_later", gettext.Tr("Later")},
		map[string]dbus.Variant{
			"x-deepin-action-_logout": dbus.MakeVariant(requestLogout),
			"x-deepin-action-_later":  dbus.MakeVariant(""),
		}, expireTimeout)
	if err != nil {
		logger.Warning(err)
	} else {
		atomic.StoreUint32(&notifyId, nid)
	}

}

func handleSetScaleFactorStarted() {
	body := gettext.Tr("Setting display scaling")
	summary := gettext.Tr("Display scaling")
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}
	nid := atomic.LoadUint32(&notifyId)
	notifier := notifications.NewNotifications(sessionBus)
	nid, err = notifier.Notify(0, "dde-control-center", nid,
		icon, summary, body,
		nil, nil, 0)
	if err != nil {
		logger.Warning(err)
	} else {
		atomic.StoreUint32(&notifyId, nid)
	}

}

func (m *Manager) setScreenScaleFactors(factors map[string]float64) error {
	return m.xSettings.SetScreenScaleFactors(dbus.FlagNoAutoStart, factors)
}

func (m *Manager) getScreenScaleFactors() (map[string]float64, error) {
	return m.xSettings.GetScreenScaleFactors(dbus.FlagNoAutoStart)
}
