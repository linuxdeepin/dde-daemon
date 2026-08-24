// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
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

	// dconfig 配置项
	_dsettingsPS2MouseAsTouchpadKey = "ps2MouseAsTouchPadEnabled"
)

// generateUdevRuleContent 根据配置生成 udev 规则内容
func generateUdevRuleContent() string {
	baseRule := `# DDE - Disable touchpad via libinput
# 标准触控板设备
SUBSYSTEM=="input", KERNEL=="event*", ENV{ID_INPUT_TOUCHPAD}=="1", ENV{LIBINPUT_IGNORE_DEVICE}="1"
`

	// 检查是否启用 PS/2 鼠标作为触控板的功能
	if getPS2MouseAsTouchpadEnabled() {
		baseRule += `# PS/2 接口设备（通常是触控板被误识别为鼠标）- 大写
SUBSYSTEM=="input", KERNEL=="event*", ATTRS{name}=="*PS/2*", ENV{LIBINPUT_IGNORE_DEVICE}="1"
# PS/2 接口设备（通常是触控板被误识别为鼠标）- 小写
SUBSYSTEM=="input", KERNEL=="event*", ATTRS{name}=="*ps/2*", ENV{LIBINPUT_IGNORE_DEVICE}="1"
`
	}

	return baseRule
}

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
	changed := t.setPropEnable(enabled)
	if !changed && enabled {
		return t.refreshTouchpadDevices()
	}
	if !changed {
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

func (t *Touchpad) refreshTouchpadDevices() error {
	if err := reloadUdevRules(); err != nil {
		logger.Warning("failed to reload udev rules:", err)
	}
	if err := t.triggerTouchpadDevices(); err != nil {
		logger.Warning("failed to trigger touchpad devices:", err)
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
		return t.refreshTouchpadDevices()
	}

	// 禁用：写入 udev 规则文件；若内容未变则无需刷新设备
	changed, err := writeUdevRuleFile(generateUdevRuleContent())
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return t.refreshTouchpadDevices()
}

// writeUdevRuleFile 以 0644 权限原子地写入 udev 规则文件，并确保数据与目录项落盘。
// 返回 changed 表示文件内容是否发生变化（true=已写入或新建，false=内容未变跳过写入）。
// 安全要点：
//   - fast path 内容已正确时仍收窄权限，防止外部改宽的 0666 等遗留
//   - 用 O_EXCL 原子探测是否为首次新建，避免 Stat/OpenFile TOCTOU 竞态
//   - 先 Chmod 成功再 Truncate，避免权限修改失败时 O_TRUNC 已清空原内容
//   - 文件数据 fsync 后，首次创建还需 fsync 父目录以保证目录项断电不丢
func writeUdevRuleFile(udevRuleContent string) (changed bool, err error) {
	// 内容已正确时，仅做权限兜底与父目录 best-effort sync 后直接返回
	if existingContent, rerr := os.ReadFile(udevRuleFile); rerr == nil &&
		string(existingContent) == udevRuleContent {
		if err := os.Chmod(udevRuleFile, 0644); err != nil {
			return false, err
		}
		syncDirBestEffort(filepath.Dir(udevRuleFile))
		logger.Debug("udev rule file already exists with correct content, skip writing")
		return false, nil
	}

	// 尝试以 O_EXCL 独占创建，原子判断是否为首次新建，
	// 避免 Stat/OpenFile 之间因并发删除导致的 TOCTOU 竞态使父目录漏 sync
	f, err := os.OpenFile(udevRuleFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	isNew := err == nil
	if err != nil && !os.IsExist(err) {
		return false, err
	}
	if os.IsExist(err) {
		// 文件已存在，以 O_WRONLY 打开但不立即截断
		f, err = os.OpenFile(udevRuleFile, os.O_WRONLY, 0644)
		if err != nil {
			return false, err
		}
	}
	// 先收窄权限，成功后再截断写入，避免 Chmod 失败时 O_TRUNC 已清空原内容导致规则丢失
	if err := f.Chmod(0644); err != nil {
		f.Close()
		return false, err
	}
	if err := f.Truncate(0); err != nil {
		f.Close()
		return false, err
	}
	if _, err := f.Write([]byte(udevRuleContent)); err != nil {
		f.Close()
		return false, err
	}
	// fsync 确保文件数据落盘，
	// 防止强制关机（断电）时 page cache 丢失导致规则文件丢失
	if err := f.Sync(); err != nil {
		f.Close()
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	// 首次创建文件时，还需 fsync 父目录以确保目录项落盘，
	// 否则断电后文件数据虽已持久但目录项丢失，规则文件仍会缺失
	if isNew {
		dir, err := os.Open(filepath.Dir(udevRuleFile))
		if err != nil {
			return false, err
		}
		err = dir.Sync()
		dir.Close()
		if err != nil {
			return false, err
		}
	}
	logger.Info("created udev rule file:", udevRuleFile)
	return true, nil
}

// syncDirBestEffort 尽力同步目录元数据，失败仅记录不返回错误。
func syncDirBestEffort(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		logger.Warning("failed to sync parent directory:", err)
	}
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

func getPS2MouseAsTouchpadEnabled() bool {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning("failed to connect to system bus:", err)
		return true // 默认启用
	}
	ds := configManager.NewConfigManager(sysBus)
	confPath, err := ds.AcquireManager(0, _dsettingsAppID, _dsettingsInputdevicesName, "")
	if err != nil {
		logger.Warning("failed to acquire config manager:", err)
		return true // 默认启用
	}
	dsManager, err := configManager.NewManager(sysBus, confPath)
	if err != nil {
		logger.Warning("failed to create config manager:", err)
		return true // 默认启用
	}
	data, err := dsManager.Value(0, _dsettingsPS2MouseAsTouchpadKey)
	if err != nil {
		logger.Warning("failed to get ps2MouseAsTouchPadEnabled config:", err)
		return true // 默认启用
	}
	v, ok := data.Value().(bool)
	return !ok || v
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
