// SPDX-FileCopyrightText: 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/gsettings"
	"github.com/linuxdeepin/go-lib/strv"
)

type GSConfig struct {
	gs      *gio.Settings
	keyList []string
}

func NewGSConfig() *GSConfig {
	gs := gio.NewSettings(xsSchema)
	keyList := gs.ListKeys()
	return &GSConfig{
		gs:      gs,
		keyList: keyList,
	}
}

func (gc *GSConfig) ListKeys() []string {
	return gc.keyList
}

func (gc *GSConfig) hasKey(key string) bool {
	if !strv.Strv(gc.keyList).Contains(key) {
		logger.Warningf("key %v not found in dconfig", key)
		return false
	}
	return true
}

func (gc *GSConfig) GetString(key string) string {
	if !gc.hasKey(key) {
		return ""
	}
	return gc.gs.GetString(key)
}

func (gc *GSConfig) GetInt(key string) int32 {
	if !gc.hasKey(key) {
		return -1
	}
	return gc.gs.GetInt(key)
}

func (gc *GSConfig) GetBoolean(key string) bool {
	if !gc.hasKey(key) {
		return false
	}
	return gc.gs.GetBoolean(key)
}

func (gc *GSConfig) GetDouble(key string) float64 {
	if !gc.hasKey(key) {
		return -1
	}
	return gc.gs.GetDouble(key)
}

func (gc *GSConfig) SetString(key string, value string) bool {
	if !gc.hasKey(key) {
		return false
	}
	return gc.gs.SetString(key, value)
}

func (gc *GSConfig) SetInt(key string, value int32) bool {
	if !gc.hasKey(key) {
		return false
	}
	return gc.gs.SetInt(key, value)
}

func (gc *GSConfig) SetBoolean(key string, value bool) bool {
	if !gc.hasKey(key) {
		return false
	}
	return gc.gs.SetBoolean(key, value)
}

func (gc *GSConfig) SetDouble(key string, value float64) bool {
	if !gc.hasKey(key) {
		return false
	}
	return gc.gs.SetDouble(key, value)
}

func (d *GSConfig) HandleConfigChanged(cb func(string)) {
	gsettings.ConnectChanged(xsSchema, "*", cb)
}
