/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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

package keybinding

import (
	"github.com/godbus/dbus"
	display "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.display"
	backlight "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.helper.backlight"
	. "pkg.deepin.io/dde/daemon/keybinding/shortcuts"
	"pkg.deepin.io/gir/gio-2.0"
)

const (
	gsKeyOsdAdjustBrightnessState = "osd-adjust-brightness-enabled"
)

type OsdBrightnessState int32

// Osd亮度调节控制
const (
	BrightnessAdjustEnable OsdBrightnessState = iota
	BrightnessAdjustForbidden
	BrightnessAdjustHidden
)

type DisplayController struct {
	display         display.Display
	backlightHelper backlight.Backlight
	gsKeyboard      *gio.Settings
}

func NewDisplayController(backlightHelper backlight.Backlight, sessionConn *dbus.Conn) *DisplayController {
	c := &DisplayController{
		backlightHelper: backlightHelper,
		display:         display.NewDisplay(sessionConn),
	}
	c.gsKeyboard = gio.NewSettings(gsSchemaKeyboard)
	return c
}

func (*DisplayController) Name() string {
	return "Display"
}

func (c *DisplayController) ExecCmd(cmd ActionCmd) error {
	switch cmd {
	case DisplayModeSwitch:
		displayList, err := c.display.ListOutputNames(0)
		if err == nil && len(displayList) > 1 {
			showOSD("SwitchMonitors")
		}
		return err

	case MonitorBrightnessUp:
		return c.changeBrightness(true)

	case MonitorBrightnessDown:
		return c.changeBrightness(false)

	default:
		return ErrInvalidActionCmd{cmd}
	}
}

const gsKeyAmbientLightAdjustBrightness = "ambient-light-adjust-brightness"

func (c *DisplayController) changeBrightness(raised bool) error {
	var osd = "BrightnessUp"
	if !raised {
		osd = "BrightnessDown"
	}
	var state = OsdBrightnessState(c.gsKeyboard.GetEnum(gsKeyOsdAdjustBrightnessState))

	// 只有当OsdAdjustBrightnessState的值为BrightnessAdjustEnable时，才会去执行调整亮度的操作
	if BrightnessAdjustEnable == state {
		gs := gio.NewSettings("com.deepin.dde.power")
		autoAdjustBrightnessEnabled := gs.GetBoolean(gsKeyAmbientLightAdjustBrightness)
		if autoAdjustBrightnessEnabled {
			gs.SetBoolean(gsKeyAmbientLightAdjustBrightness, false)
		}
		gs.Unref()

		err := c.display.ChangeBrightness(dbus.FlagNoAutoStart, raised)
		if err != nil {
			return err
		}
	} else if BrightnessAdjustForbidden == state {
		if raised {
			osd = "BrightnessUpAsh"
		} else {
			osd = "BrightnessDownAsh"
		}
	} else {
		return nil
	}

	showOSD(osd)
	return nil
}
