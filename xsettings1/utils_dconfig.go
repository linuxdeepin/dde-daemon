// SPDX-FileCopyrightText: 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"github.com/godbus/dbus/v5"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
)

type DConfig struct {
	conn    *dbus.Conn
	c       configManager.Manager
	gs      *GSConfig //for sync
	keyList []string
}

func NewDSConfig(conn *dbus.Conn) *DConfig {
	dsg := configManager.NewConfigManager(conn)
	XSettingsConfigManagerPath, err := dsg.AcquireManager(0, dsettingsAppID, dsettingsXSettingsName, "")
	if err != nil {
		logger.Warning(err)
		return nil
	}
	dConfigManager, err := configManager.NewManager(conn, XSettingsConfigManagerPath)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	keyList, _ := dConfigManager.KeyList().Get(0)
	return &DConfig{
		c:       dConfigManager,
		conn:    conn,
		keyList: keyList,
		gs:      NewGSConfig(),
	}
}

func (d *DConfig) ListKeys() []string {
	return d.keyList
}

func (d *DConfig) hasKey(key string) bool {
	if !strv.Strv(d.keyList).Contains(key) {
		logger.Warningf("key %v not found in dconfig", key)
		return false
	}
	return true
}

func (d *DConfig) GetString(key string) string {
	if !d.hasKey(key) {
		return ""
	}
	value, err := d.c.Value(0, key)
	if err != nil {
		logger.Warning(err)
		return ""
	}
	v, ok := value.Value().(string)
	if !ok {
		logger.Warningf("key %v type is not string, real is:%T", key, value.Value())
		return ""
	}
	return v
}

func (d *DConfig) GetInt(key string) int32 {
	if !d.hasKey(key) {
		return -1
	}
	value, err := d.c.Value(0, key)
	if err != nil {
		logger.Warning(err)
		return -1
	}
	v, ok := value.Value().(int64)
	if !ok {
		logger.Warningf("key %v type is not int, real is:%T", key, value.Value())
		return -1
	}
	return int32(v)
}

func (d *DConfig) GetBoolean(key string) bool {
	if !d.hasKey(key) {
		return false
	}
	value, err := d.c.Value(0, key)
	if err != nil {
		logger.Warning(err)
		return false
	}
	v, ok := value.Value().(bool)
	if !ok {
		logger.Warningf("key %v type is not bool, real is:%T", key, value.Value())
		return false
	}
	return v
}

func (d *DConfig) GetDouble(key string) float64 {
	if !d.hasKey(key) {
		return -1
	}
	value, err := d.c.Value(0, key)
	if err != nil {
		logger.Warning(err)
		return -1
	}
	v, ok := value.Value().(float64)
	if !ok {
		// TODO: dconfig float64的类型处理有问题，待该问题解决后去掉该代码
		logger.Warningf("key %v type is not float64, real is:%T", key, value.Value())
		v1, ok := value.Value().(int64)
		if !ok {
			logger.Warningf("key %v type is not int64, real is:%T", key, value.Value())
			return -1
		}
		v = float64(v1)
	}
	return v
}

func (d *DConfig) SetString(key string, value string) bool {
	if !d.hasKey(key) {
		return false
	}
	if err := d.c.SetValue(0, key, dbus.MakeVariant(value)); err != nil {
		logger.Warning(err)
		return false
	}
	if d.gs != nil {
		d.gs.SetString(key, value)
	}
	return true
}

func (d *DConfig) SetInt(key string, value int32) bool {
	if !d.hasKey(key) {
		return false
	}
	if err := d.c.SetValue(0, key, dbus.MakeVariant(value)); err != nil {
		logger.Warning(err)
		return false
	}
	if d.gs != nil {
		d.gs.SetInt(key, value)
	}
	return true
}

func (d *DConfig) SetBoolean(key string, value bool) bool {
	if !d.hasKey(key) {
		return false
	}
	if err := d.c.SetValue(0, key, dbus.MakeVariant(value)); err != nil {
		logger.Warning(err)
		return false
	}
	if d.gs != nil {
		d.gs.SetBoolean(key, value)
	}
	return true
}

func (d *DConfig) SetDouble(key string, value float64) bool {
	if !d.hasKey(key) {
		return false
	}
	if err := d.c.SetValue(0, key, dbus.MakeVariant(value)); err != nil {
		logger.Warning(err)
		return false
	}
	if d.gs != nil {
		d.gs.SetDouble(key, value)
	}
	return true
}

func (d *DConfig) HandleConfigChanged(cb func(string)) {
	systemSigLoop := dbusutil.NewSignalLoop(d.conn, 10)
	systemSigLoop.Start()
	d.c.InitSignalExt(systemSigLoop, true)
	_, _ = d.c.ConnectValueChanged(cb)
}
