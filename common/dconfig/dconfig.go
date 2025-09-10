// SPDX-FileCopyrightText: 2024 - 2027 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package dconfig

import (
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
	DConfigManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

type DConfig struct {
	systemConn *dbus.Conn
	dbusPath   dbus.ObjectPath
	manager    DConfigManager.Manager

	configChangedCbMap      map[string]func(interface{})
	configChangedCbMapMutex sync.Mutex
	configChangedOnce       sync.Once
	signalLoopOnce          sync.Once
}

func NewDConfig(appid, name, subPath string) (*DConfig, error) {
	var dConfig DConfig
	var err error
	dConfig.systemConn, err = dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	dConfigManager := DConfigManager.NewConfigManager(dConfig.systemConn)
	dConfig.dbusPath, err = dConfigManager.AcquireManager(0, appid, name, subPath)
	if err != nil {
		return nil, err
	}
	dConfig.manager, err = DConfigManager.NewManager(dConfig.systemConn, dConfig.dbusPath)
	if err != nil {
		return nil, err
	}

	return &dConfig, nil
}

func (dConfig *DConfig) ListKeys() ([]string, error) {
	keys, err := dConfig.manager.KeyList().Get(0)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (dConfig *DConfig) Reset(key string) error {
	return dConfig.manager.Reset(0, key)
}

func (dConfig *DConfig) GetValueString(key string) (string, error) {
	value, err := dConfig.GetValue(key)
	if err != nil {
		return "", err
	}
	v, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("dconfig get string error: invalid value")
	}
	return v, nil
}

func (dConfig *DConfig) GetValueBool(key string) (bool, error) {
	value, err := dConfig.GetValue(key)
	if err != nil {
		return false, err
	}
	v, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("dconfig get bool error: invalid value")
	}
	return v, nil
}

func (dConfig *DConfig) GetValueInt(key string) (int, error) {
	v, err := dConfig.GetValueInt64(key)
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

func (dConfig *DConfig) GetValueInt64(key string) (int64, error) {
	value, err := dConfig.GetValue(key)
	if err != nil {
		return 0, err
	}
	v, ok := value.(int64)
	if !ok {
		return 0, fmt.Errorf("dconfig get int error: invalid value")
	}
	return v, nil
}

func (dConfig *DConfig) GetValueFloat64(key string) (float64, error) {
	value, err := dConfig.GetValue(key)

	if err != nil {
		return 0, err
	}
	switch v := value.(type) {
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		return 0, fmt.Errorf("dconfig get float64 error: invalid value (type %T)", value)
	}
}

func (dConfig *DConfig) GetValueStringList(key string) ([]string, error) {
	value, err := dConfig.GetValue(key)
	if err != nil {
		return nil, err
	}

	if variantSlice, ok := value.([]dbus.Variant); ok {
		result := make([]string, len(variantSlice))
		for i, variant := range variantSlice {
			if str, ok := variant.Value().(string); ok {
				result[i] = str
			} else {
				return nil, fmt.Errorf("variant at index %d is not a string: %T", i, variant.Value())
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("unsupported type for string slice conversion: %T", value)
}

func (dConfig *DConfig) GetValue(key string) (interface{}, error) {
	if dConfig.manager == nil {
		return nil, fmt.Errorf("dConfig not inited")
	}
	v, err := dConfig.manager.Value(0, key)
	if err != nil {
		return nil, err
	}
	return v.Value(), nil
}

func (dConfig *DConfig) SetValue(key string, value interface{}) error {
	if dConfig.manager == nil {
		return fmt.Errorf("dConfig not inited")
	}
	err := dConfig.manager.SetValue(0, key, dbus.MakeVariant(value))
	if err != nil {
		return err
	}
	return nil
}

func (dConfig *DConfig) initSignalLoopOnce() {
	dConfig.signalLoopOnce.Do(func() {
		systemSigLoop := dbusutil.NewSignalLoop(dConfig.systemConn, 10)
		systemSigLoop.Start()
		dConfig.manager.InitSignalExt(systemSigLoop, true)
	})
}

func (dConfig *DConfig) ConnectConfigChanged(key string, cb func(interface{})) {
	if dConfig.configChangedCbMap == nil {
		dConfig.configChangedCbMap = make(map[string]func(interface{}))
	}
	dConfig.configChangedCbMapMutex.Lock()
	dConfig.configChangedCbMap[key] = cb
	dConfig.configChangedCbMapMutex.Unlock()

	dConfig.initSignalLoopOnce()
	dConfig.configChangedOnce.Do(func() {
		dConfig.manager.ConnectValueChanged(func(key string) {
			dConfig.configChangedCbMapMutex.Lock()
			cb := dConfig.configChangedCbMap[key]
			dConfig.configChangedCbMapMutex.Unlock()
			if cb != nil {
				value, err := dConfig.GetValue(key)
				if err != nil {
					return
				}
				go cb(value)
			}
		})
	})
}

func (dConfig *DConfig) ConnectValueChanged(cb func(key string)) {
	dConfig.initSignalLoopOnce()
	dConfig.manager.ConnectValueChanged(cb)
}

func (dConfig *DConfig) ResetAll() error {
	keys, err := dConfig.ListKeys()
	if err != nil {
		return err
	}
	for _, key := range keys {
		dConfig.Reset(key)
	}
	return nil
}
