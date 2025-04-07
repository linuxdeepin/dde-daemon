// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package keybinding

import (
	"os/exec"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/keybinding1/constants"
	. "github.com/linuxdeepin/dde-daemon/keybinding1/shortcuts"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	display "github.com/linuxdeepin/go-dbus-factory/session/org.deepin.dde.display1"
	backlight "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.backlighthelper1"
	"github.com/linuxdeepin/go-lib/strv"
)

type OsdBrightnessState int32

// Osd亮度调节控制
const (
	BrightnessAdjustEnable OsdBrightnessState = iota
	BrightnessAdjustForbidden
	BrightnessAdjustHidden
)

type DisplayController struct {
	display           display.Display
	backlightHelper   backlight.Backlight
	keyboardConfigMgr configManager.Manager
	powerConfigMgr    configManager.Manager
	brightStatusBusy  bool
	brightStatusMu    sync.Mutex
	m                 *Manager
}

func NewDisplayController(backlightHelper backlight.Backlight, sessionConn *dbus.Conn, m *Manager) *DisplayController {
	c := &DisplayController{
		backlightHelper: backlightHelper,
		display:         display.NewDisplay(sessionConn),
		m:               m,
	}

	bus, _ := dbus.SystemBus()
	dsg := configManager.NewConfigManager(bus)

	dsgPath, err := dsg.AcquireManager(0, constants.DSettingsAppID, constants.DSettingsKeyboardId, "")
	if err != nil || dsgPath == "" {
		logger.Warning(err)
		return c
	}

	c.keyboardConfigMgr, err = configManager.NewManager(bus, dsgPath)
	if err != nil {
		logger.Warning(err)
	}

	dsgPath, err = dsg.AcquireManager(0, constants.DSettingsAppID, constants.DSettingsPowerId, "")
	if err != nil || dsgPath == "" {
		logger.Warning(err)
		return c
	}
	c.powerConfigMgr, err = configManager.NewManager(bus, dsgPath)
	if err != nil {
		logger.Warning(err)
	}

	return c
}

func (*DisplayController) Name() string {
	return "Display"
}

func (c *DisplayController) ExecCmd(cmd ActionCmd) error {
	c.brightStatusMu.Lock()
	if c.brightStatusBusy {
		c.brightStatusMu.Unlock()
		return nil
	}

	c.brightStatusBusy = true
	c.brightStatusMu.Unlock()

	go func() {
		defer func() {
			c.brightStatusMu.Lock()
			c.brightStatusBusy = false
			c.brightStatusMu.Unlock()
		}()

		var err error
		switch cmd {
		case DisplayModeSwitch:
			// TODO 联想xrandr -q需要修改成X的接口
			if c.m.dmiInfo.ProductName != "" &&
				strv.Strv(c.m.needXrandrQDevice).Contains(c.m.dmiInfo.ProductName) {
				logger.Info("win+p pressed,need run xrandr -q")
				err = exec.Command("xrandr", "-q").Run()
				if err != nil {
					logger.Warning(err)
				}
			}
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
		if err != nil {
			logger.Warning("Controller exec cmd err:", err)
		}

	}()

	return nil
}

func (c *DisplayController) changeBrightness(raised bool) error {
	var osd = "BrightnessUp"
	if !raised {
		osd = "BrightnessDown"
	}

	osdAdjustBrightnessState, err := c.keyboardConfigMgr.Value(0, constants.DSettingsKeyOsdAdjustBrightnessState)
	if err != nil {
		logger.Warning(err)
		return err
	}

	var state = OsdBrightnessState(osdAdjustBrightnessState.Value().(int64))

	// 只有当OsdAdjustBrightnessState的值为BrightnessAdjustEnable时，才会去执行调整亮度的操作
	if BrightnessAdjustEnable == state {

		autoAdjustBrightnessEnabledValue, err := c.powerConfigMgr.Value(0, constants.DSettingsKeyAmbientLightAdjustBrightness)
		if err != nil {
			logger.Warning(err)
		}
		autoAdjustBrightnessEnabled := autoAdjustBrightnessEnabledValue.Value().(bool)
		if autoAdjustBrightnessEnabled {
			c.powerConfigMgr.SetValue(0, constants.DSettingsKeyAmbientLightAdjustBrightness, dbus.MakeVariant(false))
		}

		err = c.display.ChangeBrightness(dbus.FlagNoAutoStart, raised)
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
