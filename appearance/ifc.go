// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-daemon/appearance/fonts"
	"github.com/linuxdeepin/dde-daemon/appearance/subthemes"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
)

// Reset reset all themes and fonts settings to default values
func (m *Manager) Reset() *dbus.Error {
	logger.Debug("Reset settings")

	var settingKeys = []string{
		gsKeyGtkTheme,
		gsKeyIconTheme,
		gsKeyCursorTheme,
		gsKeyFontSize,
	}
	for _, key := range settingKeys {
		userVal := m.setting.GetUserValue(key)
		if userVal != nil {
			logger.Debug("reset setting", key)
			m.setting.Reset(key)
		}
	}

	m.resetFonts()
	return nil
}

// List list all available for the special type, return a json format list
func (m *Manager) List(ty string) (list string, busErr *dbus.Error) {
	logger.Debug("List for type:", ty)
	jsonStr, err := m.list(ty)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return jsonStr, nil
}

func (m *Manager) list(ty string) (string, error) {
	switch strings.ToLower(ty) {
	case TypeGtkTheme:
		themes := subthemes.ListGtkTheme()
		var gtkThemes subthemes.Themes
		for _, theme := range themes {
			if !strings.HasPrefix(theme.Id, "deepin") {
				continue
			}
			gtkThemes = append(gtkThemes, theme)
		}
		gtkThemes = append(gtkThemes, &subthemes.Theme{
			Id:        autoGtkTheme,
			Path:      "",
			Deletable: false,
		})
		return m.doShow(gtkThemes)
	case TypeIconTheme:
		return m.doShow(subthemes.ListIconTheme())
	case TypeCursorTheme:
		return m.doShow(subthemes.ListCursorTheme())
	case TypeBackground:
		return m.doShow(m.listBackground())
	case TypeStandardFont:
		return m.doShow(fonts.GetFamilyTable().ListStandard())
	case TypeMonospaceFont:
		return m.doShow(fonts.GetFamilyTable().ListMonospace())
	}
	return "", fmt.Errorf("invalid type: %v", ty)

}

// Show show detail infos for the special type
// ret0: detail info, json format
func (m *Manager) Show(ty string, names []string) (detail string, busErr *dbus.Error) {
	logger.Debugf("Show '%s' type '%s'", names, ty)
	jsonStr, err := m.show(ty, names)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return jsonStr, nil
}

func (m *Manager) show(ty string, names []string) (string, error) {
	switch strings.ToLower(ty) {
	case TypeGtkTheme:
		gtkThemes := subthemes.ListGtkTheme().ListGet(names)
		if strv.Strv(names).Contains(autoGtkTheme) {
			gtkThemes = append(gtkThemes, &subthemes.Theme{
				Id:        autoGtkTheme,
				Path:      "",
				Deletable: false,
			})
		}
		return m.doShow(gtkThemes)
	case TypeIconTheme:
		return m.doShow(subthemes.ListIconTheme().ListGet(names))
	case TypeCursorTheme:
		return m.doShow(subthemes.ListCursorTheme().ListGet(names))
	case TypeBackground:
		return m.doShow(m.listBackground().ListGet(names))
	case TypeStandardFont, TypeMonospaceFont:
		return m.doShow(fonts.GetFamilyTable().GetFamilies(names))
	}
	return "", fmt.Errorf("invalid type: %v", ty)
}

// Set set to the special 'value'
func (m *Manager) Set(ty, value string) *dbus.Error {
	logger.Debugf("Set '%s' for type '%s'", value, ty)
	err := m.set(ty, value)
	return dbusutil.ToError(err)
}

func (m *Manager) set(ty, value string) error {
	var err error
	switch strings.ToLower(ty) {
	case TypeGtkTheme:
		if m.GtkTheme.Get() == value {
			return nil
		}

		err = m.doSetGtkTheme(value)
		if err == nil {
			m.GtkTheme.Set(value)
		}
	case TypeIconTheme:
		if m.IconTheme.Get() == value {
			return nil
		}
		err = m.doSetIconTheme(value)
		if err == nil {
			m.IconTheme.Set(value)
		}
	case TypeCursorTheme:
		if m.CursorTheme.Get() == value {
			return nil
		}
		err = m.doSetCursorTheme(value)
		if err == nil {
			m.CursorTheme.Set(value)
		}
	case TypeBackground: //old change wallpaple interface (no used)
		file, err := m.doSetBackground(value)
		if err == nil && m.wsLoopMap[m.curMonitorSpace] != nil {
			m.wsLoopMap[m.curMonitorSpace].AddToShowed(file)
		}
	case TypeGreeterBackground:
		err = m.doSetGreeterBackground(value)
	case TypeStandardFont:
		if m.StandardFont.Get() == value {
			return nil
		}
		err = m.doSetStandardFont(value)
		if err == nil {
			m.StandardFont.Set(value)
		}
	case TypeMonospaceFont:
		if m.MonospaceFont.Get() == value {
			return nil
		}
		err = m.doSetMonospaceFont(value)
		if err == nil {
			m.MonospaceFont.Set(value)
		}
	case TypeFontSize:
		size, e := strconv.ParseFloat(value, 64)
		if e != nil {
			return e
		}

		cur := m.FontSize.Get()
		if cur > size-0.01 && cur < size+0.01 {
			return nil
		}
		err = m.doSetFontSize(size)
		if err == nil {
			m.FontSize.Set(size)
		}
	default:
		return fmt.Errorf("invalid type: %v", ty)
	}
	return err
}

func (m *Manager) SetMonitorBackground(monitorName string, imageFile string) *dbus.Error {
	logger.Debugf("Set Background '%s' for Monitor '%s'", imageFile, monitorName)
	file, err := m.doSetMonitorBackground(monitorName, imageFile)
	if err == nil {
		idx, err := m.wm.GetCurrentWorkspace(0)
		if err == nil {
			wsLoop := m.wsLoopMap[genMonitorKeyString(monitorName, int(idx))]
			if wsLoop != nil {
				wsLoop.AddToShowed(file)
			}
		}
	}
	return dbusutil.ToError(err)
}

func (m *Manager) SetWallpaperSlideShow(monitorName string, wallpaperSlideShow string) *dbus.Error {
	logger.Debugf("Set Current Workspace Wallpaper SlideShow '%s' For Monitor '%s'", wallpaperSlideShow, monitorName)
	err := m.doSetWallpaperSlideShow(monitorName, wallpaperSlideShow)
	return dbusutil.ToError(err)
}

func (m *Manager) GetWallpaperSlideShow(monitorName string) (slideShow string, busErr *dbus.Error) {
	logger.Debugf("Get Current Workspace Wallpaper SlideShow For Monitor '%s'", monitorName)
	slideShow, err := m.doGetWallpaperSlideShow(monitorName)
	return slideShow, dbusutil.ToError(err)
}

// Delete delete the special 'name'
func (m *Manager) Delete(ty, name string) *dbus.Error {
	logger.Debugf("Delete '%s' type '%s'", name, ty)
	err := m.delete(ty, name)
	return dbusutil.ToError(err)
}

func (m *Manager) delete(ty, name string) error {
	switch strings.ToLower(ty) {
	case TypeGtkTheme:
		return subthemes.ListGtkTheme().Delete(name)
	case TypeIconTheme:
		return subthemes.ListIconTheme().Delete(name)
	case TypeCursorTheme:
		return subthemes.ListCursorTheme().Delete(name)
	case TypeBackground:
		return m.listBackground().Delete(name)
		//case TypeStandardFont:
		//case TypeMonospaceFont:
	}
	return fmt.Errorf("invalid type: %v", ty)
}

// Thumbnail get thumbnail for the special 'name'
func (m *Manager) Thumbnail(ty, name string) (file string, busErr *dbus.Error) {
	file, err := m.thumbnail(ty, name)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return file, nil
}

var gtkThumbnailMap = map[string]string{
	"deepin":      "light",
	"deepin-dark": "dark",
	"deepin-auto": "auto",
}

func (m *Manager) thumbnail(ty, name string) (string, error) {
	logger.Debugf("Get thumbnail for '%s' type '%s'", name, ty)
	switch strings.ToLower(ty) {
	case TypeGtkTheme:
		fName, ok := gtkThumbnailMap[name]
		if ok {
			return filepath.Join("/usr/share/dde-daemon/appearance", fName+".svg"), nil
		}
		return subthemes.GetGtkThumbnail(name)
	case TypeIconTheme:
		return subthemes.GetIconThumbnail(name)
	case TypeCursorTheme:
		return subthemes.GetCursorThumbnail(name)
	}
	return "", fmt.Errorf("invalid type: %v", ty)
}

func (m *Manager) GetScaleFactor() (scaleFactor float64, busErr *dbus.Error) {
	return m.getScaleFactor(), nil
}

func (m *Manager) SetScaleFactor(scale float64) *dbus.Error {
	err := m.setScaleFactor(scale)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	return nil
}

func (m *Manager) SetScreenScaleFactors(v map[string]float64) *dbus.Error {
	err := m.setScreenScaleFactors(v)
	if err != nil {
		logger.Warning(err)
	}
	return dbusutil.ToError(err)
}

func (m *Manager) GetScreenScaleFactors() (scaleFactors map[string]float64, busErr *dbus.Error) {
	v, err := m.getScreenScaleFactors()
	return v, dbusutil.ToError(err)
}
