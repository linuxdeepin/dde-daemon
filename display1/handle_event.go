// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"time"

	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

// 在 X 下，显示器属性改变，断开或者连接显示器。
// 在 wayland 下，仅显示器属性改变。
func (m *Manager) handleMonitorChanged(monitorInfo *MonitorInfo) {
	m.updateMonitor(monitorInfo)
	if monitorInfo.Enabled {
		m.tryToChangeScaleFactor(monitorInfo.Width, monitorInfo.Height)
	}
	if _useWayland {
		return
	}

	// 后续只在 X 下需要
	currentNumMonitors := len(m.getConnectedMonitors())
	m.PropsMu.Lock()
	prevCurrentNumMonitors := m.prevCurrentNumMonitors
	m.prevCurrentNumMonitors = currentNumMonitors

	if prevCurrentNumMonitors != currentNumMonitors {
		m.prevNumMonitors = prevCurrentNumMonitors
		m.prevNumMonitorsUpdatedAt = time.Now()
	}

	m.PropsMu.Unlock()

	logger.Debugf("prevCurrentNumMonitors: %v, currentNumMonitors: %v", prevCurrentNumMonitors, currentNumMonitors)
	var options applyOptions
	if currentNumMonitors < prevCurrentNumMonitors && currentNumMonitors >= 1 {
		// 连接状态的显示器数量减少了，并且现存一个及以上连接状态的显示器。
		logger.Debug("should disable crtc in apply")
		if options == nil {
			options = applyOptions{}
		}
		options[optionDisableCrtc] = true
	}
	m.updateMonitorsId(options)
}

// wayland 下连接显示器
func (m *Manager) handleMonitorAdded(monitorInfo *MonitorInfo) {
	err := m.addMonitor(monitorInfo)
	if err != nil {
		logger.Warning(err)
		return
	}
	m.updatePropMonitors()
	m.updateMonitorsId(nil)
	m.tryToChangeScaleFactor(monitorInfo.Width, monitorInfo.Height)
}

// wayland 下断开显示器
func (m *Manager) handleMonitorRemoved(monitorId uint32) {
	logger.Debug("monitor removed", monitorId)
	monitor := m.removeMonitor(monitorId)
	if monitor == nil {
		logger.Warning("remove monitor failed, invalid id", monitorId)
		return
	}

	m.handleMonitorConnectedChanged(monitor, false)
	m.updatePropMonitors()
	m.updateMonitorsId(nil)
}

func (m *Manager) handleOutputPropertyChanged(ev *randr.OutputPropertyNotifyEvent) {
	logger.Debug("output property changed", ev.Output, ev.Atom)
}

func (m *Manager) handleScreenChanged(ev *randr.ScreenChangeNotifyEvent, cfgTsChanged bool) {
	width, height := ev.Width, ev.Height
	swapWidthHeightWithRotation(uint16(ev.Rotation), &width, &height)
	logger.Debugf("screen changed cfgTs: %v, rotation:%v, screen size: %vx%v", ev.ConfigTimestamp,
		ev.Rotation, width, height)

	m.PropsMu.Lock()
	m.setPropScreenWidth(width)
	m.setPropScreenHeight(height)
	m.PropsMu.Unlock()

	if cfgTsChanged {
		logger.Debug("config timestamp changed")
		if !_hasRandr1d2 {

			// randr 版本低于 1.2
			root := m.xConn.GetDefaultScreen().Root
			screenInfo, err := randr.GetScreenInfo(m.xConn, root).Reply(m.xConn)
			if err == nil {
				monitor := m.updateMonitorFallback(screenInfo)
				m.setPropPrimaryRect(x.Rectangle{
					X:      monitor.X,
					Y:      monitor.Y,
					Width:  monitor.Width,
					Height: monitor.Height,
				})
			} else {
				logger.Warning(err)
			}
		}
	}

	logger.Info("redo map touch screen")
	m.handleTouchscreenChanged()

	if cfgTsChanged {
		m.showTouchscreenDialogs()
	}
}
