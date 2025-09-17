// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"errors"
	"fmt"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

var (
	errPropNotFound     = fmt.Errorf("this property not found")
	errPropTypeNotMatch = fmt.Errorf("this property's type not match")
)

func (m *XSManager) ListProps() (string, *dbus.Error) {
	datas, err := getSettingPropValue(m.owner, m.conn)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	infos := unmarshalSettingData(datas)
	if infos == nil || len(infos.items) == 0 {
		return "", nil
	}
	return infos.items.listProps(), nil
}

func (m *XSManager) SetInteger(prop string, v int32) *dbus.Error {
	var setting = xsSetting{
		sType: settingTypeInteger,
		prop:  prop,
		value: v,
	}

	err := m.setSettings([]xsSetting{setting})
	if err != nil {
		logger.Debugf("Set '%s' to '%v' failed: %v", prop, v, err)
		return dbusutil.ToError(err)
	}
	err = m.setGSettingsByXProp(prop, v)
	if err != nil {
		return dbusutil.ToError(err)
	}

	return nil
}

func (m *XSManager) GetInteger(prop string) (int32, *dbus.Error) {
	v, sType, err := m.getSettingValue(prop)
	if err != nil {
		logger.Debugf("Get '%s' value failed: %v", prop, err)
		return 0, dbusutil.ToError(err)
	}

	if sType != settingTypeInteger {
		return 0, dbusutil.ToError(errPropTypeNotMatch)
	}

	return v.(*integerValueInfo).value, nil
}

func (m *XSManager) SetString(prop, v string) *dbus.Error {
	err := m.SetStringInternal(prop, v)
	return dbusutil.ToError(err)
}

func (m *XSManager) SetStringInternal(prop, v string) error {
	var setting = xsSetting{
		sType: settingTypeString,
		prop:  prop,
		value: v,
	}

	err := m.setSettings([]xsSetting{setting})
	if err != nil {
		logger.Debugf("Set '%s' to '%v' failed: %v", prop, v, err)
		return err
	}
	return m.setGSettingsByXProp(prop, v)
}

func (m *XSManager) GetString(prop string) (string, *dbus.Error) {
	str, err := m.GetStringInternal(prop)
	return str, dbusutil.ToError(err)
}

func (m *XSManager) GetStringInternal(prop string) (string, error) {
	v, sType, err := m.getSettingValue(prop)
	if err != nil {
		logger.Debugf("Get '%s' value failed: %v", prop, err)
		return "", err
	}

	if sType != settingTypeString {
		return "", errPropTypeNotMatch
	}

	return v.(*stringValueInfo).value, nil
}

func (m *XSManager) SetColor(prop string, v []uint16) *dbus.Error {
	if len(v) != 4 {
		return dbusutil.ToError(errors.New("length of value is not 4"))
	}

	var val [4]uint16
	copy(val[:], v)

	var setting = xsSetting{
		sType: settingTypeColor,
		prop:  prop,
		value: val,
	}

	err := m.setSettings([]xsSetting{setting})
	if err != nil {
		logger.Debugf("Set '%s' to '%v' failed: %v", prop, val, err)
		return dbusutil.ToError(err)
	}
	err = m.setGSettingsByXProp(prop, val)
	return dbusutil.ToError(err)
}

func (m *XSManager) GetColor(prop string) ([]uint16, *dbus.Error) {
	v, sType, err := m.getSettingValue(prop)
	if err != nil {
		logger.Debugf("Get '%s' value failed: %v", prop, err)
		return nil, dbusutil.ToError(err)
	}

	if sType != settingTypeColor {
		return nil, dbusutil.ToError(errPropTypeNotMatch)
	}

	tmp := v.(*colorValueInfo)

	return []uint16{tmp.red, tmp.green, tmp.blue, tmp.alpha}, nil
}

func (m *XSManager) getSettingValue(prop string) (interface{}, uint8, error) {
	m.settingsLocker.RLock()
	defer m.settingsLocker.RUnlock()
	datas, err := getSettingPropValue(m.owner, m.conn)
	if err != nil {
		return nil, 0, err
	}

	xsInfo := unmarshalSettingData(datas)
	item := xsInfo.getPropItem(prop)
	if item == nil {
		return nil, 0, errPropNotFound
	}

	return item.value, item.header.sType, nil
}

func (m *XSManager) setGSettingsByXProp(prop string, v interface{}) error {
	info := gsInfos.getByXSKey(prop)
	if info == nil {
		return errPropNotFound
	}

	return info.setValue(m.xsettingsConfig, v)
}

func (m *XSManager) GetScaleFactor() (float64, *dbus.Error) {
	return m.getScaleFactor(), nil
}

func (m *XSManager) SetScaleFactor(scale float64) *dbus.Error {
	err := m.setScreenScaleFactors(singleToMapSF(scale), true)
	return dbusutil.ToError(err)
}

func (m *XSManager) SetScreenScaleFactors(factors map[string]float64) *dbus.Error {
	err := m.setScreenScaleFactors(factors, true)
	return dbusutil.ToError(err)
}

func (m *XSManager) GetScreenScaleFactors() (map[string]float64, *dbus.Error) {
	v := m.getScreenScaleFactors()
	return v, nil
}
