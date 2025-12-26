// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices1

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	touchpadDBusPath      = "/org/deepin/dde/InputDevices1/Touchpad"
	touchpadDBusInterface = "org.deepin.dde.InputDevices1.Touchpad"

	// udev 规则文件路径
	udevRuleFile = "/etc/udev/rules.d/90-dde-touchpad.rules"
	// udev 规则内容（禁用触控板）
	// ENV{LIBINPUT_IGNORE_DEVICE}="1": 让 libinput 忽略设备
	// 注意：这只影响 libinput，evtest 仍能读取内核事件
	udevRuleContent = `# DDE - Disable touchpad via libinput
SUBSYSTEM=="input", KERNEL=="event*", ENV{ID_INPUT_TOUCHPAD}=="1", ENV{LIBINPUT_IGNORE_DEVICE}="1"
`
)

type Touchpad struct {
	service     *dbusutil.Service
	Enable      bool
	DeviceList  []string
	udevMonitor *udevMonitor
}

func newTouchpad(service *dbusutil.Service) *Touchpad {
	t := &Touchpad{
		service: service,
		Enable:  getDsgConf(),
	}

	// 初始化 udev 监听器
	t.udevMonitor = newUdevMonitor(func(devices []string) {
		t.handleDeviceChange(devices)
	})

	// 初始化设备列表
	if t.udevMonitor != nil {
		devices := t.udevMonitor.enumerateDevices()
		t.setPropDeviceList(devices)
		logger.Infof("touchpad initialized with %d device(s)", len(devices))
	}

	return t
}

// handleDeviceChange 处理设备变化
func (t *Touchpad) handleDeviceChange(devices []string) {
	t.setPropDeviceList(devices)
	logger.Infof("touchpad devices updated: %d device(s)", len(devices))
}

func (t *Touchpad) SetTouchpadEnable(enabled bool) *dbus.Error {
	err := t.setTouchpadEnable(enabled)
	return dbusutil.ToError(err)
}

func (t *Touchpad) setTouchpadEnable(enabled bool) error {
	logger.Debugf("setTouchpadEnable: %v", enabled)
	if !t.setPropEnable(enabled) {
		return nil
	}

	// 1. 保存到 dconfig（持久化配置）
	err := setDsgConf(enabled)
	if err != nil {
		logger.Warning("failed to save to dconfig:", err)
		return err
	}

	// 2. 使用 udev 规则方案
	if err := t.setTouchpadEnableViaUdev(enabled); err != nil {
		logger.Warning("udev rules method failed:", err)
		return err
	}

	return nil
}

// setTouchpadEnableViaUdev 通过 udev 规则禁用/启用触控板
func (t *Touchpad) setTouchpadEnableViaUdev(enabled bool) error {
	if enabled {
		// 启用：删除 udev 规则文件
		if err := os.Remove(udevRuleFile); err != nil && !os.IsNotExist(err) {
			return err
		}
		logger.Info("removed udev rule file:", udevRuleFile)
	} else {
		// 禁用：检查文件是否已存在且内容相同
		existingContent, err := os.ReadFile(udevRuleFile)
		if err == nil && string(existingContent) == udevRuleContent {
			logger.Debug("udev rule file already exists with correct content, skip writing")
			return nil
		}

		// 创建或覆盖 udev 规则文件
		if err := os.WriteFile(udevRuleFile, []byte(udevRuleContent), 0644); err != nil {
			return err
		}
		logger.Info("created udev rule file:", udevRuleFile)
	}

	// 重新加载 udev 规则
	if err := reloadUdevRules(); err != nil {
		logger.Warning("failed to reload udev rules:", err)
	}

	// 只触发触控板设备，减少不必要的事件
	if err := t.triggerTouchpadDevices(); err != nil {
		logger.Warning("failed to trigger touchpad devices:", err)
	}

	return nil
}

// reloadUdevRules 重新加载 udev 规则
func reloadUdevRules() error {
	cmd := exec.Command("udevadm", "control", "--reload-rules")
	if err := cmd.Run(); err != nil {
		return err
	}
	logger.Info("udev rules reloaded")
	return nil
}

// triggerTouchpadDevices 只触发触控板设备，减少不必要的事件
func (t *Touchpad) triggerTouchpadDevices() error {
	touchpadNames := t.DeviceList
	if len(touchpadNames) == 0 {
		logger.Warning("no touchpad devices to trigger")
		return nil
	}

	sysInputPath := "/sys/class/input"
	files, err := os.ReadDir(sysInputPath)
	if err != nil {
		return err
	}

	count := 0
	for _, file := range files {
		// 只处理 event 设备
		if !strings.HasPrefix(file.Name(), "event") {
			continue
		}

		// 读取设备名称
		namePath := filepath.Join(sysInputPath, file.Name(), "device/name")
		nameBytes, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}

		name := strings.TrimSpace(string(nameBytes))

		// 检查是否是触控板设备
		isTouchpad := false
		for _, touchpadName := range touchpadNames {
			if name == touchpadName {
				isTouchpad = true
				break
			}
		}

		if !isTouchpad {
			continue
		}

		// 只触发这个触控板设备
		cmd := exec.Command("udevadm", "trigger", "--action=change", "--sysname="+file.Name())
		if err := cmd.Run(); err != nil {
			logger.Warningf("failed to trigger %s: %v", file.Name(), err)
		} else {
			count++
			logger.Debugf("triggered touchpad device: %s (%s)", file.Name(), name)
		}

	}

	if count > 0 {
		logger.Infof("triggered %d touchpad device(s)", count)
	} else {
		logger.Warning("no touchpad devices found to trigger")
	}

	return nil
}

func setDsgConf(enable bool) error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	ds := configManager.NewConfigManager(sysBus)
	confPath, err := ds.AcquireManager(0, _dsettingsAppID, _dsettingsInputdevicesName, "")
	if err != nil {
		return err
	}
	dsManager, err := configManager.NewManager(sysBus, confPath)
	if err != nil {
		return err
	}
	err = dsManager.SetValue(0, _dsettingsTouchpadEnabledKey, dbus.MakeVariant(enable))
	if err != nil {
		return err
	}
	return nil
}

func getDsgConf() bool {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return false
	}
	ds := configManager.NewConfigManager(sysBus)
	confPath, err := ds.AcquireManager(0, _dsettingsAppID, _dsettingsInputdevicesName, "")
	if err != nil {
		return false
	}
	dsManager, err := configManager.NewManager(sysBus, confPath)
	if err != nil {
		return false
	}
	data, err := dsManager.Value(0, _dsettingsTouchpadEnabledKey)
	if err != nil {
		return false
	}
	return data.Value().(bool)
}

func (t *Touchpad) GetInterfaceName() string {
	return touchpadDBusInterface
}

func (t *Touchpad) export(path dbus.ObjectPath) error {
	return t.service.Export(path, t)
}

// setPropDeviceList 设置 DeviceList 属性并发送信号
func (t *Touchpad) setPropDeviceList(devices []string) {
	t.DeviceList = devices
	// 发送属性变化信号
	_ = t.service.EmitPropertyChanged(t, "DeviceList", devices)
}

// destroy 销毁触控板对象
func (t *Touchpad) destroy() {
	if t.udevMonitor != nil {
		t.udevMonitor.destroy()
		t.udevMonitor = nil
	}
}
