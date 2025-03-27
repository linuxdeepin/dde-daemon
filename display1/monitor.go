// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

const (
	dbusInterfaceMonitor = dbusInterface + ".Monitor"
)

const (
	fillModeDefault    string = "None"
	fillModeCenter     string = "Center"
	fillModeFull       string = "Full"
	fillModeFullaspect string = "Full aspect"
)

type Monitor struct {
	m       *Manager
	service *dbusutil.Service
	uuid    string // uuid v1
	uuidV0  string
	PropsMu sync.RWMutex

	ID            uint32
	Name          string
	Connected     bool
	realConnected bool
	Manufacturer  string
	Model         string
	// dbusutil-gen: equal=uint16SliceEqual
	Rotations []uint16
	// dbusutil-gen: equal=uint16SliceEqual
	Reflects []uint16
	BestMode ModeInfo
	// dbusutil-gen: equal=modeInfosEqual
	Modes []ModeInfo
	// dbusutil-gen: equal=modeInfosEqual
	PreferredModes []ModeInfo
	MmWidth        uint32
	MmHeight       uint32

	Enabled           bool
	X                 int16
	Y                 int16
	Width             uint16
	Height            uint16
	Rotation          uint16
	Reflect           uint16
	RefreshRate       float64
	Brightness        float64
	CurrentRotateMode uint8

	oldRotation uint16

	CurrentMode     ModeInfo
	CurrentFillMode string `prop:"access:rw"`
	// dbusutil-gen: equal=method:Equal
	AvailableFillModes strv.Strv

	backup *MonitorBackup
	// changes 记录 DBus 接口对显示器对象做的设置，也用 PropsMu 保护。
	changes monitorChanges
}

// monitorChanges 用于记录从 DBus 接收到的显示器新设置，key 是显示器属性名。
type monitorChanges map[string]interface{}

// monitorChanges 中 key 支持的的显示器属性名
const (
	monitorPropEnabled     = "Enabled"
	monitorPropX           = "X"
	monitorPropY           = "Y"
	monitorPropWidth       = "Width"
	monitorPropHeight      = "Height"
	monitorPropRotation    = "Rotation"
	monitorPropRefreshRate = "RefreshRate"
)

func (changes monitorChanges) clone() monitorChanges {
	if changes == nil {
		return nil
	}
	result := make(monitorChanges, len(changes))
	for name, value := range changes {
		result[name] = value
	}
	return result
}

// 合并新改变
func (m *Monitor) mergeChanges(newChanges monitorChanges) {
	// NOTE: 不用对 Monitor.PropsMu 加锁
	if m.changes == nil {
		m.changes = make(monitorChanges)
	}
	for name, value := range newChanges {
		m.changes[name] = value
	}
}

func (m *Monitor) String() string {
	return fmt.Sprintf("<Monitor id=%d name=%s>", m.ID, m.Name)
}

func (m *Monitor) GetInterfaceName() string {
	return dbusInterfaceMonitor
}

func (m *Monitor) getPath() dbus.ObjectPath {
	return dbus.ObjectPath(dbusPath + "/Monitor_" + strconv.Itoa(int(m.ID)))
}

func (m *Monitor) clone() *Monitor {
	m.PropsMu.RLock()
	defer m.PropsMu.RUnlock()

	monitorCp := Monitor{
		m:                  m.m,
		service:            m.service,
		uuid:               m.uuid,
		uuidV0:             m.uuidV0,
		ID:                 m.ID,
		Name:               m.Name,
		Connected:          m.Connected,
		realConnected:      m.realConnected,
		Manufacturer:       m.Manufacturer,
		Model:              m.Model,
		Rotations:          m.Rotations,
		Reflects:           m.Reflects,
		BestMode:           m.BestMode,
		Modes:              m.Modes,
		PreferredModes:     m.PreferredModes,
		MmWidth:            m.MmWidth,
		MmHeight:           m.MmHeight,
		Enabled:            m.Enabled,
		X:                  m.X,
		Y:                  m.Y,
		Width:              m.Width,
		Height:             m.Height,
		Rotation:           m.Rotation,
		Reflect:            m.Reflect,
		RefreshRate:        m.RefreshRate,
		Brightness:         m.Brightness,
		CurrentRotateMode:  m.CurrentRotateMode,
		oldRotation:        m.oldRotation,
		CurrentMode:        m.CurrentMode,
		CurrentFillMode:    m.CurrentFillMode,
		AvailableFillModes: m.AvailableFillModes,
		backup:             nil,
		changes:            m.changes.clone(),
	}

	return &monitorCp
}

func (m *Monitor) getUuids() strv.Strv {
	return []string{m.uuid, m.uuidV0}
}

type MonitorBackup struct {
	Enabled    bool
	Mode       ModeInfo
	X, Y       int16
	Reflect    uint16
	Rotation   uint16
	Brightness float64
}

func (m *Monitor) markChanged() {
	// NOTE: 不用加锁
	m.m.setPropHasChanged(true)
	if m.backup == nil {
		m.backup = &MonitorBackup{
			Enabled:    m.Enabled,
			Mode:       m.CurrentMode,
			X:          m.X,
			Y:          m.Y,
			Reflect:    m.Reflect,
			Rotation:   m.Rotation,
			Brightness: m.Brightness,
		}
	}
}

func (m *Monitor) Enable(enabled bool) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call Enable %v", m.ID, m.Name, enabled)
	if m.Enabled == enabled {
		return nil
	}

	m.markChanged()
	m.setPropEnabled(enabled)
	m.mergeChanges(monitorChanges{
		monitorPropEnabled: enabled,
	})
	return nil
}

func (m *Monitor) SetMode(mode uint32) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call SetMode %v", m.ID, m.Name, mode)
	return m.setModeNoLock(mode)
}

func (m *Monitor) setModeNoLock(mode uint32) *dbus.Error {
	if m.CurrentMode.Id == mode {
		return nil
	}

	newMode := findMode(m.Modes, mode)
	if newMode.isZero() {
		return dbusutil.ToError(errors.New("invalid mode"))
	}

	m.markChanged()
	m.setMode(newMode)
	m.mergeChanges(monitorChanges{
		monitorPropWidth:       m.Width,
		monitorPropHeight:      m.Height,
		monitorPropRefreshRate: m.RefreshRate,
	})
	return nil
}

func (m *Monitor) setMode(mode ModeInfo) {
	m.setPropCurrentMode(mode)

	width := mode.Width
	height := mode.Height

	swapWidthHeightWithRotation(m.Rotation, &width, &height)

	m.setPropWidth(width)
	m.setPropHeight(height)
	m.setPropRefreshRate(mode.Rate)
}

// setMode 的不发送属性改变信号版本
func (m *Monitor) setModeNoEmitChanged(mode ModeInfo) {
	m.CurrentMode = mode

	width := mode.Width
	height := mode.Height

	swapWidthHeightWithRotation(m.Rotation, &width, &height)

	m.Width = width
	m.Height = height
	m.RefreshRate = mode.Rate
}

func (m *Monitor) selectMode(width, height uint16, rate float64) ModeInfo {
	mode := getFirstModeBySizeRate(m.Modes, width, height, rate)
	if !mode.isZero() {
		return mode
	}
	mode = getFirstModeBySize(m.Modes, width, height)
	if !mode.isZero() {
		return mode
	}
	return m.BestMode
}

func (m *Monitor) SetModeBySize(width, height uint16) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call SetModeBySize %v %v", m.ID, m.Name, width, height)
	mode := getFirstModeBySize(m.Modes, width, height)
	if mode.isZero() {
		return dbusutil.ToError(errors.New("not found match mode"))
	}
	return m.setModeNoLock(mode.Id)
}

func (m *Monitor) SetRefreshRate(value float64) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call SetRefreshRate %v", m.ID, m.Name, value)
	if m.Width == 0 || m.Height == 0 {
		return dbusutil.ToError(errors.New("width or height is 0"))
	}
	mode := getFirstModeBySizeRate(m.Modes, m.Width, m.Height, value)
	if mode.isZero() {
		return dbusutil.ToError(errors.New("not found match mode"))
	}
	return m.setModeNoLock(mode.Id)
}

func (m *Monitor) SetPosition(X, y int16) *dbus.Error {
	logger.Debugf("monitor %v %v dbus call SetPosition %v %v", m.ID, m.Name, X, y)
	if _dpy == nil {
		return dbusutil.ToError(errors.New("_dpy is nil"))
	}

	if _dpy.getInApply() {
		logger.Debug("reject set position, in apply")
		return nil
	}

	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	if m.X == X && m.Y == y {
		logger.Debug("reject set position, no change")
		return nil
	}

	m.markChanged()
	m.setPropX(X)
	m.setPropY(y)
	m.mergeChanges(monitorChanges{
		monitorPropX: X,
		monitorPropY: y,
	})
	return nil
}

func (m *Monitor) SetReflect(value uint16) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call SetReflect %v", m.ID, m.Name, value)
	if m.Reflect == value {
		return nil
	}
	m.markChanged()
	m.setPropReflect(value)
	return nil
}

func (m *Monitor) SetRotation(value uint16) *dbus.Error {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	logger.Debugf("monitor %v %v dbus call SetRotation %v", m.ID, m.Name, value)
	if m.Rotation == value {
		return nil
	}
	m.oldRotation = m.Rotation
	m.markChanged()
	m.setRotation(value)
	m.mergeChanges(monitorChanges{
		monitorPropWidth:    m.Width,
		monitorPropHeight:   m.Height,
		monitorPropRotation: value,
	})
	m.setPropCurrentRotateMode(RotationFinishModeManual)
	return nil
}

func (m *Monitor) setRotation(value uint16) {
	width := m.CurrentMode.Width
	height := m.CurrentMode.Height

	swapWidthHeightWithRotation(value, &width, &height)

	m.setPropRotation(value)
	m.setPropWidth(width)
	m.setPropHeight(height)
}

func (m *Monitor) setPropBrightnessWithLock(value float64) {
	m.PropsMu.Lock()
	m.setPropBrightness(value)
	m.PropsMu.Unlock()
}

func (m *Monitor) resetChanges() {
	m.PropsMu.Lock()
	defer m.PropsMu.Unlock()

	if m.backup == nil {
		return
	}

	logger.Debug("restore from backup", m.ID)
	b := m.backup
	m.setPropEnabled(b.Enabled)
	m.setPropX(b.X)
	m.setPropY(b.Y)
	m.setPropRotation(b.Rotation)
	m.setPropReflect(b.Reflect)

	m.setPropCurrentMode(b.Mode)
	m.setPropWidth(b.Mode.Width)
	m.setPropHeight(b.Mode.Height)
	m.setPropRefreshRate(b.Mode.Rate)
	m.setPropBrightness(b.Brightness)

	m.backup = nil
}

func getRandrStatusStr(status uint8) string {
	switch status {
	case randr.SetConfigSuccess:
		return "success"
	case randr.SetConfigFailed:
		return "failed"
	case randr.SetConfigInvalidConfigTime:
		return "invalid config time"
	case randr.SetConfigInvalidTime:
		return "invalid time"
	default:
		return fmt.Sprintf("unknown status %d", status)
	}
}

func toSysMonitorConfigs(monitors []*Monitor, primary string) SysMonitorConfigs {
	found := false
	result := make(SysMonitorConfigs, len(monitors))
	for i, m := range monitors {
		cfg := m.toSysConfig()
		if !found && m.Name == primary {
			cfg.Primary = true
			found = true
		}
		result[i] = cfg
	}
	return result
}

func (m *Monitor) toBasicSysConfig() *SysMonitorConfig {
	return &SysMonitorConfig{
		UUID: m.uuid,
		Name: m.Name,
	}
}

func (m *Monitor) toSysConfig() *SysMonitorConfig {
	return &SysMonitorConfig{
		UUID:        m.uuid,
		Name:        m.Name,
		Enabled:     m.Enabled,
		X:           m.X,
		Y:           m.Y,
		Width:       m.Width,
		Height:      m.Height,
		Rotation:    m.Rotation,
		Reflect:     m.Reflect,
		RefreshRate: m.RefreshRate,
		Brightness:  m.Brightness,
	}
}

func (m *Monitor) dumpInfoForDebug() {
	logger.Debugf("dump info monitor %v %v, enabled: %v, uuid: %v, %v+%v,%vx%v, rotation: %v, reflect: %v, current mode: %+v",
		m.ID,
		m.Name,
		m.Enabled,
		m.uuid,
		m.X, m.Y, m.Width, m.Height,
		m.Rotation, m.Reflect,
		m.CurrentMode)
}

type Monitors []*Monitor

type monitorsId struct {
	v0, v1 string
}

func (monitors Monitors) getMonitorsId() monitorsId {
	if len(monitors) == 0 {
		return monitorsId{}
	}
	var idsV0 []string
	var idsV1 []string
	for _, monitor := range monitors {
		monitor.PropsMu.RLock()
		uuidV1 := monitor.uuid
		uuidV0 := monitor.uuidV0
		monitor.PropsMu.RUnlock()
		idsV0 = append(idsV0, uuidV0)
		idsV1 = append(idsV1, uuidV1)
	}
	sort.Strings(idsV0)
	sort.Strings(idsV1)
	return monitorsId{
		v0: strings.Join(idsV0, monitorsIdDelimiter),
		v1: strings.Join(idsV1, monitorsIdDelimiter),
	}
}

func (monitors Monitors) getPaths() []dbus.ObjectPath {
	sort.Slice(monitors, func(i, j int) bool {
		return monitors[i].ID < monitors[j].ID
	})
	paths := make([]dbus.ObjectPath, len(monitors))
	for i, monitor := range monitors {
		paths[i] = monitor.getPath()
	}
	return paths
}

func (monitors Monitors) GetByName(name string) *Monitor {
	for _, monitor := range monitors {
		if monitor.Name == name {
			return monitor
		}
	}
	return nil
}

func (monitors Monitors) GetById(id uint32) *Monitor {
	for _, monitor := range monitors {
		if monitor.ID == id {
			return monitor
		}
	}
	return nil
}

func (monitors Monitors) GetByUuid(uuid string) *Monitor {
	for _, monitor := range monitors {
		if monitor.getUuids().Contains(uuid) {
			return monitor
		}
	}
	return nil
}

const fillModeKeyDelimiter = ":"

func (m *Monitor) generateFillModeKey() string {
	width, height := m.Width, m.Height
	swapWidthHeightWithRotation(m.Rotation, &width, &height)
	return m.uuid + fillModeKeyDelimiter + fmt.Sprintf("%dx%d", width, height)
}

func (m *Monitor) setCurrentFillMode(write *dbusutil.PropertyWrite) *dbus.Error {
	value, _ := write.Value.(string)
	logger.Debugf("dbus call %v setCurrentFillMode %v", m, value)
	err := m.m.setMonitorFillMode(m, value)
	if err != nil {
		logger.Warning(err)
	}
	return dbusutil.ToError(err)
}

// MonitorInfo X 和 Wayland 共用的显示器信息
type MonitorInfo struct {
	// 只在 X 下使用
	crtc               randr.Crtc
	Enabled            bool
	ID                 uint32
	UUID               string // UUID V1
	UuidV0             string
	Name               string
	Connected          bool // 实际的是否连接，对应于 Monitor 的 realConnected
	VirtualConnected   bool // 用于前端，对应于 Monitor 的 Connected
	Modes              []ModeInfo
	CurrentMode        ModeInfo
	PreferredMode      ModeInfo
	X                  int16
	Y                  int16
	Width              uint16
	Height             uint16
	Rotation           uint16
	Rotations          uint16
	MmHeight           uint32
	MmWidth            uint32
	EDID               []byte
	Manufacturer       string
	Model              string
	CurrentFillMode    string
	AvailableFillModes []string
}

func (m *MonitorInfo) dumpForDebug() {
	logger.Debugf("MonitorInfo{crtc: %d,\nID: %v,\nName: %v,\nConnected: %v,\nVirtualConnected: %v,\n"+
		"CurrentMode: %v,\nPreferredMode: %v,\nX: %v, Y: %v, Width: %v, Height: %v,\nRotation: %v,\nRotations: %v,\n"+
		"MmWidth: %v,\nMmHeight: %v,\nModes: %v}",
		m.crtc, m.ID, m.Name, m.Connected, m.VirtualConnected,
		m.CurrentMode, m.PreferredMode, m.X, m.Y, m.Width, m.Height, m.Rotation, m.Rotations,
		m.MmWidth, m.MmHeight, m.Modes)
}

func (m *MonitorInfo) outputId() randr.Output {
	return randr.Output(m.ID)
}

func (m *MonitorInfo) equal(other *MonitorInfo) bool {
	return reflect.DeepEqual(m, other)
}

func (m *MonitorInfo) getRect() x.Rectangle {
	return x.Rectangle{
		X:      m.X,
		Y:      m.Y,
		Width:  m.Width,
		Height: m.Height,
	}
}
