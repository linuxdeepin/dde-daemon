/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package screenedge

// Enable desktop edge zone detected
//
// 是否启用桌面边缘热区功能
func (m *Manager) EnableZoneDetected(enable bool) {
	m.unregisterEdgeAreas()
	if enable {
		m.registerEdgeAreas()
	}
}

// Set left-top edge action
func (m *Manager) SetTopLeft(value string) {
	m.settings.SetEdgeAction(TopLeft, value)
}

// Get left-top edge action
func (m *Manager) TopLeftAction() string {
	return m.settings.GetEdgeAction(TopLeft)
}

// Set left-bottom edge action
func (m *Manager) SetBottomLeft(value string) {
	m.settings.SetEdgeAction(BottomLeft, value)
}

// Get left-bottom edge action
func (m *Manager) BottomLeftAction() string {
	return m.settings.GetEdgeAction(BottomLeft)
}

// Set right-top edge action
func (m *Manager) SetTopRight(value string) {
	m.settings.SetEdgeAction(TopRight, value)
}

// Get right-top edge action
func (m *Manager) TopRightAction() string {
	return m.settings.GetEdgeAction(TopRight)
}

// Set right-bottom edge action
func (m *Manager) SetBottomRight(value string) {
	m.settings.SetEdgeAction(BottomRight, value)
}

// Get right-bottom edge action
func (m *Manager) BottomRightAction() string {
	return m.settings.GetEdgeAction(BottomRight)
}
