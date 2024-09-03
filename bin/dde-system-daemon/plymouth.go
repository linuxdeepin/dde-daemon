// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"sync"

	"github.com/godbus/dbus/v5"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

var plymouthLocker sync.Mutex

const (
	dsettingsAppID                       = "org.deepin.dde.daemon"
	dsettingsAppearanceName              = "org.deepin.dde.daemon.appearance"
	dsettingsScaleWithoutPlymouthEnabled = "scaleWithoutPlymouthEnabled"
)

func (d *Daemon) getScaleWithoutPlymouthEnabled() bool {
	ds := configManager.NewConfigManager(d.systemSigLoop.Conn())

	appearancePath, err := ds.AcquireManager(0, dsettingsAppID, dsettingsAppearanceName, "")
	if err != nil {
		logger.Warning(err)
		return false
	}

	dsAppearance, err := configManager.NewManager(d.systemSigLoop.Conn(), appearancePath)
	if err != nil {
		logger.Warning(err)
		return false
	}

	v, err := dsAppearance.Value(0, dsettingsScaleWithoutPlymouthEnabled)
	if err != nil {
		logger.Warning(err)
		return false
	}

	if enabled, ok := v.Value().(bool); ok {
		return enabled
	}

	return false
}

func (d *Daemon) ScalePlymouth(scale uint32) *dbus.Error {
	return dbusutil.ToError(d.scalePlymouth(scale))
}

func (d *Daemon) scalePlymouth(scale uint32) error {
	if d.getScaleWithoutPlymouthEnabled() {
		logger.Info("skip scale plymouth")
		return nil
	}

	plymouthLocker.Lock()
	defer plymouthLocker.Unlock()
	defer logger.Debug("end ScalePlymouth", scale)

	var (
		out    []byte
		kernel []byte
		err    error
	)

	// TODO: inhibit poweroff
	switch scale {
	case 1:
		var name = "uos-ssd-logo"
		//if isSSD() {
		//	name = "deepin-ssd-logo" //}
		out, err = exec.Command("plymouth-set-default-theme", name).CombinedOutput()
	case 2:
		var name = "uos-hidpi-ssd-logo"
		//if isSSD() {
		//	name = "deepin-hidpi-ssd-logo"
		//}
		out, err = exec.Command("plymouth-set-default-theme", name).CombinedOutput()
	default:
		return fmt.Errorf("invalid scale value: %d", scale)
	}

	if err != nil {
		return fmt.Errorf("failed to set plymouth theme: %s, err: %v", string(out), err)
	}

	kernel, err = exec.Command("uname", "-r").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get kernel, err: %v", err)
	}
	out, err = exec.Command("update-initramfs",
		"-u", "-k", string(bytes.TrimSpace(kernel))).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update initramfs: %s, err: %v", string(out), err)
	}

	return nil
}
