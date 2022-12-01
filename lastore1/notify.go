/*
 * Copyright (C) 2015 ~ 2017 Deepin Technology Co., Ltd.
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

package lastore

import (
	"fmt"
	"os"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/gettext"
)

type NotifyAction struct {
	Id       string
	Name     string
	Callback func()
}

func getAppStoreAppName() string {
	_, err := os.Stat("/usr/share/applications/deepin-app-store.desktop")
	if err == nil {
		return "deepin-app-store"
	}
	return "deepin-appstore"
}

const (
	notifyExpireTimeoutDefault  = -1
	notifyExpireTimeoutReboot   = 30 * 1000
	systemUpdatedIcon           = "system-updated"
	notifyActKeyRebootNow       = "RebootNow"
	notifyActKeyRebootLater     = "Later"
	notifyActKeyReboot10Minutes = "10Min"
	notifyActKeyReboot30Minutes = "30Min"
	notifyActKeyReboot2Hours    = "2Hr"
	notifyActKeyReboot6Hours    = "6Hr"
)

func (l *Lastore) sendNotify(icon string, summary string, msg string, actions []NotifyAction,
	hints map[string]dbus.Variant, expireTimeout int32, appName string) {
	logger.Infof("sendNotify icon: %s, msg: %q, actions: %v, timeout: %d",
		icon, msg, actions, expireTimeout)
	n := l.notifications
	var as []string
	for _, action := range actions {
		as = append(as, action.Id, action.Name)
	}

	if icon == "" {
		icon = "deepin-appstore"
	}
	id, err := n.Notify(0, appName, 0, icon, summary,
		msg, as, hints, expireTimeout)
	if err != nil {
		logger.Warningf("Notify failed: %q: %v\n", msg, err)
		return
	}

	if len(actions) == 0 {
		return
	}

	hid, err := l.notifications.ConnectActionInvoked(
		func(id0 uint32, actionId string) {
			logger.Debugf("notification action invoked id: %d, actionId: %q",
				id0, actionId)
			if id != id0 {
				return
			}

			for _, action := range actions {
				if action.Id == actionId && action.Callback != nil {
					action.Callback()
					return
				}
			}
			logger.Warningf("not found action id %q in %v", actionId, actions)
		})
	if err != nil {
		logger.Warning(err)
		return
	}
	logger.Debugf("notifyIdHidMap[%d]=%d", id, hid)
	l.notifyIdHidMap[id] = hid
}

// NotifyInstall send desktop notify for install job
func (l *Lastore) notifyInstall(pkgId string, succeed bool, ac []NotifyAction) {
	var msg string
	if succeed {
		msg = fmt.Sprintf(gettext.Tr("%q installed successfully."), pkgId)
		l.sendNotify("package_install_succeed", "", msg, ac, nil, notifyExpireTimeoutDefault, getAppStoreAppName())
	} else {
		msg = fmt.Sprintf(gettext.Tr("%q failed to install."), pkgId)
		l.sendNotify("package_install_failed", "", msg, ac, nil, notifyExpireTimeoutDefault, getAppStoreAppName())
	}
}

func (l *Lastore) notifyRemove(pkgId string, succeed bool, ac []NotifyAction) {
	var msg string
	if succeed {
		msg = gettext.Tr("Removed successfully")
	} else {
		msg = gettext.Tr("Failed to remove the app")
	}
	l.sendNotify("deepin-appstore", "", msg, ac, nil, notifyExpireTimeoutDefault, getAppStoreAppName())
}

//NotifyLowPower send notify for low power
func (l *Lastore) notifyLowPower() {
	msg := gettext.Tr("In order to prevent automatic shutdown, please plug in for normal update.")
	l.sendNotify("notification-battery_low", "", msg, nil, nil, notifyExpireTimeoutDefault, getAppStoreAppName())
}

func (l *Lastore) notifyAutoClean() {
	msg := gettext.Tr("Package cache wiped")
	l.sendNotify("deepin-appstore", "", msg, nil, nil, notifyExpireTimeoutDefault, "dde-control-center")
}

func (l *Lastore) notifyUpdateSource(actions []NotifyAction) {
	msg := gettext.Tr("Updates Available")
	l.sendNotify("preferences-system", "", msg, actions, nil, notifyExpireTimeoutDefault, "dde-control-center")
}

func (l *Lastore) updateSucceedNotify(actions []NotifyAction) {
	summary := gettext.Tr("Reboot after Updates")
	msg := gettext.Tr("Restart the computer to use the system and applications properly")
	hints := map[string]dbus.Variant{"x-deepin-action-RebootNow": dbus.MakeVariant("busctl,--user,call,org.deepin.dde.SessionManager1," +
		"/org/deepin/dde/SessionManager1,org.deepin.dde.SessionManager1,RequestReboot")}
	l.sendNotify(systemUpdatedIcon, summary, msg, actions, hints, notifyExpireTimeoutReboot, "dde-control-center")

	// 默认弹出横幅时间为每2小时
	l.resetUpdateSucceedNotifyTimer(l.intervalTime)
}

func (l *Lastore) lowBatteryInUpdatingNotify() {
	msg := gettext.Tr("Your system is being updated, but the capacity is lower than 50%, please plug in to avoid power outage")
	actions := []string{"ok", gettext.Tr("OK")}
	notifyID, err := l.notifications.Notify(0,
		"dde-control-center",
		0,
		systemUpdatedIcon,
		"",
		msg,
		actions,
		nil,
		0,
	)

	if err != nil {
		logger.Warning("failed to send notify:", err)
		return
	}
	l.updateNotifyId = notifyID
}

func (l *Lastore) removeLowBatteryInUpdatingNotify() {
	err := l.notifications.CloseNotification(0, l.updateNotifyId)
	if err != nil {
		logger.Warningf("close low battery in updating notification failed,err:%v", err)
	}
	l.updateNotifyId = 0
}
