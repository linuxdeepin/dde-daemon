/*
 * Copyright (C) 2013 ~ 2018 Deepin Technology Co., Ltd.
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

package main

import (
	"fmt"
	"sync"

	"github.com/linuxdeepin/dde-daemon/loader"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/gsettings"

	_ "github.com/linuxdeepin/dde-daemon/audio1"
	_ "github.com/linuxdeepin/dde-daemon/bluetooth1"
	_ "github.com/linuxdeepin/dde-daemon/screenedge1"

	// depends: network
	_ "github.com/linuxdeepin/dde-daemon/calltrace"
	_ "github.com/linuxdeepin/dde-daemon/clipboard1"
	_ "github.com/linuxdeepin/dde-daemon/debug"

	// _ "github.com/linuxdeepin/dde-daemon/dock" // 转移到am
	_ "github.com/linuxdeepin/dde-daemon/gesture"
	_ "github.com/linuxdeepin/dde-daemon/housekeeping"
	_ "github.com/linuxdeepin/dde-daemon/inputdevices"
	_ "github.com/linuxdeepin/dde-daemon/keybinding"
	_ "github.com/linuxdeepin/dde-daemon/lastore1"

	// _ "github.com/linuxdeepin/dde-daemon/launcher" // // 转移到am
	_ "github.com/linuxdeepin/dde-daemon/network"
	_ "github.com/linuxdeepin/dde-daemon/screensaver1"
	_ "github.com/linuxdeepin/dde-daemon/service_trigger"
	_ "github.com/linuxdeepin/dde-daemon/session/power1"
	_ "github.com/linuxdeepin/dde-daemon/session/uadpagent1"
	_ "github.com/linuxdeepin/dde-daemon/sessionwatcher1"
	_ "github.com/linuxdeepin/dde-daemon/systeminfo1"
	_ "github.com/linuxdeepin/dde-daemon/timedate1"
	_ "github.com/linuxdeepin/dde-daemon/trayicon1"
	_ "github.com/linuxdeepin/dde-daemon/x_event_monitor1"
)

var (
	moduleLocker   sync.Mutex
	daemonSchema   = "com.deepin.dde.daemon"
	daemonSettings = gio.NewSettings(daemonSchema)
)

func listenDaemonSettings() {
	gsettings.ConnectChanged(daemonSchema, "*", func(name string) {
		// gsettings key names must keep consistent with module names
		moduleLocker.Lock()
		defer moduleLocker.Unlock()
		module := loader.GetModule(name)
		if module == nil {
			logger.Error("Invalid module name:", name)
			return
		}

		enable := daemonSettings.GetBoolean(name)
		err := checkDependencies(daemonSettings, module, enable)
		if err != nil {
			logger.Error(err)
			return
		}

		err = module.Enable(enable)
		if err != nil {
			logger.Warningf("Enable '%s' failed: %v", name, err)
			return
		}
	})
}

func checkDependencies(s *gio.Settings, module loader.Module, enabled bool) error {
	if enabled {
		depends := module.GetDependencies()
		for _, n := range depends {
			if !s.GetBoolean(n) {
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
