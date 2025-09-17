// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"fmt"
	"os"
	"sync"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	ddeSysDaemon "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.daemon1"
	greeter "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.greeter1"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/dbusutil"
	x "github.com/linuxdeepin/go-x11-client"
)

//go:generate dbusutil-gen em -type XSManager

const (
	defaultScaleFactor = 1.0

	xsDBusService          = "org.deepin.dde.XSettings1"
	xsDBusPath             = "/org/deepin/dde/XSettings1"
	xsDBusIFC              = xsDBusService
	dsettingsAppID         = "org.deepin.dde.daemon"
	dsettingsXSettingsName = "org.deepin.XSettings"
)

// XSManager xsettings manager
type XSManager struct {
	service *dbusutil.Service
	conn    *x.Conn
	owner   x.Window

	xsettingsConfig *dconfig.DConfig
	greeter         greeter.Greeter
	sysDaemon       ddeSysDaemon.Daemon

	plymouthScalingMu    sync.Mutex
	plymouthScalingTasks []int
	plymouthScaling      bool

	restartOSD bool // whether to restart dde-osd

	// locker for xsettings prop read and write
	settingsLocker sync.RWMutex

	sessionSigLoop *dbusutil.SignalLoop

	//nolint
	signals *struct {
		SetScaleFactorStarted, SetScaleFactorDone struct{}
	}
}

type xsSetting struct {
	sType uint8
	prop  string
	value interface{} // int32, string, [4]uint16
}

func NewXSManager(conn *x.Conn, recommendedScaleFactor float64, service *dbusutil.Service) (*XSManager, error) {
	var m = &XSManager{
		conn:    conn,
		service: service,
	}

	var err error
	m.owner, err = createSettingWindow(m.conn)
	if err != nil {
		return nil, err
	}
	logger.Debug("owner:", m.owner)

	if !isSelectionOwned(settingPropScreen, m.owner, m.conn) {
		logger.Errorf("Owned '%s' failed", settingPropSettings)
		return nil, fmt.Errorf("Owned '%s' failed", settingPropSettings)
	}

	systemBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	m.greeter = greeter.NewGreeter(systemBus)
	m.sysDaemon = ddeSysDaemon.NewDaemon(systemBus)
	m.xsettingsConfig, err = dconfig.NewDConfig(dsettingsAppID, dsettingsXSettingsName, "")
	if err != nil {
		return nil, fmt.Errorf("new dconfig failed")
	}

	err = m.setScreenScaleFactors(m.getScreenScaleFactors(), false)
	if err != nil {
		logger.Warning(err)
	}
	m.adjustScaleFactor(recommendedScaleFactor)
	err = m.setSettings(m.getSettingsInSchema())
	if err != nil {
		logger.Warning("Change xsettings property failed:", err)
	}
	m.sessionSigLoop = dbusutil.NewSignalLoop(m.service.Conn(), 10)
	m.sessionSigLoop.Start()

	return m, nil
}

func (m *XSManager) GetInterfaceName() string {
	return xsDBusIFC
}

var _gs *gio.Settings

func (m *XSManager) getScaleFactor() float64 {
	scale := _gs.GetDouble(gsKeyScaleFactor)
	return scale
}

func (m *XSManager) adjustScaleFactor(recommendedScaleFactor float64) {
	logger.Debug("recommended scale factor:", recommendedScaleFactor)
	var err error
	if value, _ := m.xsettingsConfig.GetValueFloat64(gsKeyScaleFactor); value <= 0 {
		err = m.setScaleFactorWithoutNotify(recommendedScaleFactor)
		if err != nil {
			logger.Warning("failed to set scale factor:", err)
		}
		m.restartOSD = true
	}

	// migrate old configuration
	if os.Getenv("STARTDDE_MIGRATE_SCALE_FACTOR") != "" {
		scaleFactor := m.getScaleFactor()
		if scaleFactor > 0 {
			err = m.setScreenScaleFactorsForQt(map[string]float64{"": scaleFactor})
			if err != nil {
				logger.Warning("failed to set scale factor for qt:", err)
			}
		}

		err = cleanUpDdeEnv()
		if err != nil {
			logger.Warning("failed to clean up dde env:", err)
		}
		return
	}

	_, err = os.Stat("/etc/lightdm/deepin/qt-theme.ini")
	if err != nil {
		if os.IsNotExist(err) {
			// lightdm-deepin-greeter does not have the qt-theme.ini file yet.
			scaleFactor := m.getScaleFactor()
			if scaleFactor > 0 {
				err = m.setScreenScaleFactorsForQt(map[string]float64{"": scaleFactor})
				if err != nil {
					logger.Warning("failed to set scale factor for qt:", err)
				}
			}
		} else {
			logger.Warning(err)
		}
	}
}

func (m *XSManager) setSettings(settings []xsSetting) error {
	m.settingsLocker.Lock()
	defer m.settingsLocker.Unlock()
	datas, err := getSettingPropValue(m.owner, m.conn)
	if err != nil {
		return err
	}

	xsInfo := unmarshalSettingData(datas)
	xsInfo.serial++ // auto increment
	for _, s := range settings {
		item := xsInfo.getPropItem(s.prop)
		if item != nil {
			xsInfo.items = xsInfo.modifyProperty(s)
			continue
		}

		if s.value == nil {
			continue
		}

		var tmp *xsItemInfo
		switch s.sType {
		case settingTypeInteger:
			tmp = newXSItemInteger(s.prop, s.value.(int32))
		case settingTypeString:
			tmp = newXSItemString(s.prop, s.value.(string))
		case settingTypeColor:
			tmp = newXSItemColor(s.prop, s.value.([4]uint16))
		}

		xsInfo.items = append(xsInfo.items, *tmp)
		xsInfo.numSettings++
	}

	data := marshalSettingData(xsInfo)
	return changeSettingProp(m.owner, data, m.conn)
}

func (m *XSManager) getSettingsInSchema() []xsSetting {
	var settings []xsSetting
	keys, err := m.xsettingsConfig.ListKeys()
	if err != nil {
		logger.Warning(err)
		return settings
	}
	for _, key := range keys {
		info := gsInfos.getByGSKey(key)
		if info == nil {
			continue
		}

		value, err := info.getValue(m.xsettingsConfig)
		if err != nil {
			logger.Warning(err)
			continue
		}

		settings = append(settings, xsSetting{
			sType: info.getKeySType(),
			prop:  info.xsKey,
			value: value,
		})
	}
	return settings
}

func (m *XSManager) handleDConfigChangedCb(key string) {
	switch key {
	case "xft-dpi":
		return
	case gsKeyScaleFactor:
		// 删除m.updateDPI()，保证设置屏幕缩放比例不会立刻生效
		return
	case gsKeyGtkCursorThemeName:
		cursorTheme, _ := m.xsettingsConfig.GetValueString(gsKeyGtkCursorThemeName)
		updateXResources(xresourceInfos{
			&xresourceInfo{
				key:   "Xcursor.theme",
				value: cursorTheme,
			},
		})
	case gsKeyGtkCursorThemeSize:
		// 删除updateXResources,阻止设置屏幕缩放后,修改光标大小
		return
	case gsKeyWindowScale:
		// 删除m.updateDPI()，保证设置屏幕缩放比例不会立刻生效
		return
	}
	info := gsInfos.getByGSKey(key)
	if info == nil {
		return
	}

	value, err := info.getValue(m.xsettingsConfig)
	if err == nil {
		err = m.setSettings([]xsSetting{
			{
				sType: info.getKeySType(),
				prop:  info.xsKey,
				value: value,
			},
		})
		if err != nil {
			logger.Warning(err)
		}
	} else {
		logger.Warning(err)
	}
}

// Start load xsettings module
func Start(conn *x.Conn, recommendedScaleFactor float64, service *dbusutil.Service) (*XSManager, error) {
	m, err := NewXSManager(conn, recommendedScaleFactor, service)
	if err != nil {
		logger.Error("Start xsettings failed:", err)
		return nil, err
	}
	m.updateDPI()
	m.updateXResources()
	go m.updateFirefoxDPI()

	err = service.Export(xsDBusPath, m)
	if err != nil {
		logger.Warning("export XSManager failed:", err)
		return nil, err
	}

	err = service.RequestName(xsDBusService)
	if err != nil {
		return nil, err
	}

	m.xsettingsConfig.ConnectValueChanged(m.handleDConfigChangedCb)
	return m, nil
}

func (m *XSManager) NeedRestartOSD() bool {
	if m == nil {
		return false
	}
	return m.restartOSD
}
