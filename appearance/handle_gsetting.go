// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"fmt"
	"strings"

	"github.com/linuxdeepin/go-lib/gsettings"
)

func (m *Manager) listenGSettingChanged() {
	gsettings.ConnectChanged(xSettingsSchema, gsKeyQtActiveColor, func(key string) {
		value, err := m.getQtActiveColor()
		if err != nil {
			logger.Warning(err)
			return
		}
		if m.QtActiveColor != value {
			m.QtActiveColor = value
			err = m.service.EmitPropertyChanged(m, propQtActiveColor, value)
			if err != nil {
				logger.Warning(err)
			}
		}
	})

	gsettings.ConnectChanged(appearanceSchema, "*", func(key string) {
		if m.setting == nil {
			return
		}

		var (
			ty    string
			value string
			err   error
		)
		switch key {
		case gsKeyGtkTheme:
			ty = TypeGtkTheme
			value = m.setting.GetString(key)
			err = m.doSetGtkTheme(value)
			m.updateThemeAuto(value == autoGtkTheme)
		case gsKeyIconTheme:
			ty = TypeIconTheme
			value = m.setting.GetString(key)
			err = m.doSetIconTheme(value)
		case gsKeyCursorTheme:
			ty = TypeCursorTheme
			value = m.setting.GetString(key)
			err = m.doSetCursorTheme(value)
		case gsKeyFontStandard:
			ty = TypeStandardFont
			value = m.setting.GetString(key)
			// 如果值相同，可能是dde设置导致的gsetting变化，不做变更
			if m.StandardFont.Get() != value {
				err = m.doSetStandardFont(value)
			}
		case gsKeyFontMonospace:
			ty = TypeMonospaceFont
			value = m.setting.GetString(key)
			// 如果值相同，可能是dde设置导致的gsetting变化，不做变更
			if m.MonospaceFont.Get() != value {
				err = m.doSetMonospaceFont(value)
			}
		case gsKeyDTKSizeMode:
			ty = TypeDTKSizeMode
			enabled := m.setting.GetInt(key)
			value = fmt.Sprint(enabled)
			err = m.doSetDTKSizeMode(enabled)
			if err == nil {
				fontSizeKey := gsKeyFontSize
				if m.DTKSizeMode.Get() == 1 {
					fontSizeKey = gsKeyCompactFontSize
				}
				m.FontSize.Bind(m.setting, fontSizeKey)
				_ = m.service.EmitPropertyChanged(m, propFontSize, m.FontSize.Get())
				err = m.doSetFontSize(m.FontSize.Get())
			}
		case gsKeyFontSize:
			ty = TypeFontSize
			size := m.setting.GetDouble(key)
			value = fmt.Sprint(size)
			if m.isHasDTKSizeModeKey() && (m.DTKSizeMode.Get() == 0) {
				err = m.doSetFontSize(size)
			} else {
				err = m.doSetFontSize(size)
			}
		case gsKeyCompactFontSize:
			ty = TypeCompactFontSize
			size := m.setting.GetDouble(key)
			value = fmt.Sprint(size)
			if m.isHasDTKSizeModeKey() && (m.DTKSizeMode.Get() == 1) {
				err = m.doSetFontSize(size)
			}
		case gsKeyBackgroundURIs:
			ty = TypeBackground
			bgs := m.setting.GetStrv(key)
			m.desktopBgs = bgs
			m.setDesktopBackgrounds(bgs)
			value = strings.Join(bgs, ";")
		case gsKeyWallpaperSlideshow:
			policy := m.setting.GetString(key)
			m.updateWSPolicy(policy)
		default:
			return
		}
		if err != nil {
			logger.Warningf("Set %v failed: %v", key, err)
			return
		}
		if ty != "" {
			m.emitSignalChanged(ty, value)
		}
	})

	m.listenBgGSettings()
}

func (m *Manager) emitSignalChanged(type0, value string) {
	err := m.service.Emit(m, "Changed", type0, value)
	if err != nil {
		logger.Warning("emit emitSignalChanged Failed:", err)
	}
}

func (m *Manager) listenBgGSettings() {
	gsettings.ConnectChanged(wrapBgSchema, "picture-uri", func(key string) {
		if m.wrapBgSetting == nil {
			return
		}

		logger.Debug(wrapBgSchema, "changed")
		value := m.wrapBgSetting.GetString(key)
		file, err := m.doSetBackground(value)
		if err != nil {
			logger.Warning(err)
			return
		}
		if m.wsLoopMap[m.curMonitorSpace] != nil {
			m.wsLoopMap[m.curMonitorSpace].AddToShowed(file)
		}
	})

	if m.gnomeBgSetting == nil {
		return
	}
	gsettings.ConnectChanged(gnomeBgSchema, "picture-uri", func(key string) {
		if m.gnomeBgSetting == nil {
			return
		}

		logger.Debug(gnomeBgSchema, "changed")
		value := m.gnomeBgSetting.GetString(gsKeyBackground)
		file, err := m.doSetBackground(value)
		if err != nil {
			logger.Warning(err)
			return
		}
		if m.wsLoopMap[m.curMonitorSpace] != nil {
			m.wsLoopMap[m.curMonitorSpace].AddToShowed(file)
		}
	})
}
