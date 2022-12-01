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

package inputdevices

const (
	dbusServiceName = "org.deepin.dde.InputDevices1"
	dbusPath        = "/org/deepin/dde/InputDevices1"
	dbusInterface   = dbusServiceName

	kbdDBusPath      = "/org/deepin/dde/InputDevice1/Keyboard"
	kbdDBusInterface = "org.deepin.dde.InputDevice1.Keyboard"

	mouseDBusPath           = "/org/deepin/dde/InputDevice1/Mouse"
	mouseDBusInterface      = "org.deepin.dde.InputDevice1.Mouse"
	trackPointDBusInterface = "org.deepin.dde.InputDevice1.TrackPoint"

	touchPadDBusPath      = "/org/deepin/dde/InputDevice1/TouchPad"
	touchPadDBusInterface = "org.deepin.dde.InputDevice1.TouchPad"

	wacomDBusPath      = "/org/deepin/dde/InputDevice1/Wacom"
	wacomDBusInterface = "org.deepin.dde.InputDevice1.Wacom"
)

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (*Keyboard) GetInterfaceName() string {
	return kbdDBusInterface
}

func (*Mouse) GetInterfaceName() string {
	return mouseDBusInterface
}

func (*TrackPoint) GetInterfaceName() string {
	return trackPointDBusInterface
}

func (*Touchpad) GetInterfaceName() string {
	return touchPadDBusInterface
}

func (*Wacom) GetInterfaceName() string {
	return wacomDBusInterface
}
