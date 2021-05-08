/*
 * Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:     zsien <i@zsien.cn>
 *
 * Maintainer: zsien <i@zsien.cn>
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
package inputdevices

import (
	"strconv"
	"sync"

	"github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
)

//go:generate dbusutil-gen -type InputDevices inputdevices.go
//go:generate dbusutil-gen em -type InputDevices
type InputDevices struct {
	service *dbusutil.Service
	l       *libinput

	maxTouchscreenId uint32

	touchscreensMu sync.Mutex
	touchscreens   map[dbus.ObjectPath]*Touchscreen
	// dbusutil-gen: equal=touchscreenSliceEqual
	Touchscreens []dbus.ObjectPath

	//nolint
	signals *struct {
		TouchscreenAdded, TouchscreenRemoved struct {
			path dbus.ObjectPath
		}
	}
}

func newInputDevices() *InputDevices {
	return &InputDevices{
		touchscreens: make(map[dbus.ObjectPath]*Touchscreen),
	}
}

func (*InputDevices) GetInterfaceName() string {
	return dbusInterface
}

func (m *InputDevices) init() {
	m.l = newLibinput(m)
	m.l.start()
}

func (m *InputDevices) newTouchscreen(dev *libinputDevice) {
	m.touchscreensMu.Lock()
	defer m.touchscreensMu.Unlock()

	m.maxTouchscreenId++
	t := newTouchscreen(dev, m.service, m.maxTouchscreenId)

	path := dbus.ObjectPath(touchscreenDBusPath + strconv.FormatUint(uint64(t.id), 10))
	t.export(path)

	m.touchscreens[path] = t

	touchscreens := append(m.Touchscreens, path)
	m.setPropTouchscreens(touchscreens)

	m.service.Emit(m, "TouchscreenAdded", path)
}

func (m *InputDevices) removeTouchscreen(dev *libinputDevice) {
	m.touchscreensMu.Lock()
	defer m.touchscreensMu.Unlock()

	i := m.getIndexByDevNode(dev.GetDevNode())
	if i == -1 {
		logger.Warningf("device %s not found", dev.GetDevNode())
		return
	}

	path := m.Touchscreens[i]

	touchscreens := append(m.Touchscreens[:i], m.Touchscreens[i+1:]...)
	m.setPropTouchscreens(touchscreens)

	t := m.touchscreens[path]
	t.stopExport()

	delete(m.touchscreens, path)

	m.service.Emit(m, "TouchscreenRemoved", path)
}

func (m *InputDevices) getIndexByDevNode(devNode string) int {
	var path dbus.ObjectPath
	for i, v := range m.touchscreens {
		if v.DevNode == devNode {
			path = i
		}
	}

	if path == "" {
		return -1
	}

	for i, v := range m.Touchscreens {
		if v == path {
			return i
		}
	}

	return -1
}

func touchscreenSliceEqual(v1 []dbus.ObjectPath, v2 []dbus.ObjectPath) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i, e1 := range v1 {
		if e1 != v2[i] {
			return false
		}
	}
	return true
}
