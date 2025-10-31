// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"fmt"
	"strconv"
)

const (
	DPI_FALLBACK = 96
	HIDPI_LIMIT  = DPI_FALLBACK * 2

	ffKeyPixels = `user_pref("layout.css.devPixelsPerPx",`
)

// TODO: update 'antialias, hinting, hintstyle, rgba, cursor-theme, cursor-size'
func (m *XSManager) updateDPI() {
	scale, _ := m.xsettingsConfig.GetValueFloat64(gsKeyScaleFactor)
	if scale <= 0 {
		scale = 1
	}

	var infos []xsSetting
	scaledDPI := int32(float64(DPI_FALLBACK*1024) * scale)
	dpiValueInt, _ := m.xsettingsConfig.GetValueInt("xft-dpi")
	dpiValue := int32(dpiValueInt)
	if scaledDPI != int32(dpiValue) {
		m.xsettingsConfig.SetValue("xft-dpi", scaledDPI)
		infos = append(infos, xsSetting{
			sType: settingTypeInteger,
			prop:  "Xft/DPI",
			value: scaledDPI,
		})
	}

	// update window scale and cursor size
	windowScaleValue, _ := m.xsettingsConfig.GetValueInt(gsKeyWindowScale)
	windowScale := int32(windowScaleValue)
	cursorSizeValue, _ := m.xsettingsConfig.GetValueInt(gsKeyGtkCursorThemeSize)
	cursorSize := int32(cursorSizeValue)

	if windowScale > 1 {
		scaledDPI = int32(DPI_FALLBACK * 1024)
	}
	v, _ := m.GetInteger("Gdk/WindowScalingFactor")
	if v != windowScale {
		infos = append(infos, xsSetting{
			sType: settingTypeInteger,
			prop:  "Gdk/WindowScalingFactor",
			value: windowScale,
		}, xsSetting{
			sType: settingTypeInteger,
			prop:  "Gdk/UnscaledDPI",
			value: scaledDPI,
		}, xsSetting{
			sType: settingTypeInteger,
			prop:  "Gtk/CursorThemeSize",
			value: cursorSize,
		})
	}

	if len(infos) != 0 {
		err := m.setSettings(infos)
		if err != nil {
			logger.Warning("Failed to update dpi:", err)
		}
		m.updateXResources()
	}
}

func (m *XSManager) updateXResources() {
	scaleFactor, _ := m.xsettingsConfig.GetValueFloat64(gsKeyScaleFactor)
	cursorTheme, _ := m.xsettingsConfig.GetValueString(gsKeyGtkCursorThemeName)
	cursorSize, _ := m.xsettingsConfig.GetValueInt(gsKeyGtkCursorThemeSize)
	xftDpi := int(DPI_FALLBACK * scaleFactor)
	updateXResources(xresourceInfos{
		&xresourceInfo{
			key:   "Xcursor.theme",
			value: cursorTheme,
		},
		&xresourceInfo{
			key:   "Xcursor.size",
			value: fmt.Sprintf("%d", int32(cursorSize)),
		},
		&xresourceInfo{
			key:   "Xft.dpi",
			value: strconv.Itoa(xftDpi),
		},
	})
}
