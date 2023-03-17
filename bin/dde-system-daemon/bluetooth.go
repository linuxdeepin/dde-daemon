// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"path/filepath"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/keyfile"
)

const (
	bluetoothPrefixDir = "/var/lib/bluetooth"

	kfSectionGeneral  = "General"
	kfKeyTechnologies = "SupportedTechnologies"
)

func (*Daemon) BluetoothGetDeviceTechnologies(adapter, device string) (technologies []string, busErr *dbus.Error) {
	var filename = filepath.Join(bluetoothPrefixDir, adapter, device, "info")
	technologies, err := doBluetoothGetDeviceTechnologies(filename)
	if err != nil {
		return nil, dbusutil.ToError(err)
	}
	return technologies, nil
}

func doBluetoothGetDeviceTechnologies(filename string) ([]string, error) {
	var kf = keyfile.NewKeyFile()
	err := kf.LoadFromFile(filename)
	if err != nil {
		return nil, err
	}
	return kf.GetStringList(kfSectionGeneral, kfKeyTechnologies)
}
