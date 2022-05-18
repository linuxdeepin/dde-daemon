/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
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

package bluetooth1

import (
	btcommon "github.com/linuxdeepin/dde-daemon/common/bluetooth"
)

const (
	notifyIconBluetoothConnected     = "notification-bluetooth-connected"
	notifyIconBluetoothDisconnected  = "notification-bluetooth-disconnected"
	notifyIconBluetoothConnectFailed = "notification-bluetooth-error"
)

func notify(icon string, summary, body *btcommon.LocalizeStr) {
	logger.Info("notify", icon, summary, body)

	args := marshalJSON(btcommon.NotifyMsg{
		Icon:    icon,
		Summary: summary,
		Body:    body,
	})
	ua := _bt.getActiveUserAgent()
	if ua == nil {
		logger.Debug("ua is nil")
		return
	}
	err := ua.SendNotify(0, args)
	if err != nil {
		logger.Warning("send notify err:", err)
	}
}

// Tr 真正的翻译在session级别的模块
func Tr(msgId string) string {
	return msgId
}

func notifyConnected(alias string) {
	notify(notifyIconBluetoothConnected, nil, &btcommon.LocalizeStr{
		Format: Tr("Connect %q successfully"),
		Args:   []string{alias},
	})
}

func notifyDisconnected(alias string) {
	notify(notifyIconBluetoothDisconnected, nil, &btcommon.LocalizeStr{
		Format: Tr("%q disconnected"),
		Args:   []string{alias},
	})
}

func notifyConnectFailedHostDown(alias string) {
	format := Tr("Make sure %q is turned on and in range")
	notifyConnectFailedAux(alias, format)
}

func notifyConnectFailedAux(alias, format string) {
	notify(notifyIconBluetoothConnectFailed,
		&btcommon.LocalizeStr{ // summary
			Format: Tr("Bluetooth connection failed"),
		}, &btcommon.LocalizeStr{ // body
			Format: format,
			Args:   []string{alias},
		})
}

func notifyConnectFailedResourceUnavailable(devAlias, adapterAlias string) {
	notify(notifyIconBluetoothConnectFailed, &btcommon.LocalizeStr{ // summary
		Format: Tr("Bluetooth connection failed"),
	}, &btcommon.LocalizeStr{ //body
		Format: Tr("%q can no longer connect to %q. Try to forget this device and pair it again."),
		Args:   []string{adapterAlias, devAlias},
	})
}
