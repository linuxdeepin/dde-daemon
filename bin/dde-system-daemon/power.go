// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/loader"
	"github.com/linuxdeepin/dde-daemon/securityloader"
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

type shortIdleController interface {
	ShortIdleState() (bool, error)
	SetShortIdleState(bool) error
}

func getShortIdleController() (shortIdleController, error) {
	module := loader.GetModule("power")
	if module == nil {
		return nil, errors.New("power module is not registered")
	}
	if !module.IsEnable() {
		return nil, errors.New("power module is not enabled")
	}
	controller, ok := module.(shortIdleController)
	if !ok {
		return nil, errors.New("power module does not support short idle control")
	}
	return controller, nil
}

func systemPowerSetShortIdleState(controller shortIdleController, state bool) error {
	logger.Info("systemPowerSetShortIdleState : ", state)
	if err := controller.SetShortIdleState(state); err != nil {
		return fmt.Errorf("failed to set short idle mode: %w", err)
	}
	return nil
}

// 1.设置 system/power 模块的短 idle 状态
// 2.写file内核文件
func (d *Daemon) setState(file string, state bool) error {
	if file != d.idleStatePath {
		return d.writeStateFile(file, state)
	}

	controller, err := getShortIdleController()
	if err != nil {
		return fmt.Errorf("failed to get short idle controller: %w", err)
	}
	return d.setStateWithController(controller, file, state)
}

func (d *Daemon) setStateWithController(controller shortIdleController, file string, state bool) error {
	d.idleStateMu.Lock()
	defer d.idleStateMu.Unlock()

	shortIdleState, err := controller.ShortIdleState()
	if err != nil {
		return fmt.Errorf("failed to get short idle state: %w", err)
	}
	logger.Infof("##### setState shortIdleState : %v, state : %v", shortIdleState, state)
	if shortIdleState == state {
		logger.Info("shortIdleState is same with state : ", state)
		return d.writeStateFile(file, state)
	}
	// 设置 system/power 模块的短 idle 状态
	if err := systemPowerSetShortIdleState(controller, state); err != nil {
		return err
	}
	return d.writeStateFile(file, state)
}

func (d *Daemon) writeStateFile(file string, state bool) error {
	// 写file内核文件
	if !utils.IsFileExist(file) {
		err := fmt.Errorf("%s not found", file)
		logger.Warning(err)
		return err
	}

	// 读取file文件内容
	content, err := os.ReadFile(file)
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
	err = os.WriteFile(file, []byte(newContent), 0644)
	if err != nil {
		logger.Errorf("Failed to write file %s: %v", file, err)
		return err
	}
	syscall.Sync()
	logger.Infof("Successfully updated %s with value: %d", file, newValue)
	return nil
}

func (d *Daemon) SetAllowCaller(sender dbus.Sender, uniqueName string) *dbus.Error {
	return dbusutil.ToError(d.allowCallers.AddCaller(securityloader.DaemonScope, sender, uniqueName))
}

func (d *Daemon) authorize(sender dbus.Sender, actionID string) error {
	return securityloader.AuthorizeWithPolkit(
		d.allowCallers,
		securityloader.DaemonScope,
		sender,
		d.service.Conn(),
		actionID,
	)
}

func (d *Daemon) SetIdleState(sender dbus.Sender, state bool) *dbus.Error {
	if err := d.authorize(sender, "org.deepin.dde.daemon.set-idle-state"); err != nil {
		logger.Warningf("SetIdleState authorization failed: %q", err.Error())
		return dbusutil.ToError(err)
	}
	logger.Infof("SetIdleState %s try set state: %v", d.idleStatePath, state)
	return dbusutil.ToError(d.setState(d.idleStatePath, state))
}

func (d *Daemon) SetScreenState(sender dbus.Sender, state bool) *dbus.Error {
	if err := d.authorize(sender, "org.deepin.dde.daemon.set-screen-state"); err != nil {
		logger.Warningf("SetScreenState authorization failed: %q", err.Error())
		return dbusutil.ToError(err)
	}
	logger.Infof("SetScreenState %s try set state: %v", d.idleScreenStatePath, state)
	return dbusutil.ToError(d.setState(d.idleScreenStatePath, state))
}