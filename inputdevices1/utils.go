// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"errors"
	"os/exec"

	"github.com/linuxdeepin/dde-daemon/common/dconfig"
)

const (
	xsPropBlinkTimeut = "cursor-blink-time"
	xsPropDoubleClick = "double-click-time"
	xsPropDragThres   = "dnd-drag-threshold"
)

var (
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

// 统一的xsettings设置函数
func xsSetInt32(prop string, value int32) {
	// 同步到 xsettings 的 dconfig
	logger.Debugf("set xsettings %s to %d", prop, value)
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
		return errors.New(string(out))
	}
	return nil
}
