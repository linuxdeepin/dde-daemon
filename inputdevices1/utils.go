// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	"github.com/linuxdeepin/go-gir/gio-2.0"
)

const (
	xsettingsSchema   = "com.deepin.xsettings"
	xsPropBlinkTimeut = "cursor-blink-time"
	xsPropDoubleClick = "double-click-time"
	xsPropDragThres   = "dnd-drag-threshold"
)

var (
	xsLocker           sync.Mutex
	xsSetting          = gio.NewSettings(xsettingsSchema)
	dconf              *dconfig.DConfig
	watcherInitialized bool   // 防止重复初始化监听器
	globalMouse        *Mouse // 全局mouse引用，用于属性同步
)

// 初始化dconfig
func initDConfig() error {
	var err error
	dconf, err = dconfig.NewDConfig("org.deepin.dde.daemon", "org.deepin.XSettings", "")
	if err != nil {
		logger.Warning("init dconfig failed:", err)
		return err
	}

	if !watcherInitialized {
		watchDConfigChanges()
		watcherInitialized = true
		logger.Debug("dconfig watcher initialized")
	}

	return nil
}

// 监听dconfig变化
func watchDConfigChanges() {
	if dconf == nil {
		return
	}

	keys := []string{xsPropBlinkTimeut, xsPropDoubleClick, xsPropDragThres}
	for _, key := range keys {
		currentKey := key
		dconf.ConnectConfigChanged(currentKey, func(value interface{}) {
			var intValue int32
			switch v := value.(type) {
			case int:
				intValue = int32(v)
			case int32:
				intValue = v
			case int64:
				intValue = int32(v)
			default:
				logger.Warning("unexpected dconfig value type:", currentKey, value)
				return
			}

			logger.Debugf("dconfig changed: %s = %d", currentKey, intValue)

			xsLocker.Lock()
			xsSetting.SetInt(currentKey, intValue)
			xsLocker.Unlock()

			notifyMouseModuleUpdate(currentKey, intValue)
		})
	}
}

// 设置全局mouse引用
func SetGlobalMouse(mouse *Mouse) {
	globalMouse = mouse
}

// dconfig变化时通知mouse模块更新dbus属性
func notifyMouseModuleUpdate(key string, value int32) {
	if globalMouse == nil {
		return
	}

	switch key {
	case xsPropDoubleClick:
		globalMouse.DoubleClick.Set(value)
	case xsPropDragThres:
		globalMouse.DragThreshold.Set(value)
	}
}

// 启动时从dconfig同步到xsettings
func syncFromDConfig() {
	if dconf == nil {
		return
	}

	logger.Debug("syncing from dconfig to xsettings")
	keys := []string{xsPropBlinkTimeut, xsPropDoubleClick, xsPropDragThres}
	for _, key := range keys {
		if value, err := dconf.GetValueInt(key); err == nil && value > 0 {
			logger.Debugf("sync %s from dconfig: %d", key, value)
			xsLocker.Lock()
			xsSetting.SetInt(key, int32(value))
			xsLocker.Unlock()
		}
	}
}

// 统一的xsettings设置函数（以dconfig为准，并同步回dconfig）
func xsSetInt32(prop string, value int32) {
	xsLocker.Lock()
	defer xsLocker.Unlock()

	// 以dconfig为准：先检查dconfig的值
	if dconf != nil {
		if dconfigValue, err := dconf.GetValueInt(prop); err == nil && dconfigValue > 0 {
			if value != int32(dconfigValue) {
				logger.Debugf("using dconfig value for %s: %d (requested: %d)", prop, dconfigValue, value)
			}
			value = int32(dconfigValue)
		}
	}

	if value == xsSetting.GetInt(prop) {
		return
	}

	logger.Debugf("set xsettings %s to %d", prop, value)
	xsSetting.SetInt(prop, value)

	// 同步到dconfig
	if dconf != nil {
		if err := dconf.SetValue(prop, int(value)); err != nil {
			logger.Warning("sync to dconfig failed:", err)
		}
	}
}

func addItemToList(item string, list []string) ([]string, bool) {
	if isItemInList(item, list) {
		return list, false
	}

	list = append(list, item)
	return list, true
}

func delItemFromList(item string, list []string) ([]string, bool) {
	var (
		found bool
		ret   []string
	)

	for _, v := range list {
		if v == item {
			found = true
			continue
		}
		ret = append(ret, v)
	}

	return ret, found
}

func filterSpaceStr(list []string) []string {
	var ret []string
	for _, v := range list {
		if v != "" {
			ret = append(ret, v)
		}
	}
	return ret
}

func isItemInList(item string, list []string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

func doAction(cmd string) error {
	out, err := exec.Command("/bin/sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return fmt.Errorf(string(out))
	}
	return nil
}
