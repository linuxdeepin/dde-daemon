// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"sync"

	"github.com/linuxdeepin/dde-daemon/loader"

	_ "github.com/linuxdeepin/dde-daemon/appearance"
	_ "github.com/linuxdeepin/dde-daemon/audio"
	_ "github.com/linuxdeepin/dde-daemon/bluetooth"
	_ "github.com/linuxdeepin/dde-daemon/screenedge"

	_ "github.com/linuxdeepin/dde-daemon/mime"

	// depends: network
	_ "github.com/linuxdeepin/dde-daemon/systeminfo"

	_ "github.com/linuxdeepin/dde-daemon/calltrace"
	_ "github.com/linuxdeepin/dde-daemon/clipboard"
	_ "github.com/linuxdeepin/dde-daemon/debug"
	_ "github.com/linuxdeepin/dde-daemon/dock"
	_ "github.com/linuxdeepin/dde-daemon/gesture"
	_ "github.com/linuxdeepin/dde-daemon/housekeeping"
	_ "github.com/linuxdeepin/dde-daemon/inputdevices"
	_ "github.com/linuxdeepin/dde-daemon/keybinding"
	_ "github.com/linuxdeepin/dde-daemon/lastore"
	_ "github.com/linuxdeepin/dde-daemon/launcher"
	_ "github.com/linuxdeepin/dde-daemon/mime"
	_ "github.com/linuxdeepin/dde-daemon/network"
	_ "github.com/linuxdeepin/dde-daemon/screenedge"
	_ "github.com/linuxdeepin/dde-daemon/screensaver"
	_ "github.com/linuxdeepin/dde-daemon/service_trigger"
	_ "github.com/linuxdeepin/dde-daemon/session/eventlog"
	_ "github.com/linuxdeepin/dde-daemon/session/power"
	_ "github.com/linuxdeepin/dde-daemon/session/uadpagent"
	_ "github.com/linuxdeepin/dde-daemon/sessionwatcher"
	_ "github.com/linuxdeepin/dde-daemon/systeminfo"
	_ "github.com/linuxdeepin/dde-daemon/timedate"
	_ "github.com/linuxdeepin/dde-daemon/trayicon"
	_ "github.com/linuxdeepin/dde-daemon/x_event_monitor"
)

var (
	moduleLocker sync.Mutex
)

func (s *SessionDaemon) checkDependencies(module loader.Module, enabled bool) error {
	if enabled {
		depends := module.GetDependencies()
		for _, n := range depends {
			if !s.getConfigValue(n) {
				return fmt.Errorf("Dependency lose: %v", n)
			}
		}
		return nil
	}

	for _, m := range loader.List() {
		if m == nil || m.Name() == module.Name() {
			continue
		}

		if m.IsEnable() && isStrInList(module.Name(), m.GetDependencies()) {
			return fmt.Errorf("Can not disable this module '%s', because of it was depended by'%s'",
				module.Name(), m.Name())
		}
	}
	return nil
}

func isStrInList(item string, list []string) bool {
	for _, v := range list {
		if item == v {
			return true
		}
	}
	return false
}
