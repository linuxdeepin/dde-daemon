// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/dxinput"
	"github.com/linuxdeepin/dde-api/dxinput/common"
	dxutils "github.com/linuxdeepin/dde-api/dxinput/utils"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

const (
	BusTypeUnknown uint8 = iota
	BusTypeUSB
)

const (
	rotationReflectAll = randr.RotationReflectX | randr.RotationReflectY
)

type Touchscreen struct {
	Id         int32
	Name       string
	DeviceNode string
	Serial     string
	UUID       string
	outputName string
	busType    uint8
	width      float64
	height     float64
	path       dbus.ObjectPath
}

type dxTouchscreens []*Touchscreen

var (
	devInfos           common.DeviceInfos
	touchscreenInfos   dxTouchscreens
	touchscreenInfosMu sync.Mutex
)

func getDeviceInfos(force bool) common.DeviceInfos {
	if force || len(devInfos) == 0 {
		devInfos = dxutils.ListDevice()
	}

	return devInfos
}

func getXTouchscreenInfo(t *Touchscreen) {
	for _, v := range getDeviceInfos(false) {
		if v.Type != common.DevTypeTouchscreen {
			continue
		}

		tmp, _ := dxinput.NewTouchscreenFromDevInfo(v)
		data, num := dxutils.GetProperty(tmp.Id, "Device Node")
		if len(data) == 0 {
			logger.Warningf("could not get DeviceNode for %s (%d)", tmp.Name, tmp.Id)
			continue
		}

		deviceNode := string(data[:num])
		logger.Warningf("deviceNode: %s", deviceNode)

		logger.Warningf("devNode: %s, deviceNode: %s", t.DeviceNode, deviceNode)
		if t.DeviceNode != deviceNode {
			continue
		}

		t.Id = tmp.Id
	}
}

type TransformationMatrix [9]float32

func (m *TransformationMatrix) set(row int, col int, v float32) {
	m[row*3+col] = v
}

func (m *TransformationMatrix) setUnity() {
	m.set(0, 0, 1)
	m.set(1, 1, 1)
	m.set(2, 2, 1)
}

func (m *TransformationMatrix) s4(x02 float32, x12 float32, d1 float32, d2 float32, mainDiag bool) {
	m.set(0, 2, x02)
	m.set(1, 2, x12)

	if mainDiag {
		m.set(0, 0, d1)
		m.set(1, 1, d2)
	} else {
		m.set(0, 0, 0)
		m.set(1, 1, 0)
		m.set(0, 1, d1)
		m.set(1, 0, d2)
	}
}

func genTransformationMatrix(offsetX int16, offsetY int16,
	screenWidth uint16, screenHeight uint16,
	rotation uint16) TransformationMatrix {

	// 必须新的 X 链接才能获取最新的 WidthInPixels 和 HeightInPixels
	xConn, err := x.NewConn()
	if err != nil {
		logger.Warning("failed to connect to x server")
		return genTransformationMatrixAux(offsetX, offsetY, screenWidth, screenHeight, screenWidth, screenHeight, rotation)
	}

	// total display size
	width := xConn.GetDefaultScreen().WidthInPixels
	height := xConn.GetDefaultScreen().HeightInPixels
	xConn.Close()

	return genTransformationMatrixAux(offsetX, offsetY, screenWidth, screenHeight, width, height, rotation)
}

func genTransformationMatrixAux(offsetX int16, offsetY int16,
	screenWidth uint16, screenHeight uint16,
	totalDisplayWidth uint16, totalDisplayHeight uint16,
	rotation uint16) TransformationMatrix {

	var matrix TransformationMatrix
	matrix.setUnity()

	x := float32(offsetX) / float32(totalDisplayWidth)
	y := float32(offsetY) / float32(totalDisplayHeight)

	w := float32(screenWidth) / float32(totalDisplayWidth)
	h := float32(screenHeight) / float32(totalDisplayHeight)

	/*
	 * There are 16 cases:
	 * Rotation X Reflection
	 * Rotation: 0 | 90 | 180 | 270
	 * Reflection: None | X | Y | XY
	 *
	 * They are spelled out instead of doing matrix multiplication to avoid
	 * any floating point errors.
	 */
	switch int(rotation) {
	case randr.RotationRotate0:
		fallthrough
	case randr.RotationRotate180 | rotationReflectAll:
		matrix.s4(x, y, w, h, true)

	case randr.RotationReflectX | randr.RotationRotate0:
		fallthrough
	case randr.RotationReflectY | randr.RotationRotate180:
		matrix.s4(x+w, y, -w, h, true)

	case randr.RotationReflectY | randr.RotationRotate0:
		fallthrough
	case randr.RotationReflectX | randr.RotationRotate180:
		matrix.s4(x, y+h, w, -h, true)

	case randr.RotationRotate90:
		fallthrough
	case randr.RotationRotate270 | rotationReflectAll: /* left limited - correct in working zone. */
		matrix.s4(x+w, y, -w, h, false)

	case randr.RotationRotate270:
		fallthrough
	case randr.RotationRotate90 | rotationReflectAll: /* left limited - correct in working zone. */
		matrix.s4(x, y+h, w, -h, false)

	case randr.RotationRotate90 | randr.RotationReflectX: /* left limited - correct in working zone. */
		fallthrough
	case randr.RotationRotate270 | randr.RotationReflectY: /* left limited - correct in working zone. */
		matrix.s4(x, y, w, h, false)

	case randr.RotationRotate90 | randr.RotationReflectY: /* right limited - correct in working zone. */
		fallthrough
	case randr.RotationRotate270 | randr.RotationReflectX: /* right limited - correct in working zone. */
		matrix.s4(x+w, y+h, -w, -h, false)

	case randr.RotationRotate180:
		fallthrough
	case rotationReflectAll | randr.RotationRotate0:
		matrix.s4(x+w, y+h, -w, -h, true)
	}

	return matrix
}
