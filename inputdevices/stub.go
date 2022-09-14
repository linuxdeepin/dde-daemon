// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

const (
	dbusServiceName = "com.deepin.daemon.InputDevices"
	dbusPath        = "/com/deepin/daemon/InputDevices"
	dbusInterface   = dbusServiceName

	kbdDBusPath      = "/com/deepin/daemon/InputDevice/Keyboard"
	kbdDBusInterface = "com.deepin.daemon.InputDevice.Keyboard"

	mouseDBusPath           = "/com/deepin/daemon/InputDevice/Mouse"
	mouseDBusInterface      = "com.deepin.daemon.InputDevice.Mouse"
	trackPointDBusInterface = "com.deepin.daemon.InputDevice.TrackPoint"

	touchPadDBusPath      = "/com/deepin/daemon/InputDevice/TouchPad"
	touchPadDBusInterface = "com.deepin.daemon.InputDevice.TouchPad"

	wacomDBusPath      = "/com/deepin/daemon/InputDevice/Wacom"
	wacomDBusInterface = "com.deepin.daemon.InputDevice.Wacom"
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
