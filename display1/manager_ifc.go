// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/display1/brightness"
	"github.com/linuxdeepin/dde-daemon/display1/utils"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
)

func (m *Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) ApplyChanges() *dbus.Error {
	logger.Debug("dbus call ApplyChanges")
	err := m.applyChanges()
	return dbusutil.ToError(err)
}

func (m *Manager) ResetChanges() *dbus.Error {
	logger.Debug("dbus call ResetChanges")
	m.PropsMu.Lock()
	if !m.HasChanged {
		m.PropsMu.Unlock()
		return nil
	}
	m.setPropHasChanged(false)
	m.PropsMu.Unlock()

	m.monitorMapMu.Lock()
	for _, monitor := range m.monitorMap {
		monitor.resetChanges()
	}
	monitorMap := m.cloneMonitorMapNoLock()
	m.monitorMapMu.Unlock()

	monitorsId := getConnectedMonitors(monitorMap).getMonitorsId()
	// 简化实现主屏ID为0。
	err := m.apply(monitorsId, monitorMap, nil, 0, m.DisplayMode)
	if err != nil {
		return dbusutil.ToError(err)
	}

	return nil
}

func (m *Manager) SwitchMode(mode byte, name string) *dbus.Error {
	logger.Debug("dbus call SwitchMode", mode, name)
	err := m.switchMode(mode, name)
	return dbusutil.ToError(err)
}

func (m *Manager) Save() *dbus.Error {
	logger.Debug("dbus call Save")
	err := m.save()
	return dbusutil.ToError(err)
}

func (m *Manager) AssociateTouch(outputName, touchSerial string) *dbus.Error {
	var UUID string
	for _, v := range m.Touchscreens {
		if v.Serial == touchSerial {
			UUID = v.UUID
			break
		}
	}

	if UUID == "" {
		return dbusutil.ToError(errors.New("touchscreen not exists"))
	}

	monitor := m.getConnectedMonitors().GetByName(outputName)
	if monitor == nil {
		return dbusutil.ToError(errors.New("monitor not exists"))
	}

	err := m.associateTouch(monitor, UUID, false)
	return dbusutil.ToError(err)
}

func (m *Manager) AssociateTouchByUUID(outputName, touchUUID string) *dbus.Error {
	var UUID string
	for _, v := range m.Touchscreens {
		if v.UUID == touchUUID {
			UUID = v.UUID
			break
		}
	}

	if UUID == "" {
		return dbusutil.ToError(errors.New("touchscreen not exists"))
	}

	monitor := m.getConnectedMonitors().GetByName(outputName)
	if monitor == nil {
		return dbusutil.ToError(errors.New("monitor not exists"))
	}

	err := m.associateTouch(monitor, UUID, false)
	return dbusutil.ToError(err)
}

// ChangeBrightness 通过键盘控制所有显示器一起亮度加或减，保存配置。
func (m *Manager) ChangeBrightness(raised bool) *dbus.Error {
	logger.Debug("dbus call ChangeBrightness", raised)
	err := m.changeBrightness(raised)
	if err == nil {
		// 通知自动亮度管理器手动调节
		m.notifyManualBrightnessChange()
	}
	return dbusutil.ToError(err)
}

// SetAutoBrightnessEnabled 设置自动亮度启用状态
func (m *Manager) SetAutoBrightnessEnabled(enabled bool) *dbus.Error {
	logger.Debug("dbus call SetAutoBrightnessEnabled", enabled)
	if m.autoBrightnessManager == nil {
		return dbusutil.ToError(errors.New("auto brightness not supported"))
	}

	err := m.autoBrightnessManager.SetEnabled(enabled)
	if err != nil {
		return dbusutil.ToError(err)
	}

	m.setPropAutoBrightnessEnabled(enabled)

	return nil
}

func (m *Manager) GetBrightness() (map[string]float64, *dbus.Error) {
	m.PropsMu.RLock()
	defer m.PropsMu.RUnlock()
	return m.Brightness, nil
}

func (m *Manager) ListOutputNames() ([]string, *dbus.Error) {
	logger.Debug("dbus call ListOutputNames")
	var names []string

	monitors := m.getConnectedMonitors()
	for _, monitor := range monitors {
		names = append(names, monitor.Name)
	}
	return names, nil
}

// ListEffectiveOutputNames 返回当前所有有效显示器名
// X11 下使用 randr.GetMonitors 返回物理 + 虚拟屏
// Wayland 下复用 ListOutputNames 返回物理屏,
func (m *Manager) ListEffectiveOutputNames() ([]string, *dbus.Error) {
	logger.Debug("dbus call ListEffectiveOutputNames")
	if _useWayland {
		return m.ListOutputNames()
	}

	names, err := m.getEffectiveOutputNames()
	if err != nil {
		return nil, dbusutil.ToError(err)
	}
	return names, nil
}

func (m *Manager) ListOutputsCommonModes() ([]ModeInfo, *dbus.Error) {
	logger.Debug("dbus call ListOutputsCommonModes")
	monitors := m.getConnectedMonitors()
	if len(monitors) == 0 {
		return nil, nil
	}

	commonSizes := getMonitorsCommonSizes(monitors)
	result := make([]ModeInfo, len(commonSizes))
	for i, size := range commonSizes {
		result[i] = getFirstModeBySize(monitors[0].Modes, size.width, size.height)
	}
	return result, nil
}

// ModifyConfigName 废弃方法
func (m *Manager) ModifyConfigName(name, newName string) *dbus.Error {
	return dbusutil.ToError(errors.New("obsoleted method"))
}

// DeleteCustomMode 废弃方法
func (m *Manager) DeleteCustomMode(name string) *dbus.Error {
	return dbusutil.ToError(errors.New("obsoleted method"))
}

// RefreshBrightness 重置亮度，主要被 session/power 模块调用。从配置恢复亮度。
func (m *Manager) RefreshBrightness() *dbus.Error {
	logger.Debug("dbus call RefreshBrightness")
	if m.AutoBrightnessEnabled {
		logger.Debug("auto brightness enabled, skip RefreshBrightness")
		return nil
	}
	monitors := m.getConnectedMonitors()
	monitorsId := monitors.getMonitorsId()
	configs := m.getSuitableSysMonitorConfigs(m.DisplayMode, monitorsId, monitors)
	for _, config := range configs {
		if config.Enabled {
			err := m.setBrightness(config.Name, config.Brightness)
			if err != nil {
				logger.Warning(err)
			}
		}
	}
	m.syncPropBrightness()
	return nil
}

func (m *Manager) Reset() *dbus.Error {
	// TODO
	return nil
}

// SetAndSaveBrightness 设置并保持亮度
func (m *Manager) SetAndSaveBrightness(outputName string, value float64) *dbus.Error {
	logger.Debug("dbus call SetAndSaveBrightness", outputName, value)
	can, _ := m.CanSetBrightness(outputName)
	if !can {
		return dbusutil.ToError(fmt.Errorf("the port %s cannot set brightness", outputName))
	}
	err := m.setBrightnessAndSync(outputName, value)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	// 通知自动亮度管理器手动调节
	m.notifyManualBrightnessChange()

	err = m.saveBrightnessInCfg(map[string]float64{
		outputName: value,
	})
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	return nil
}

// SetBrightness 设置亮度但是不保存, 主要被 session/power 模块调用。
func (m *Manager) SetBrightness(outputName string, value float64) *dbus.Error {
	logger.Debug("dbus call SetBrightness", outputName, value)
	if value > 1 || value < 0 {
		return dbusutil.ToError(fmt.Errorf("the brightness value range is 0-1"))
	}

	can, _ := m.CanSetBrightness(outputName)
	if !can {
		return dbusutil.ToError(fmt.Errorf("the port %s cannot set brightness", outputName))
	}

	err := m.setBrightnessAndSync(outputName, value)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	// 通知自动亮度管理器手动调节
	m.notifyManualBrightnessChange()

	return nil
}

func (m *Manager) SetPrimary(outputName string) *dbus.Error {
	logger.Debug("dbus call SetPrimary", outputName)
	err := m.setPrimary(outputName)
	return dbusutil.ToError(err)
}

func (m *Manager) SetConcatScreen(enable bool) *dbus.Error {
	if _useWayland {
		return dbusutil.ToError(errors.New("concat screen not supported on wayland"))
	}

	if enable {
		m.PropsMu.RLock()
		displayMode := m.DisplayMode
		m.PropsMu.RUnlock()
		if displayMode != DisplayModeExtend {
			return dbusutil.ToError(errors.New("concat screen is only supported in extend mode"))
		}
		if err := m.applyConcatScreen(); err != nil {
			return dbusutil.ToError(err)
		}
	} else {
		if err := m.removeConcatScreen(); err != nil {
			return dbusutil.ToError(err)
		}
	}
	return nil
}

func (m *Manager) CanRotate() (bool, *dbus.Error) {
	if os.Getenv("DEEPIN_DISPLAY_DISABLE_ROTATE") == "1" {
		return false, nil
	}
	return true, nil
}

func (m *Manager) canSetBrightnessDDCCI(name string) (ret bool) {
	if m.CanSetBrightnessMap == nil {
		logger.Warning("canSetBrightness, return false -> CanSetBrightnessMap is nil ")
		return ret
	}
	m.brightnessMapMu.RLock()
	value, ok := m.CanSetBrightnessMap[name]
	m.brightnessMapMu.RUnlock()
	ret = value
	if !ok {
		logger.Warning("canSetBrightness, return false -> CanSetBrightnessMap not have the monitor : ", name)
		return false
	}
	return ret
}

func (m *Manager) canSetBrightnessGeneral(name string) (ret bool) {
	// 如果是龙芯集显，且不是内置显示器，则不支持调节亮度
	if os.Getenv("CAN_SET_BRIGHTNESS") == "N" {
		if m.builtinMonitor == nil || m.builtinMonitor.Name != name {
			return false
		}
	}

	return true
}

// copyCanSetBrightnessMapLocked 返回 CanSetBrightnessMap 的快照，调用方必须持有
// brightnessMapMu（读或写锁）。发出 D-Bus 属性变更信号前先拷贝快照，避免信号
// 处理期间其他 goroutine 并发 delete 触发 map 读写的 fatal error。
func (m *Manager) copyCanSetBrightnessMapLocked() map[string]bool {
	snapshot := make(map[string]bool, len(m.CanSetBrightnessMap))
	for k, v := range m.CanSetBrightnessMap {
		snapshot[k] = v
	}
	return snapshot
}

func (m *Manager) checkMonitorConnection(id uint32) bool {
	m.monitorMapMu.Lock()
	monitor, ok := m.monitorMap[id]
	m.monitorMapMu.Unlock()
	if !ok || monitor == nil {
		return false
	}
	monitor.PropsMu.RLock()
	defer monitor.PropsMu.RUnlock()
	return monitor.realConnected
}

// updateCanSetBrightnessAsync 延迟检测显示器亮度调节能力并更新 CanSetBrightnessMap。
// 在屏幕接入后调用：等待 dsgCanSetBrightnessInterval 毫秒后，根据是否走 DDCCI
func (m *Manager) updateCanSetBrightnessAsync(monitor *Monitor) {
	edid := utils.EncodeEdidBase64(monitor.edid)
	name := monitor.Name
	id := monitor.ID
	time.AfterFunc(time.Duration(m.dsgCanSetBrightnessInterval)*time.Millisecond, func() {
		if !m.checkMonitorConnection(id) {
			return
		}
		var canSetBrightness bool
		if m.shouldUseDDCCIBrightness(name) {
			// 屏幕接入且走 DDCCI 路径：刷新显示列表后检测支持情况
			canSetBrightness = brightness.RefreshAndSupportDDCCIBrightness(edid)
		} else {
			canSetBrightness = m.canSetBrightnessGeneral(name)
		}
		m.brightnessMapMu.Lock()
		m.CanSetBrightnessMap[name] = canSetBrightness
		snapshot := m.copyCanSetBrightnessMapLocked()
		m.brightnessMapMu.Unlock()
		m.emitPropChangedCanSetBrightnessMap(snapshot)
		logger.Infof("updateCanSetBrightness: %s, canSetBrightness : %v", name, canSetBrightness)
	})
}

// removeCanSetBrightness 删除指定显示器的亮度能力条目并发出 D-Bus 信号。
// 用于屏幕断开连接时清理 CanSetBrightnessMap。
func (m *Manager) removeCanSetBrightness(name string) {
	m.brightnessMapMu.Lock()
	delete(m.CanSetBrightnessMap, name)
	snapshot := m.copyCanSetBrightnessMapLocked()
	m.brightnessMapMu.Unlock()
	m.emitPropChangedCanSetBrightnessMap(snapshot)
}

func (m *Manager) canSetBrightness(name string) (ret bool) {
	if m.shouldUseDDCCIBrightness(name) {
		ret = m.canSetBrightnessDDCCI(name)
		logger.Debug("canSetBrightness : ", name, ret)
		return ret
	}

	ret = m.canSetBrightnessGeneral(name)
	logger.Debug("canSetBrightness : ", name, ret)
	return ret
}

func (m *Manager) CanSetBrightness(outputName string) (bool, *dbus.Error) {
	if outputName == "" {
		return false, dbusutil.ToError(errors.New("monitor Name is err"))
	}

	return m.canSetBrightness(outputName), nil
}

func (m *Manager) getBuiltinMonitor() *Monitor {
	m.builtinMonitorMu.Lock()
	defer m.builtinMonitorMu.Unlock()
	return m.builtinMonitor
}

func (m *Manager) GetBuiltinMonitor() (string, dbus.ObjectPath, *dbus.Error) {
	builtinMonitor := m.getBuiltinMonitor()
	if builtinMonitor == nil {
		return "", "/", nil
	}

	m.monitorMapMu.Lock()
	_, ok := m.monitorMap[builtinMonitor.ID]
	m.monitorMapMu.Unlock()
	if !ok {
		return "", "/", dbusutil.ToError(fmt.Errorf("not found monitor %d", builtinMonitor.ID))
	}

	return builtinMonitor.Name, builtinMonitor.getPath(), nil
}

func (m *Manager) SetMethodAdjustCCT(adjustMethod int32) *dbus.Error {
	err := m.setColorTempMode(adjustMethod)
	return dbusutil.ToError(err)
}

func (m *Manager) SetColorTemperature(value int32) *dbus.Error {
	err := m.setColorTempValue(value)
	return dbusutil.ToError(err)
}

func (m *Manager) GetRealDisplayMode() (uint8, *dbus.Error) {
	monitors := m.getConnectedMonitors()

	mode := DisplayModeUnknown
	var pairs strv.Strv
	for _, m := range monitors {
		if !m.Enabled {
			continue
		}

		pair := fmt.Sprintf("%d,%d", m.X, m.Y)

		// 左上角座标相同，是复制
		if pairs.Contains(pair) {
			mode = DisplayModeMirror
		}

		pairs = append(pairs, pair)
	}

	if mode == DisplayModeUnknown && len(pairs) != 0 {
		if len(pairs) == 1 {
			mode = DisplayModeOnlyOne
		} else {
			mode = DisplayModeExtend
		}
	}

	return mode, nil
}

func (m *Manager) SupportSetColorTemperature() (bool, *dbus.Error) {
	return !(m.isVM || !m.drmSupportGamma), nil
}

func (m *Manager) SetCustomColorTempTimePeriod(timePeriod string) *dbus.Error {
	err := m.setCustomColorTempTimePeriod(timePeriod)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	return nil
}
