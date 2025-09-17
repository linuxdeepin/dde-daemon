// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/linuxdeepin/dde-daemon/common/dconfig"
)

const (
	gsKeyTypeBool int = iota + 1
	gsKeyTypeInt
	gsKeyTypeString
	gsKeyTypeDouble
)

type typeGSKeyInfo struct {
	gsKey         string
	gsType        int
	xsKey         string
	xsType        *uint8
	convertGsToXs func(interface{}) (interface{}, error)
	convertXsToGs func(interface{}) (interface{}, error)
}

type typeGSKeyInfos []typeGSKeyInfo

var gsInfos = typeGSKeyInfos{
	{
		gsKey:  "theme-name",
		xsKey:  "Net/ThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "icon-theme-name",
		xsKey:  "Net/IconThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "fallback-icon-theme",
		xsKey:  "Net/FallbackIconTheme",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "sound-theme-name",
		xsKey:  "Net/SoundThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-theme-name",
		xsKey:  "Gtk/ThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-cursor-theme-name",
		xsKey:  "Gtk/CursorThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-font-name",
		xsKey:  "Gtk/FontName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-key-theme-name",
		xsKey:  "Gtk/KeyThemeName",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-color-palette",
		xsKey:  "Gtk/ColorPalette",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-toolbar-style",
		xsKey:  "Gtk/ToolbarStyle",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-toolbar-icon-size",
		xsKey:  "Gtk/ToolbarIconSize",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-color-scheme",
		xsKey:  "Gtk/ColorScheme",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-im-preedit-style",
		xsKey:  "Gtk/IMPreeditStyle",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-im-status-style",
		xsKey:  "Gtk/IMStatusStyle",
		gsType: gsKeyTypeString,
	}, //deprecated
	{
		gsKey:  "gtk-im-module",
		xsKey:  "Gtk/IMModule",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-modules",
		xsKey:  "Gtk/Modules",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "gtk-menubar-accel",
		xsKey:  "Gtk/MenuBarAccel",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "xft-hintstyle",
		xsKey:  "Xft/HintStyle",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "xft-rgba",
		xsKey:  "Xft/RGBA",
		gsType: gsKeyTypeString,
	},
	{
		gsKey:  "cursor-blink-time",
		xsKey:  "Net/CursorBlinkTime",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "gtk-cursor-blink-timeout",
		xsKey:  "Net/CursorBlinkTimeout",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "double-click-time",
		xsKey:  "Net/DoubleClickTime",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "double-click-distance",
		xsKey:  "Net/DoubleClickDistance",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "dnd-drag-threshold",
		xsKey:  "Net/DndDragThreshold",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  gsKeyGtkCursorThemeSize,
		xsKey:  "Gtk/CursorThemeSize",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "gtk-timeout-initial",
		xsKey:  "Gtk/TimeoutInitial",
		gsType: gsKeyTypeInt,
	}, //deprecated
	{
		gsKey:  "gtk-timeout-repeat",
		xsKey:  "Gtk/TimeoutRepeat",
		gsType: gsKeyTypeInt,
	}, //deprecated
	{
		gsKey:  "gtk-recent-files-max-age",
		xsKey:  "Gtk/RecentFilesMaxAge",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "xft-dpi",
		xsKey:  "Xft/DPI",
		gsType: gsKeyTypeInt,
	},
	{
		gsKey:  "cursor-blink",
		xsKey:  "Net/CursorBlink",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "enable-event-sounds",
		xsKey:  "Net/EnableEventSounds",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "enable-input-feedback-sounds",
		xsKey:  "Net/EnableInputFeedbackSounds",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "gtk-can-change-accels",
		xsKey:  "Gtk/CanChangeAccels",
		gsType: gsKeyTypeBool,
	}, //deprecated
	{
		gsKey:  "gtk-menu-images",
		xsKey:  "Gtk/MenuImages",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "gtk-button-images",
		xsKey:  "Gtk/ButtonImages",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "gtk-enable-animations",
		xsKey:  "Gtk/EnableAnimations",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "gtk-show-input-method-menu",
		xsKey:  "Gtk/ShowInputMethodMenu",
		gsType: gsKeyTypeBool,
	}, //deprecated
	{
		gsKey:  "gtk-show-unicode-menu",
		xsKey:  "Gtk/ShowUnicodeMenu",
		gsType: gsKeyTypeBool,
	}, //deprecated
	{
		gsKey:  "gtk-auto-mnemonics",
		xsKey:  "Gtk/AutoMnemonics",
		gsType: gsKeyTypeBool,
	}, //deprecated
	{
		gsKey:  "gtk-recent-files-enabled",
		xsKey:  "Gtk/RecentFilesEnabled",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "gtk-shell-shows-app-menu",
		xsKey:  "Gtk/ShellShowsAppMenu",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "xft-antialias",
		xsKey:  "Xft/Antialias",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:  "xft-hinting",
		xsKey:  "Xft/Hinting",
		gsType: gsKeyTypeBool,
	},
	{
		gsKey:         "qt-active-color",
		gsType:        gsKeyTypeString,
		xsKey:         "Qt/ActiveColor",
		xsType:        &settingTypeColorVar,
		convertGsToXs: convertStrToColor,
		convertXsToGs: convertColorToStr,
	},
	{
		gsKey:         "qt-dark-active-color",
		gsType:        gsKeyTypeString,
		xsKey:         "Qt/DarkActiveColor",
		xsType:        &settingTypeColorVar,
		convertGsToXs: convertStrToColor,
		convertXsToGs: convertColorToStr,
	},
	{
		gsKey:  "qt-font-name",
		gsType: gsKeyTypeString,
		xsKey:  "Qt/FontName",
	},
	{
		gsKey:  "qt-mono-font-name",
		gsType: gsKeyTypeString,
		xsKey:  "Qt/MonoFontName",
	},
	{
		gsKey:  "qt-font-point-size",
		gsType: gsKeyTypeDouble,
		xsKey:  "Qt/FontPointSize",
		// xsType is string
		convertGsToXs: convertDoubleToStr,
		convertXsToGs: convertStrToDouble,
	},
	{
		gsKey:  "dtk-window-radius",
		gsType: gsKeyTypeInt,
		xsKey:  "DTK/WindowRadius",
	},
	{
		gsKey:  "primary-monitor-name",
		gsType: gsKeyTypeString,
		xsKey:  "Gdk/PrimaryMonitorName",
	},
	{
		gsKey:  "dtk-size-mode",
		gsType: gsKeyTypeInt,
		xsKey:  "DTK/SizeMode",
	},
	{
		gsKey:  "qt-scrollbar-policy",
		gsType: gsKeyTypeInt,
		xsKey:  "Qt/ScrollBarPolicy",
	},
}

var settingTypeColorVar = settingTypeColor

func convertDoubleToStr(in interface{}) (interface{}, error) {
	value, ok := in.(float64)
	if !ok {
		return nil, errors.New("type is not float64")
	}
	return strconv.FormatFloat(value, 'f', -1, 64), nil
}

func convertStrToDouble(in interface{}) (interface{}, error) {
	value, ok := in.(string)
	if !ok {
		return nil, errors.New("type is not string")
	}
	return strconv.ParseFloat(value, 64)
}

func convertStrToColor(in interface{}) (interface{}, error) {
	str, ok := in.(string)
	if !ok {
		return nil, errors.New("type is not string")
	}

	fields := strings.Split(str, ",")
	if len(fields) != 4 {
		return nil, errors.New("length is not 4")
	}

	// R G B A
	var array [4]uint16
	for idx, field := range fields {
		fieldNum, err := strconv.ParseUint(field, 10, 16)
		if err != nil {
			return nil, err
		}
		array[idx] = uint16(fieldNum)
	}

	// 把值从 0~65535 范围内修正为 0~255 范围内
	for idx, value := range array {
		array[idx] = uint16((float64(value) / float64(math.MaxUint16)) * float64(math.MaxUint8))
	}
	return array, nil
}

func convertColorToStr(in interface{}) (interface{}, error) {
	array, ok := in.([4]uint16)
	if !ok {
		return nil, errors.New("type is not [4]uint16")
	}

	// 把值从 0~255 范围内修正为 0~65535 范围内
	for idx, value := range array {
		array[idx] = uint16((float64(value) / float64(math.MaxUint8) * float64(math.MaxUint16)))
	}

	return fmt.Sprintf("%d,%d,%d,%d", array[0], array[1],
		array[2], array[3]), nil
}

func (infos typeGSKeyInfos) getByGSKey(key string) *typeGSKeyInfo {
	for _, info := range infos {
		if key == info.gsKey {
			return &info
		}
	}

	return nil
}

func (infos typeGSKeyInfos) getByXSKey(key string) *typeGSKeyInfo {
	for _, info := range infos {
		if key == info.xsKey {
			return &info
		}
	}

	return nil
}

func (info *typeGSKeyInfo) getKeySType() uint8 {
	if info.xsType != nil {
		return *info.xsType
	}

	switch info.gsType {
	case gsKeyTypeBool, gsKeyTypeInt:
		return settingTypeInteger
	case gsKeyTypeString, gsKeyTypeDouble:
		return settingTypeString
	}

	return settingTypeInteger
}

func (info *typeGSKeyInfo) getValue(s *dconfig.DConfig) (result interface{}, err error) {
	switch info.gsType {
	case gsKeyTypeBool:
		v, _ := s.GetValueBool(info.gsKey)
		if v {
			result = int32(1)
		} else {
			result = int32(0)
		}
	case gsKeyTypeInt:
		intValue, _ := s.GetValueInt(info.gsKey)
		result = int32(intValue)
	case gsKeyTypeString:
		result, _ = s.GetValueString(info.gsKey)
	case gsKeyTypeDouble:
		result, _ = s.GetValueFloat64(info.gsKey)
	}

	if info.convertGsToXs != nil {
		result, err = info.convertGsToXs(result)
	}
	return
}

func (info *typeGSKeyInfo) setValue(s *dconfig.DConfig, v interface{}) error {
	var err error
	if info.convertXsToGs != nil {
		v, err = info.convertXsToGs(v)
		if err != nil {
			return err
		}
	}

	switch info.gsType {
	case gsKeyTypeBool:
		tmp := v.(int32)
		if tmp == 1 {
			s.SetValue(info.gsKey, true)
		} else {
			s.SetValue(info.gsKey, false)
		}
	case gsKeyTypeInt:
		s.SetValue(info.gsKey, v.(int32))
	case gsKeyTypeString:
		s.SetValue(info.gsKey, v.(string))
	case gsKeyTypeDouble:
		s.SetValue(info.gsKey, v.(float64))
	}
	return nil
}
