// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/godbus/dbus/v5"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/utils"
)

const (
	IdleFile       = "/sys/devices/system/loongarch/relax_state"
	IdleScreenFile = "/sys/devices/system/loongarch/idle_state"
)

const (
	dsettingsPowerName           = "org.deepin.dde.daemon.power"
	dsettingsIdleStatePath       = "idleStatePath"
	dsettingsIdleScreenStatePath = "idleScreenStatePath"
)

func isStrInList(item string, items []string) bool {
	for _, v := range items {
		if item == v {
			return true
		}
	}
	return false
}

func (d *Daemon) getDsgValue() {
	ds := configManager.NewConfigManager(d.systemSigLoop.Conn())

	powerPath, err := ds.AcquireManager(0, dsettingsSystemDaemonID, dsettingsPowerName, "")
	if err != nil {
		logger.Warning(err)
		return
	}

	dsPower, err := configManager.NewManager(d.systemSigLoop.Conn(), powerPath)
	if err != nil {
		logger.Warning(err)
		return
	}

	keyList, err := dsPower.KeyList().Get(0)
	if err != nil {
		logger.Warning(err)
	}

	if isStrInList(dsettingsIdleStatePath, keyList) {
		v, err := dsPower.Value(0, dsettingsIdleStatePath)
		if err != nil {
			logger.Warning(err)
		} else {
			if dsgIdleStatePath, ok := v.Value().(string); ok {
				d.idleStatePath = dsgIdleStatePath
				logger.Info("idleStatePath : ", d.idleStatePath)
			}
		}
	}

	if isStrInList(dsettingsIdleScreenStatePath, keyList) {
		v, err := dsPower.Value(0, dsettingsIdleScreenStatePath)
		if err != nil {
			logger.Warning(err)
		} else {
			if dsgIdleScreenStatePath, ok := v.Value().(string); ok {
				d.idleScreenStatePath = dsgIdleScreenStatePath
				logger.Info("idleScreenStatePath : ", d.idleScreenStatePath)
			}
		}
	}
}

// TODO: 临时方案，hwe一些机型内核wifi有问题，需要停止wifip2p扫描，待内核修改后去掉
func stopNetworkDisaplay() {
	err := exec.Command("killall", "deepin-network-display-daemon").Run()
	if err != nil {
		logger.Warning("Failed to stop network disaplay")
	}
}

func (d *Daemon) forwardPrepareForSleepSignal(service *dbusutil.Service) error {
	d.loginManager.InitSignalExt(d.systemSigLoop, true)

	_, err := d.loginManager.ConnectPrepareForSleep(func(before bool) {
		logger.Info("login1 PrepareForSleep", before)
		// signal `PrepareForSleep` true -> false
		if before {
			stopNetworkDisaplay()
		}
		err := service.Emit(d, "HandleForSleep", before)
		if err != nil {
			logger.Warning("failed to emit HandleForSleep signal")
			return
		}
	})
	if err != nil {
		logger.Warning("failed to ConnectPrepareForSleep")
		return err
	}
	return nil
}

func (d *Daemon) systemPowerSetShortIdleState(state bool) {
	logger.Info("systemPowerSetShortIdleState : ", state)
	if d.systemPower != nil {
		err := d.systemPower.SetShortIdleState(0, state)
		if err != nil {
			logger.Warning("failed to SetShortIdleState, err : ", err)
		}
	}
}

func (d *Daemon) setState(file string, state bool) error {
	shortIdleState, err := d.systemPower.ShortIdleState().Get(0)
	if err != nil {
		logger.Warning("Get systemPower.ShortIdleState err :", err)
	}
	logger.Infof("##### setState shortIdleState : %v, state : %v", shortIdleState, state)
	if shortIdleState == state {
		logger.Info("shortIdleState is same with state : ", state)
		return errors.New("Short idle state not exchange.")
	}
	if file == d.idleStatePath {
		d.systemPowerSetShortIdleState(state)
	}

	// 写file内核文件
	if !utils.IsFileExist(file) {
		err := fmt.Errorf("%s not found", file)
		logger.Warning(err)
		return err
	}

	// 读取file文件内容
	content, err := ioutil.ReadFile(file)
	if err != nil {
		logger.Errorf("Failed to read file %s: %v", file, err)
		return err
	}
	contentStr := strings.TrimSpace(string(content))

	// 如果不一致，将state的值写入file
	// 将true转换为1，false转换为0
	newValue := 0
	if state {
		newValue = 1
	}
	logger.Infof("Current content=%s, will set %v", contentStr, newValue)
	// 将值写入文件
	newContent := strconv.Itoa(newValue)
	err = ioutil.WriteFile(file, []byte(newContent), 0644)
	if err != nil {
		logger.Errorf("Failed to write file %s: %v", file, err)
		return err
	}
	syscall.Sync()
	logger.Infof("Successfully updated %s with value: %d", file, newValue)
	return nil
}

func (d *Daemon) SetIdleState(state bool) *dbus.Error {
	logger.Infof("SetIdleState %s try set state: %v", d.idleStatePath, state)
	return dbusutil.ToError(d.setState(d.idleStatePath, state))
}

func (d *Daemon) SetScreenState(state bool) *dbus.Error {
	logger.Infof("SetScreenState %s try set state: %v", d.idleScreenStatePath, state)
	return dbusutil.ToError(d.setState(d.idleScreenStatePath, state))
}
