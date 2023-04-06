// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package lastore

import (
	"encoding/json"
	"errors"

	"github.com/godbus/dbus/v5"
)

type syncConfig struct {
	l *Lastore
}

type syncData struct {
	Version             string `json:"version"`
	AutoCheckUpdates    bool   `json:"auto_check_updates"`
	AutoClean           bool   `json:"auto_clean"`
	AutoDownloadUpdates bool   `json:"auto_download_updates"`
	SmartMirrorEnabled  bool   `json:"smart_mirror_enabled"`
}

const (
	syncVersion = "1.0"

	smartMirrorService = "org.deepin.dde.Lastore1.Smartmirror"
	smartMirrorPath    = "/org/deepin/dde/Lastore1/Smartmirror"
	smartMirrorIFC     = smartMirrorService
)

func (sc *syncConfig) Get() (interface{}, error) {
	var info syncData
	info.Version = syncVersion
	info.AutoCheckUpdates, _ = sc.l.core.Updater().AutoCheckUpdates().Get(0)
	info.AutoClean, _ = sc.l.core.Manager().AutoClean().Get(0)
	info.AutoDownloadUpdates, _ = sc.l.core.Updater().AutoDownloadUpdates().Get(0)
	info.SmartMirrorEnabled, _ = smartMirrorEnabledGet()
	return &info, nil
}

func (sc *syncConfig) Set(data []byte) error {
	var info syncData
	err := json.Unmarshal(data, &info)
	if err != nil {
		return err
	}
	err = sc.l.core.Updater().SetAutoCheckUpdates(0, info.AutoCheckUpdates)
	if err != nil {
		logger.Warning("Failed to set lastore auto check updates:", err)
	}
	err = sc.l.core.Manager().SetAutoClean(0, info.AutoClean)
	if err != nil {
		logger.Warning("Failed to set lastore auto clean:", err)
	}
	err = sc.l.core.Updater().SetAutoDownloadUpdates(0, info.AutoDownloadUpdates)
	if err != nil {
		logger.Warning("Failed to set lastore auto download updates:", err)
	}
	err = smartMirrorEnabledSet(info.SmartMirrorEnabled)
	if err != nil {
		logger.Warning("Failed to set lastore smart mirror:", err)
	}
	return nil
}

func smartMirrorEnabledSet(enabled bool) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	return conn.Object(smartMirrorService, smartMirrorPath).Call(
		smartMirrorIFC+".SetEnable", 0, enabled).Store()
}

func smartMirrorEnabledGet() (bool, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return false, err
	}
	var variant dbus.Variant
	err = conn.Object(smartMirrorService, smartMirrorPath).Call(
		"org.freedesktop.DBus.Properties.Get", 0, smartMirrorIFC, "Enable").Store(&variant)
	if err != nil {
		return false, err
	}

	if variant.Signature().String() != "b" {
		return false, errors.New("not excepted value type")
	}
	return variant.Value().(bool), nil
}
