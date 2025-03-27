// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"errors"
	"fmt"
	"github.com/linuxdeepin/dde-daemon/common/scale"
	"math"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/linuxdeepin/go-lib/log"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/input"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"github.com/linuxdeepin/go-x11-client/ext/xfixes"
)

var _hasRandr1d2 bool // 是否 randr 版本大于等于 1.2

var _useWayland bool

var _inVM bool

func Init(xConn *x.Conn, useWayland bool, inVM bool) {
	_xConn = xConn
	_useWayland = useWayland
	_inVM = inVM
	_hasRandr1d2 = scale.HasRandr1d2(xConn)

	var err error
	if _greeterMode {
		// 仅 greeter 需要
		_, err = xfixes.QueryVersion(xConn, xfixes.MajorVersion, xfixes.MinorVersion).Reply(xConn)
		if err != nil {
			logger.Warning(err)
		}

		_, err = input.XIQueryVersion(xConn, input.MajorVersion, input.MinorVersion).Reply(xConn)
		if err != nil {
			logger.Warning(err)
			return
		}
	}
}

const evMaskForHideCursor uint32 = input.XIEventMaskRawMotion | input.XIEventMaskRawTouchBegin

func (m *Manager) listenXEvents() {
	if _useWayland {
		return
	}
	eventChan := m.xConn.MakeAndAddEventChan(50)
	root := m.xConn.GetDefaultScreen().Root
	// 选择监听哪些 randr 事件
	err := randr.SelectInputChecked(m.xConn, root,
		randr.NotifyMaskOutputChange|randr.NotifyMaskOutputProperty|
			randr.NotifyMaskCrtcChange|randr.NotifyMaskScreenChange).Check(m.xConn)
	if err != nil {
		logger.Warning("failed to select randr event:", err)
		return
	}

	var inputExtData *x.QueryExtensionReply
	if _greeterMode {
		// 仅 greeter 需要
		err = m.doXISelectEvents(evMaskForHideCursor)
		if err != nil {
			logger.Warning(err)
		}
		inputExtData = m.xConn.GetExtensionData(input.Ext())
	}

	rrExtData := m.xConn.GetExtensionData(randr.Ext())

	go func() {
		for ev := range eventChan {
			switch ev.GetEventCode() {
			case randr.NotifyEventCode + rrExtData.FirstEvent:
				event, _ := randr.NewNotifyEvent(ev)
				switch event.SubCode {
				case randr.NotifyOutputChange:
					e, _ := event.NewOutputChangeNotifyEvent()
					m.mm.HandleEvent(e)

				case randr.NotifyCrtcChange:
					e, _ := event.NewCrtcChangeNotifyEvent()
					m.mm.HandleEvent(e)

				case randr.NotifyOutputProperty:
					e, _ := event.NewOutputPropertyNotifyEvent()
					// TODO mm 可能也应该处理这个事件
					m.handleOutputPropertyChanged(e)
				}

			case randr.ScreenChangeNotifyEventCode + rrExtData.FirstEvent:
				event, _ := randr.NewScreenChangeNotifyEvent(ev)
				cfgTsChanged := m.mm.HandleScreenChanged(event)
				m.handleScreenChanged(event, cfgTsChanged)

			case x.GeGenericEventCode:
				if !_greeterMode {
					continue
				}
				// 仅 greeter 处理这个事件
				geEvent, _ := x.NewGeGenericEvent(ev)
				if geEvent.Extension == inputExtData.MajorOpcode {
					switch geEvent.EventType {
					case input.RawMotionEventCode:
						m.beginMoveMouse()

					case input.RawTouchBeginEventCode:
						m.beginTouch()
					}
				}
			}
		}
	}()
}

type monitorManagerHooks interface {
	handleMonitorAdded(monitorInfo *MonitorInfo)
	handleMonitorRemoved(monitorId uint32)
	handleMonitorChanged(monitorInfo *MonitorInfo)
	handlePrimaryRectChanged(pmi primaryMonitorInfo)
	getMonitorsId() monitorsId
}

type monitorManager interface {
	setHooks(hooks monitorManagerHooks)
	getMonitors() []*MonitorInfo
	getMonitor(id uint32) *MonitorInfo
	apply(monitorsId monitorsId, monitorMap map[uint32]*Monitor, prevScreenSize screenSize, options applyOptions, fillModes map[string]string, primaryMonitorID uint32, displayMode byte) error
	setMonitorPrimary(monitorId uint32) error
	setMonitorFillMode(monitor *Monitor, fillMode string) error
	showCursor(show bool) error
	HandleEvent(ev interface{})
	HandleScreenChanged(e *randr.ScreenChangeNotifyEvent) (cfgTsChanged bool)
}

type xMonitorManager struct {
	hooks                   monitorManagerHooks
	mu                      sync.Mutex
	xConn                   *x.Conn
	hasRandr1d2             bool
	cfgTs                   x.Timestamp
	monitorsCache           []*MonitorInfo
	modes                   []randr.ModeInfo
	crtcs                   map[randr.Crtc]*CrtcInfo
	outputs                 map[randr.Output]*OutputInfo
	primary                 randr.Output
	monitorChangedCbEnabled bool
	// 键是 x 的 output 名称，值是标准名。
	stdNamesCache map[string]string
}

func newXMonitorManager(xConn *x.Conn, hasRandr1d2 bool) *xMonitorManager {
	xmm := &xMonitorManager{
		xConn:         xConn,
		hasRandr1d2:   hasRandr1d2,
		crtcs:         make(map[randr.Crtc]*CrtcInfo),
		outputs:       make(map[randr.Output]*OutputInfo),
		stdNamesCache: make(map[string]string),
	}
	err := xmm.init()
	if err != nil {
		logger.Warning("xMonitorManager init failed:", err)
	}
	return xmm
}

func (mm *xMonitorManager) setHooks(hooks monitorManagerHooks) {
	mm.hooks = hooks
}

type CrtcInfo randr.GetCrtcInfoReply

func (ci *CrtcInfo) getRect() x.Rectangle {
	rect := x.Rectangle{
		X:      ci.X,
		Y:      ci.Y,
		Width:  ci.Width,
		Height: ci.Height,
	}
	swapWidthHeightWithRotation(ci.Rotation, &rect.Width, &rect.Height)
	return rect
}

type OutputInfo randr.GetOutputInfoReply

func (oi *OutputInfo) PreferredMode() randr.Mode {
	return (*randr.GetOutputInfoReply)(oi).GetPreferredMode()
}

type primaryMonitorInfo struct {
	Name string
	Rect x.Rectangle
}

func (pmi primaryMonitorInfo) IsRectEmpty() bool {
	return pmi.Rect == x.Rectangle{}
}

func (mm *xMonitorManager) init() error {
	if !mm.hasRandr1d2 {
		return nil
	}
	xConn := mm.xConn
	resources, err := mm.getScreenResources(xConn)
	if err != nil {
		return err
	}
	mm.cfgTs = resources.ConfigTimestamp
	mm.modes = resources.Modes

	for _, outputId := range resources.Outputs {
		reply, err := mm.getOutputInfo(outputId)
		if err != nil {
			return err
		}
		mm.outputs[outputId] = (*OutputInfo)(reply)
	}

	for _, crtcId := range resources.Crtcs {
		reply, err := mm.getCrtcInfo(crtcId)
		if err != nil {
			return err
		}
		mm.crtcs[crtcId] = (*CrtcInfo)(reply)
	}

	mm.refreshMonitorsCache()
	return nil
}

func (mm *xMonitorManager) getCrtcs() map[randr.Crtc]*CrtcInfo {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	result := make(map[randr.Crtc]*CrtcInfo, len(mm.crtcs))
	for crtc, info := range mm.crtcs {
		infoCopy := &CrtcInfo{}
		*infoCopy = *info
		result[crtc] = infoCopy
	}
	return result
}

func (mm *xMonitorManager) getMonitor(id uint32) *MonitorInfo {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	for _, monitor := range mm.monitorsCache {
		if monitor.ID == id {
			monitorCp := *monitor
			return &monitorCp
		}
	}
	return nil
}

func (mm *xMonitorManager) getMonitors() []*MonitorInfo {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	monitors := make([]*MonitorInfo, len(mm.monitorsCache))
	for i, monitorInfo := range mm.monitorsCache {
		monitorCp := *monitorInfo
		monitors[i] = &monitorCp
	}
	return monitors
}

func (mm *xMonitorManager) doDiff() {
	logger.Debug("mm.doDiff")
	// NOTE: 不要加锁
	oldMonitors := toMonitorInfoMap(mm.monitorsCache)
	mm.refreshMonitorsCache()
	newMonitors := mm.monitorsCache
	for _, monitor := range newMonitors {
		oldMonitor, ok := oldMonitors[monitor.ID]
		if ok {
			if !monitor.equal(oldMonitor) {
				if mm.monitorChangedCbEnabled {
					if mm.hooks != nil {
						logger.Debug("call manager handleMonitorChanged", monitor.ID)
						// NOTE: mm.mu 已经上锁了
						mm.mu.Unlock()
						mm.hooks.handleMonitorChanged(monitor)
						mm.mu.Lock()
					}
				} else {
					logger.Debug("monitorChangedCb disabled")
				}
				if monitor.outputId() == mm.primary {
					// NOTE: mm.mu 已经上锁了
					mm.mu.Unlock()
					mm.invokePrimaryRectChangedCb(mm.primary)
					mm.mu.Lock()
				}
			}
		} else {
			logger.Warning("can not handle new monitor")
		}
	}
}

func (mm *xMonitorManager) wait(crtcCfgs map[randr.Crtc]crtcConfig, disabledOutputs map[randr.Output]bool, monitorsId monitorsId) {
	now := time.Now()
	defer func() {
		logger.Debug("wait cost", time.Since(now))
	}()
	const (
		timeout  = 5 * time.Second
		interval = 500 * time.Millisecond
		count    = int(timeout / interval)
	)
	for i := 0; i < count; i++ {
		if mm.compareAll(crtcCfgs, disabledOutputs) {
			logger.Debug("mm wait success")
			return
		}

		if mm.hooks != nil && mm.hooks.getMonitorsId() != monitorsId {
			logger.Debug("monitorsId changed, wait return")
			return
		}

		time.Sleep(interval)
	}
	logger.Warning("mm wait time out")
}

func (mm *xMonitorManager) compareAll(crtcCfgs map[randr.Crtc]crtcConfig, disabledOutputs map[randr.Output]bool) bool {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	//logger.Debug("crtcCfgs:", spew.Sdump(crtcCfgs))
	//logger.Debug("mm.crtcs:", spew.Sdump(mm.crtcs))
	//logger.Debug("mm.outputs:", spew.Sdump(mm.outputs))

	for crtc, crtcCfg := range crtcCfgs {
		crtcInfo := mm.crtcs[crtc]
		if len(crtcCfg.outputs) > 0 {
			// 启用 crtc 的情况
			if !(crtcCfg.x == crtcInfo.X &&
				crtcCfg.y == crtcInfo.Y &&
				crtcCfg.mode == crtcInfo.Mode &&
				crtcCfg.rotation == crtcInfo.Rotation &&
				outputSliceEqual(crtcCfg.outputs, crtcInfo.Outputs)) {
				if logger.GetLogLevel() == log.LevelDebug {
					logger.Debugf("[compareAll] crtc %d not match, crtcCfg(expect): %v, crtcInfo(actual): %v",
						crtcCfg.crtc, spew.Sdump(crtcCfg), spew.Sdump(crtcInfo))
				}
				return false
			}
		} else {
			// 禁用 crtc 的情况, len(crtcCfg.outputs) == 0 成立。
			if !(len(crtcInfo.Outputs) == 0 && crtcInfo.Mode == 0 && crtcInfo.Width == 0 && crtcInfo.Height == 0) {
				if logger.GetLogLevel() == log.LevelDebug {
					logger.Debugf("[compareAll] crtc %d not disabled, crtcCfg(expect): %v, crtcInfo(actual): %v",
						crtcCfg.crtc, spew.Sdump(crtcCfg), spew.Sdump(crtcInfo))
				}
				return false
			}
		}

		if len(crtcCfg.outputs) > 0 {
			outputId := crtcCfg.outputs[0]
			outputInfo := mm.outputs[outputId]
			if crtcCfg.crtc != outputInfo.Crtc {
				logger.Debugf("[compareAll] output %v crtc not match, output crtc expect: %v, actual: %v",
					outputId, crtcCfg.crtc, outputInfo.Crtc)
				return false
			}
		}
	}

	for output := range disabledOutputs {
		outputInfo := mm.outputs[output]
		if outputInfo.Crtc != 0 {
			logger.Debugf("[compareAll] output %v crtc != 0", output)
			return false
		}
	}

	return true
}

func toMonitorInfoMap(monitors []*MonitorInfo) map[uint32]*MonitorInfo {
	result := make(map[uint32]*MonitorInfo, len(monitors))
	for _, monitor := range monitors {
		result[monitor.ID] = monitor
	}
	return result
}

func (mm *xMonitorManager) getStdMonitorName(name string, edid []byte) (string, error) {
	// NOTE：不要加锁
	stdName := mm.stdNamesCache[name]
	if stdName != "" {
		return stdName, nil
	}

	stdName, err := getStdMonitorName(edid)
	if err != nil {
		return "", err
	}
	mm.stdNamesCache[name] = stdName
	return stdName, nil
}

func (mm *xMonitorManager) refreshMonitorsCache() {
	// NOTE: 不要加锁
	monitors := make([]*MonitorInfo, 0, len(mm.outputs))
	for outputId, outputInfo := range mm.outputs {
		monitor := &MonitorInfo{
			crtc:      outputInfo.Crtc,
			ID:        uint32(outputId),
			Name:      outputInfo.Name,
			Connected: outputInfo.Connection == randr.ConnectionConnected,
			Modes:     toModeInfos(mm.modes, outputInfo.Modes),
			MmWidth:   outputInfo.MmWidth,
			MmHeight:  outputInfo.MmHeight,
		}
		monitor.PreferredMode = getPreferredMode(monitor.Modes, uint32(outputInfo.PreferredMode()))
		var err error
		monitor.EDID, err = mm.getOutputEdid(outputId)
		if err != nil {
			logger.Warningf("get output %d edid failed: %v", outputId, err)
		}

		stdName := ""
		if monitor.Connected {
			stdName, err = mm.getStdMonitorName(monitor.Name, monitor.EDID)
			if err != nil {
				logger.Warningf("get monitor %v std name failed: %v", monitor.Name, err)
			}
		}

		monitor.UUID = getOutputUuid(monitor.Name, stdName, monitor.EDID)
		monitor.UuidV0 = getOutputUuidV0(monitor.Name, monitor.EDID)
		monitor.Manufacturer, monitor.Model = parseEdid(monitor.EDID)

		availFillModes, err := mm.getOutputAvailableFillModes(outputId)
		if err != nil {
			logger.Warningf("get output %d available fill modes failed: %v", outputId, err)
		}
		monitor.AvailableFillModes = availFillModes

		// TODO 获取显示器当前的 fill mode

		if monitor.crtc != 0 {
			crtcInfo := mm.crtcs[monitor.crtc]
			if crtcInfo != nil {
				monitor.X = crtcInfo.X
				monitor.Y = crtcInfo.Y
				monitor.Rotation = crtcInfo.Rotation
				monitor.Width, monitor.Height = crtcInfo.Width, crtcInfo.Height
				swapWidthHeightWithRotation(crtcInfo.Rotation, &monitor.Width, &monitor.Height)
				monitor.Rotations = crtcInfo.Rotations
				monitor.CurrentMode = findModeInfo(mm.modes, crtcInfo.Mode)
			}
		}

		if monitor.Connected && monitor.Width != 0 && monitor.Height != 0 {
			monitor.VirtualConnected = true
			monitor.Enabled = true
		} else {
			monitor.VirtualConnected = false
			monitor.Enabled = false
		}

		monitors = append(monitors, monitor)
	}

	mm.monitorsCache = monitors
}

func (mm *xMonitorManager) getFreeCrtcMap() map[randr.Crtc]bool {
	result := make(map[randr.Crtc]bool)
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for crtc, crtcInfo := range mm.crtcs {
		if len(crtcInfo.Outputs) == 0 {
			result[crtc] = true
		}
	}
	return result
}

func (mm *xMonitorManager) findFreeCrtc(output randr.Output, freeCrtcs map[randr.Crtc]bool) randr.Crtc {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for crtc, crtcInfo := range mm.crtcs {
		isFree := freeCrtcs[crtc]
		if isFree && outputSliceContains(crtcInfo.PossibleOutputs, output) {
			freeCrtcs[crtc] = false
			return crtc
		}
	}
	return 0
}

type applyOptions map[string]interface{}

const (
	optionDisableCrtc = "disableCrtc"
	optionOnlyOne     = "onlyOne"
)

type crtcConfig struct {
	crtc    randr.Crtc
	outputs []randr.Output

	x        int16
	y        int16
	rotation uint16
	mode     randr.Mode
}

func findOutputInCrtcCfgs(crtcCfgs map[randr.Crtc]crtcConfig, crtc randr.Crtc) randr.Output {
	for _, crtcCfg := range crtcCfgs {
		if crtcCfg.crtc == crtc {
			if len(crtcCfg.outputs) > 0 {
				return crtcCfg.outputs[0]
			}
		}
	}
	return 0
}

func (mm *xMonitorManager) apply(monitorsId monitorsId, monitorMap map[uint32]*Monitor, prevScreenSize screenSize,
	options applyOptions, fillModes map[string]string, primaryMonitorID uint32, displayMode byte) error {

	logger.Debug("call apply", monitorsId)
	optDisableCrtc, _ := options[optionDisableCrtc].(bool)

	disabledOutputs := make(map[randr.Output]bool)
	freeCrtcs := mm.getFreeCrtcMap()

	// 继续找更多的 free crtc
	for _, monitor := range monitorMap {
		monitor.dumpInfoForDebug()
		monitorInfo := mm.getMonitor(monitor.ID)
		if monitorInfo == nil {
			logger.Warningf("[apply] failed to get monitor %d", monitor.ID)
			continue
		}

		if !monitor.Enabled {
			// 禁用显示器
			disabledOutputs[randr.Output(monitor.ID)] = true
			crtc := monitorInfo.crtc
			if crtc != 0 {
				freeCrtcs[crtc] = true
			}
		}
	}

	// 根据 monitor 的配置，准备 crtc 配置放到 crtcCfgs 中。
	crtcCfgs := make(map[randr.Crtc]crtcConfig)
	for output, monitor := range monitorMap {
		monitorInfo := mm.getMonitor(monitor.ID)
		if monitorInfo == nil {
			logger.Warningf("[apply] failed to get monitor %d", monitor.ID)
			continue
		}

		crtc := monitorInfo.crtc
		if monitor.Enabled {
			// 启用显示器
			if crtc == 0 {
				crtc = mm.findFreeCrtc(randr.Output(output), freeCrtcs)
				if crtc == 0 {
					return errors.New("failed to find free crtc")
				}
			}
			crtcCfgs[crtc] = crtcConfig{
				crtc:     crtc,
				x:        monitor.X,
				y:        monitor.Y,
				mode:     randr.Mode(monitor.CurrentMode.Id),
				rotation: monitor.Rotation | monitor.Reflect,
				outputs:  []randr.Output{randr.Output(output)},
			}
		}
	}

	for crtc, isFree := range freeCrtcs {
		if isFree {
			// 禁用此 crtc，把它的 outputs 设置为空。
			crtcCfgs[crtc] = crtcConfig{
				crtc:     crtc,
				rotation: randr.RotationRotate0,
			}
		}
	}
	logger.Debug("freeCrtcs:", freeCrtcs)
	logger.Debug("disableOutputs", disabledOutputs)

	if logger.GetLogLevel() == log.LevelDebug {
		logger.Debug("crtcCfgs:", spew.Sdump(crtcCfgs))
	}

	// 未来的，apply 之后的屏幕所需尺寸
	screenSize := getScreenSize(monitorMap)
	logger.Debugf("screen size after apply: %+v", screenSize)

	monitors := getConnectedMonitors(monitorMap)

	x.GrabServer(mm.xConn)
	logger.Debug("grab server")
	ungrabServerDone := false
	mm.monitorChangedCbEnabled = false

	ungrabServer := func() {
		if !ungrabServerDone {
			logger.Debug("ungrab server")
			err := x.UngrabServerChecked(mm.xConn).Check(mm.xConn)
			if err != nil {
				logger.Warning(err)
			}
			ungrabServerDone = true
		}
	}

	defer func() {
		ungrabServer()
		mm.monitorChangedCbEnabled = true
		logger.Debug("apply return", monitorsId)
	}()

	var shouldDelayDisableCrtc = false
	for crtc, crtcInfo := range mm.getCrtcs() {
		rect := crtcInfo.getRect()
		logger.Debugf("crtc %v, crtcInfo: %+v", crtc, crtcInfo)

		// 是否考虑临时禁用 crtc
		shouldDisable := false

		if optDisableCrtc {
			// 可能是切换了显示模式
			// NOTE: 如果接入了双屏，断开一个屏幕，让另外的屏幕都暂时禁用，来避免桌面壁纸的闪烁问题（突然黑一下，然后很快恢复），
			// 这么做是为了兼顾修复 pms bug 83875 和 94116。
			// 但是对于 bug 94116，依然保留问题：先断开再连接显示器，桌面壁纸依然有闪烁问题。
			logger.Debugf("should disable crtc %v because of optDisableCrtc is true", crtc)
			shouldDisable = true
		} else if int(rect.X)+int(rect.Width) > int(screenSize.width) ||
			int(rect.Y)+int(rect.Height) > int(screenSize.height) {
			// 当前 crtc 的尺寸超过了未来的屏幕尺寸，必须禁用
			logger.Debugf("should disable crtc %v because of the size of crtc exceeds the size of future screen", crtc)
			shouldDisable = true
		} else {
			output := findOutputInCrtcCfgs(crtcCfgs, crtc)
			if output != 0 {
				monitor := monitors.GetById(uint32(output))
				// 根据 crtc 找到对应的 monitor
				if monitor != nil && monitor.Enabled {
					if rect.X != monitor.X || rect.Y != monitor.Y ||
						rect.Width != monitor.Width || rect.Height != monitor.Height ||
						crtcInfo.Rotation != monitor.Rotation|monitor.Reflect {
						// crtc 的参数将发生改变, 这里的 monitor 包含了 crtc 未来的状态。
						logger.Debugf("should disable crtc %v because of the parameters of crtc changed", crtc)
						shouldDisable = true
					}
				}
			}
		}

		if shouldDisable && len(crtcInfo.Outputs) > 0 {
			logger.Debugf("disable crtc %v, it's outputs: %v", crtc, crtcInfo.Outputs)
			shouldDelayDisableCrtc = true
			err := mm.disableCrtc(crtc)
			if err != nil {
				return err
			}
		}
	}

	// disableCrtc后等待1秒，防止多个屏幕反复disabled导致屏幕提示 频率超出范围 黑屏现象
	// NOTE：此处延时是为了修复BUG 107874  104595 107865
	if shouldDelayDisableCrtc {
		time.Sleep(1 * time.Second)
	}

	err := mm.setScreenSize(screenSize)
	if err != nil {
		return err
	}

	// 为了规避扩展模式下，A B屏相同低分辨率（X 分辨率）的平铺方式不一致，当切换A B屏为复制模式后
	// 此时默认非X分辨率，此时切换到X分辨率，此时需要做规避，以此时主屏的平铺方式去设置所有屏幕
	var primaryScreenFillMode = fillModeDefault
	if monitorMap[primaryMonitorID] != nil {
		primaryScreenFillMode = fillModes[monitorMap[primaryMonitorID].generateFillModeKey()]
	}

	for id, monitor := range monitorMap {
		var monitorFillMode = fillModeDefault
		if displayMode == DisplayModeMirror {
			monitorFillMode = primaryScreenFillMode
		} else {
			monitorFillMode = fillModes[monitor.generateFillModeKey()]
		}

		err = mm.setMonitorFillMode(monitor.m.monitorMap[id], monitorFillMode)
		if err != nil {
			logger.Warning("set monitor fill mode failed:", monitor, err)
		}
	}

	for _, crtcCfg := range crtcCfgs {
		err := mm.setCrtcConfig(crtcCfg)
		if err != nil {
			logger.Warning("set crtcConfig failed:", crtcCfg, err)
		}
	}

	ungrabServer()

	// 等待所有事件结束
	mm.wait(crtcCfgs, disabledOutputs, monitorsId)

	// 更新一遍所有显示器
	mm.monitorChangedCbEnabled = true
	logger.Debug("update all monitors")
	for _, monitor := range monitorMap {
		monitorInfo := mm.getMonitor(monitor.ID)
		if monitorInfo != nil {
			if mm.hooks != nil {
				mm.hooks.handleMonitorChanged(monitorInfo)
			}
		}
	}
	logger.Debug("after update all monitors")

	// NOTE: 为配合文件管理器修一个 bug：
	// 双屏左右摆放，两屏幕有相同最大分辨率，设置左屏为主屏，自定义模式下两屏合并、拆分循环切换，此时如果不发送 PrimaryRect 属性
	// 改变信号，将在从合并切换到拆分时，右屏的桌面壁纸没有绘制，是全黑的。可能是所有显示器的分辨率都没有改变，桌面 dde-desktop
	// 程序收不到相关信号。
	// 此时屏幕尺寸被改变是很好的特征，发送一个 PrimaryRect 属性改变通知桌面 dde-desktop 程序让它重新绘制桌面壁纸，以消除 bug。
	// TODO: 这不是一个很好的方案，后续可与桌面程序方面沟通改善方案。
	if prevScreenSize.width != screenSize.width || prevScreenSize.height != screenSize.height {
		// screen size changed
		// NOTE: 不能直接用 prevScreenSize != screenSize 进行比较，因为 screenSize 类型不止 width 和 height 字段。
		logger.Debug("[apply] screen size changed, force emit prop changed for PrimaryRect")
		// TODO
		//m.PropsMu.RLock()
		//rect := m.PrimaryRect
		//m.PropsMu.RUnlock()
		//err := m.emitPropChangedPrimaryRect(rect)
		//if err != nil {
		//	logger.Warning(err)
		//}
	}

	return nil
}

func (mm *xMonitorManager) setMonitorFillMode(monitor *Monitor, fillMode string) error {
	if !monitor.Enabled {
		return nil
	}
	if len(monitor.AvailableFillModes) == 0 {
		return nil
	}
	if fillMode == "" {
		fillMode = fillModeDefault
	}

	err := mm.setOutputScalingMode(randr.Output(monitor.ID), fillMode)
	if err != nil {
		return err
	}
	// TODO 后续可以根据 output 属性改变来处理
	monitor.setPropCurrentFillMode(fillMode)
	return nil
}

func getConnectedMonitors(monitorMap map[uint32]*Monitor) Monitors {
	var monitors Monitors
	for _, monitor := range monitorMap {
		monitor.PropsMu.RLock()
		connected := monitor.realConnected
		monitor.PropsMu.RUnlock()
		if connected {
			monitors = append(monitors, monitor)
		}
	}
	return monitors
}

// getScreenSize 计算出需要的屏幕尺寸
func getScreenSize(monitorMap map[uint32]*Monitor) screenSize {
	width, height := getScreenWidthHeight(monitorMap)
	mmWidth := uint32(float64(width) / 3.792)
	mmHeight := uint32(float64(height) / 3.792)
	return screenSize{
		width:    width,
		height:   height,
		mmWidth:  mmWidth,
		mmHeight: mmHeight,
	}
}

// getScreenWidthHeight 根据 monitorMap 中显示器的设置，计算出需要的屏幕尺寸。
func getScreenWidthHeight(monitorMap map[uint32]*Monitor) (sw, sh uint16) {
	var w, h int
	for _, monitor := range monitorMap {
		if !monitor.realConnected || !monitor.Enabled {
			continue
		}

		width := monitor.CurrentMode.Width
		height := monitor.CurrentMode.Height

		swapWidthHeightWithRotation(monitor.Rotation, &width, &height)

		w1 := int(monitor.X) + int(width)
		h1 := int(monitor.Y) + int(height)

		if w < w1 {
			w = w1
		}
		if h < h1 {
			h = h1
		}
	}
	if w > math.MaxUint16 {
		w = math.MaxUint16
	}
	if h > math.MaxUint16 {
		h = math.MaxUint16
	}
	sw = uint16(w)
	sh = uint16(h)
	return
}

func (mm *xMonitorManager) setScreenSize(ss screenSize) error {
	root := mm.xConn.GetDefaultScreen().Root
	err := randr.SetScreenSizeChecked(mm.xConn, root, ss.width, ss.height, ss.mmWidth,
		ss.mmHeight).Check(mm.xConn)
	logger.Debugf("set screen size %dx%d, mm: %dx%d",
		ss.width, ss.height, ss.mmWidth, ss.mmHeight)
	return err
}

func (mm *xMonitorManager) disableCrtc(crtc randr.Crtc) error {
	return mm.setCrtcConfig(crtcConfig{
		crtc:     crtc,
		rotation: randr.RotationRotate0,
	})
}

func (mm *xMonitorManager) setCrtcConfig(cfg crtcConfig) error {
	mm.mu.Lock()
	cfgTs := mm.cfgTs
	mm.mu.Unlock()

	logger.Debugf("setCrtcConfig crtc: %v, cfgTs: %v, x: %v, y: %v,"+
		" mode: %v, rotation|reflect: %v, outputs: %v",
		cfg.crtc, cfgTs, cfg.x, cfg.y, cfg.mode, cfg.rotation, cfg.outputs)
	setCfg, err := randr.SetCrtcConfig(mm.xConn, cfg.crtc, 0, cfgTs,
		cfg.x, cfg.y, cfg.mode, cfg.rotation,
		cfg.outputs).Reply(mm.xConn)
	if err != nil {
		return err
	}
	if setCfg.Status != randr.SetConfigSuccess {
		err = fmt.Errorf("failed to configure crtc %v: %v",
			cfg.crtc, getRandrStatusStr(setCfg.Status))
		return err
	}
	return nil
}

func (mm *xMonitorManager) getOutputAvailableFillModes(output randr.Output) ([]string, error) {
	// 判断是否有该属性
	lsPropsReply, err := randr.ListOutputProperties(mm.xConn, output).Reply(mm.xConn)
	if err != nil {
		return nil, err
	}
	atomScalingMode, err := mm.xConn.GetAtom("scaling mode")
	if err != nil {
		return nil, err
	}
	hasProp := false
	for _, atom := range lsPropsReply.Atoms {
		if atom == atomScalingMode {
			hasProp = true
			break
		}
	}
	if !hasProp {
		return nil, nil
	}
	// 获取属性可能的值
	outputProp, _ := randr.QueryOutputProperty(mm.xConn, output, atomScalingMode).Reply(mm.xConn)
	var result []string
	for _, prop := range outputProp.ValidValues {
		fillMode, _ := mm.xConn.GetAtomName(x.Atom(prop))
		result = append(result, fillMode)
	}
	return result, nil
}

func (mm *xMonitorManager) setOutputScalingMode(output randr.Output, fillMode string) error {
	if fillMode != fillModeFull &&
		fillMode != fillModeCenter &&
		fillMode != fillModeDefault &&
		fillMode != fillModeFullaspect {
		logger.Warning("invalid fill mode:", fillMode)
		return fmt.Errorf("invalid fill mode %q", fillMode)
	}

	xConn := mm.xConn
	fillModeU32, _ := xConn.GetAtom(fillMode)
	name, _ := xConn.GetAtom("scaling mode")

	// TODO 改成不用 get ？
	outputPropReply, err := randr.GetOutputProperty(xConn, output, name, 0, 0,
		100, false, false).Reply(xConn)
	if err != nil {
		logger.Warning("call GetOutputProperty reply err:", err)
		return err
	}

	w := x.NewWriter()
	w.Write4b(uint32(fillModeU32))
	fillModeByte := w.Bytes()
	err = randr.ChangeOutputPropertyChecked(xConn, output, name,
		outputPropReply.Type, outputPropReply.Format, 0, fillModeByte).Check(xConn)
	if err != nil {
		logger.Warning("err:", err)
		return err
	}

	return nil
}

func (mm *xMonitorManager) setMonitorPrimary(monitorId uint32) error {
	logger.Debug("mm.setMonitorPrimary", monitorId)
	mm.mu.Lock()
	mm.primary = randr.Output(monitorId)
	mm.mu.Unlock()
	err := mm.setOutputPrimary(randr.Output(monitorId))
	if err != nil {
		return err
	}

	// 设置之后处理一次更新回调
	pOut, err := mm.GetOutputPrimary()
	if err != nil {
		return err
	}
	mm.invokePrimaryRectChangedCb(pOut)
	return nil
}

func (mm *xMonitorManager) invokePrimaryRectChangedCb(pOut randr.Output) {
	// NOTE: 需要处于不加锁 mm.mu 的环境
	pmi := mm.getPrimaryMonitorInfo(pOut)
	if mm.hooks != nil {
		mm.hooks.handlePrimaryRectChanged(pmi)
	}
}

func (mm *xMonitorManager) setOutputPrimary(output randr.Output) error {
	logger.Debug("set output primary", output)
	root := mm.xConn.GetDefaultScreen().Root
	return randr.SetOutputPrimaryChecked(mm.xConn, root, output).Check(mm.xConn)
}

func (mm *xMonitorManager) GetOutputPrimary() (randr.Output, error) {
	root := mm.xConn.GetDefaultScreen().Root
	reply, err := randr.GetOutputPrimary(mm.xConn, root).Reply(mm.xConn)
	if err != nil {
		return 0, err
	}
	return reply.Output, nil
}

func (mm *xMonitorManager) getPrimaryMonitorInfo(pOutput randr.Output) (pmi primaryMonitorInfo) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if pOutput != 0 {
		for output, outputInfo := range mm.outputs {
			if pOutput != output {
				continue
			}

			pmi.Name = outputInfo.Name

			if outputInfo.Crtc == 0 {
				logger.Warning("new primary output crtc is 0")
			} else {
				crtcInfo := mm.crtcs[outputInfo.Crtc]
				if crtcInfo == nil {
					logger.Warning("crtcInfo is nil")
				} else {
					pmi.Rect = crtcInfo.getRect()
				}
			}
			break
		}
	}
	return
}

func (mm *xMonitorManager) getCrtcInfo(crtc randr.Crtc) (*randr.GetCrtcInfoReply, error) {
	crtcInfo, err := randr.GetCrtcInfo(mm.xConn, crtc, mm.cfgTs).Reply(mm.xConn)
	if err != nil {
		return nil, err
	}
	if crtcInfo.Status != randr.StatusSuccess {
		return nil, fmt.Errorf("status is not success, is %v", crtcInfo.Status)
	}
	return crtcInfo, err
}

func (mm *xMonitorManager) getOutputInfo(outputId randr.Output) (*randr.GetOutputInfoReply, error) {
	outputInfo, err := randr.GetOutputInfo(mm.xConn, outputId, mm.cfgTs).Reply(mm.xConn)
	if err != nil {
		return nil, err
	}
	if outputInfo.Status != randr.StatusSuccess {
		return nil, fmt.Errorf("status is not success, is %v", outputInfo.Status)
	}
	return outputInfo, err
}

func (mm *xMonitorManager) getOutputEdid(output randr.Output) ([]byte, error) {
	atomEDID, err := mm.xConn.GetAtom("EDID")
	if err != nil {
		return nil, err
	}

	reply, err := randr.GetOutputProperty(mm.xConn, output,
		atomEDID, x.AtomInteger,
		0, 32, false, false).Reply(mm.xConn)
	if err != nil {
		return nil, err
	}
	return reply.Value, nil
}

func (mm *xMonitorManager) getScreenResources(xConn *x.Conn) (*randr.GetScreenResourcesReply, error) {
	root := xConn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResources(xConn, root).Reply(xConn)
	return resources, err
}

func (mm *xMonitorManager) getScreenResourcesCurrent() (*randr.GetScreenResourcesCurrentReply, error) {
	root := mm.xConn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResourcesCurrent(mm.xConn, root).Reply(mm.xConn)
	return resources, err
}

func (mm *xMonitorManager) handleCrtcChanged(e *randr.CrtcChangeNotifyEvent) {
	// NOTE: 不要加锁
	reply, err := mm.getCrtcInfo(e.Crtc)
	if err != nil {
		logger.Warningf("get crtc %v info failed: %v", e.Crtc, err)
		return
	}
	// 这些字段使用 event 中提供的
	reply.X = e.X
	reply.Y = e.Y
	reply.Width = e.Width
	reply.Height = e.Height
	reply.Rotation = e.Rotation
	reply.Mode = e.Mode

	mm.crtcs[e.Crtc] = (*CrtcInfo)(reply)
}

func (mm *xMonitorManager) handleOutputChanged(e *randr.OutputChangeNotifyEvent) {
	// NOTE: 不要加锁
	outputInfo := mm.outputs[e.Output]
	if outputInfo == nil {
		reply, err := mm.getOutputInfo(e.Output)
		if err != nil {
			logger.Warningf("get output %v info failed: %v", e.Output, err)
			return
		}
		mm.outputs[e.Output] = (*OutputInfo)(reply)
		return
	}

	// e.Mode 和 e.Rotation 没有被使用到, 因为在 refreshMonitorsCache 中没有用到 outputInfo 的 Mode 和 Rotation。
	// 只用 crtcInfo 的 Mode 和 Rotation 就足够了。
	outputInfo.Crtc = e.Crtc
	outputInfo.Connection = e.Connection
	outputInfo.SubPixelOrder = e.SubPixelOrder
}

func (mm *xMonitorManager) HandleEvent(ev interface{}) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	logger.Debugf("mm.HandleEvent %#v", ev)
	defer logger.Debugf("mm.HandleEvent return %#v", ev)

	switch e := ev.(type) {
	case *randr.CrtcChangeNotifyEvent:
		mm.handleCrtcChanged(e)
	case *randr.OutputChangeNotifyEvent:
		mm.handleOutputChanged(e)
		// NOTE: ScreenChangeNotifyEvent 事件比较特殊，不在这里处理。
	default:
		logger.Debug("invalid event", ev)
		return
	}

	mm.doDiff()
}

func (mm *xMonitorManager) HandleScreenChanged(e *randr.ScreenChangeNotifyEvent) (cfgTsChanged bool) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	logger.Debugf("mm.HandleScreenChanged %#v", e)
	defer logger.Debugf("mm.HandleScreenChanged return %#v", e)
	cfgTsChanged = mm.handleScreenChanged(e)
	mm.doDiff()
	return
}

func (mm *xMonitorManager) handleScreenChanged(e *randr.ScreenChangeNotifyEvent) (cfgTsChanged bool) {
	// NOTE: 不要加锁
	if mm.cfgTs == e.ConfigTimestamp {
		return false
	}
	cfgTsChanged = true
	resources, err := mm.getScreenResourcesCurrent()
	if err != nil {
		logger.Warning("get current screen resources failed:", err)
		return
	}
	mm.cfgTs = resources.ConfigTimestamp
	mm.modes = resources.Modes

	mm.outputs = make(map[randr.Output]*OutputInfo)
	for _, outputId := range resources.Outputs {
		reply, err := mm.getOutputInfo(outputId)
		if err != nil {
			logger.Warningf("get output %v info failed: %v", outputId, err)
			continue
		}
		mm.outputs[outputId] = (*OutputInfo)(reply)
	}

	mm.crtcs = make(map[randr.Crtc]*CrtcInfo)
	for _, crtcId := range resources.Crtcs {
		reply, err := mm.getCrtcInfo(crtcId)
		if err != nil {
			logger.Warningf("get crtc %v info failed: %v", crtcId, err)
			continue
		}
		mm.crtcs[crtcId] = (*CrtcInfo)(reply)
	}
	return
}

func (mm *xMonitorManager) showCursor(show bool) error {
	rootWin := mm.xConn.GetDefaultScreen().Root
	var cookie x.VoidCookie
	if show {
		logger.Debug("xfixes show cursor")
		cookie = xfixes.ShowCursorChecked(mm.xConn, rootWin)
	} else {
		logger.Debug("xfixes hide cursor")
		cookie = xfixes.HideCursorChecked(mm.xConn, rootWin)
	}
	err := cookie.Check(mm.xConn)
	return err
}

func distory() {
	if _dpy != nil {
		_dpy.dbusDaemon.RemoveAllHandlers()
		_dpy.service.StopExport(_dpy)
		_dpy.sysSigLoop.Stop()
		_dpy.sessionSigLoop.Stop()
	}
}
