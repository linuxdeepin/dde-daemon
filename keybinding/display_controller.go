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
	"sync"

	"github.com/godbus/dbus"
	. "github.com/linuxdeepin/dde-daemon/keybinding/shortcuts"
	display "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.display"
	backlight "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.helper.backlight"
	"github.com/linuxdeepin/go-gir/gio-2.0"
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
	brightStatus    bool
	brightStatusMu  sync.Mutex
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

func (c *DisplayController) SetBrightStatus() {
	c.brightStatusMu.Lock()
	c.brightStatus = false
	c.brightStatusMu.Unlock()
}

func (c *DisplayController) ExecCmd(cmd ActionCmd) error {
	c.brightStatusMu.Lock()
	if !c.brightStatus {
		go func() {
			defer c.SetBrightStatus()
			c.brightStatus = true
			c.brightStatusMu.Unlock()
			var err error
			switch cmd {
			case DisplayModeSwitch:
				var displayList []string
				displayList, err = c.display.ListOutputNames(0)
				if err == nil && len(displayList) > 1 {
					showOSD("SwitchMonitors")
				}
			case MonitorBrightnessUp:
				err = c.changeBrightness(true)
			case MonitorBrightnessDown:
				err = c.changeBrightness(false)
			default:
				err = ErrInvalidActionCmd{cmd}
			}
			logger.Warning("Controller exec cmd err:", err)
		}()
	} else {
		c.brightStatusMu.Unlock()
	}

	return nil
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

	// 如果正在使用的显示器都不支持调节亮度，则不进行osd显示
	pathList, err := c.display.Monitors().Get(0)
	if err != nil {
		logger.Warning(err)
		return err
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return err
	}
	for _, path := range pathList {
		m, err := display.NewMonitor(conn, path)
		if err != nil {
			logger.Warning(err)
			return err
		}
		enable, err := m.Enabled().Get(0)
		if err != nil {
			logger.Warning(err)
			return err
		}
		if enable {
			name, err := m.Name().Get(0)
			if err != nil {
				logger.Warning(err)
				return err
			}
			canSet, err := c.display.CanSetBrightness(0, name)
			if err != nil {
				logger.Warning(err)
				return err
			}
			if canSet {
				showOSD(osd)
				return nil
			}
		}
	}

	return nil
}
