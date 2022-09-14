// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/gsprop"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"github.com/linuxdeepin/dde-api/dxinput"
)

const (
	wacomSchema       = "com.deepin.dde.wacom"
	wacomStylusSchema = wacomSchema + ".stylus"
	wacomEraserSchema = wacomSchema + ".eraser"

	wacomKeyLeftHanded        = "left-handed"
	wacomKeyCursorMode        = "cursor-mode"
	wacomKeyForceProportions  = "force-proportions"
	wacomKeyUpAction          = "keyup-action"
	wacomKeyDownAction        = "keydown-action"
	wacomKeySuppress          = "suppress"
	wacomKeyPressureSensitive = "pressure-sensitive"
	wacomKeyRawSample         = "raw-sample"
	wacomKeyThreshold         = "threshold"
	wacomKeyMouseEnterRemap   = "mouse-enter-remap"
)

const (
	btnNumUpKey   = 3
	btnNumDownKey = 2
)

var actionMap = map[string]string{
	"LeftClick":   "button 1",
	"MiddleClick": "button 2",
	"RightClick":  "button 3",
	"PageUp":      "key KP_Page_Up",
	"PageDown":    "key KP_Page_Down",
}

type ActionInfo struct {
	Action string
	Desc   string
}
type ActionInfos []*ActionInfo

type OutputInfo struct {
	Name string
	X    int16
	Y    int16
	W    uint16
	H    uint16
}

func (i *OutputInfo) String() string {
	return fmt.Sprintf("OutputInfo %s, crtc (%d,%d) %dx%d",
		i.Name, i.X, i.Y, i.W, i.H)
}

func (i *OutputInfo) Contains(x, y int16) bool {
	return i.X <= x && x < i.X+int16(i.W) &&
		i.Y <= y && y < i.Y+int16(i.H)
}

func getOutputInfo(conn *x.Conn, output randr.Output, ts x.Timestamp) (*OutputInfo, error) {
	outputInfoReply, err := randr.GetOutputInfo(conn, output, ts).Reply(conn)
	if err != nil {
		logger.Warningf("failed to get output %d info: %v", output, err)
		return nil, err
	}

	if outputInfoReply.Crtc != 0 {
		crtcInfoReply, err := randr.GetCrtcInfo(conn, outputInfoReply.Crtc, ts).Reply(conn)
		if err != nil {
			logger.Warningf("failed to get crtc %d info: %v",
				outputInfoReply.Crtc, err)
			return nil, err
		}
		return &OutputInfo{
			Name: string(outputInfoReply.Name),
			X:    crtcInfoReply.X,
			Y:    crtcInfoReply.Y,
			W:    crtcInfoReply.Width,
			H:    crtcInfoReply.Height,
		}, nil
	}
	return nil, errors.New("BadCrtc")
}

type Wacom struct {
	service    *dbusutil.Service
	PropsMu    sync.RWMutex
	DeviceList string
	Exist      bool
	MapOutput  string

	// dbusutil-gen: ignore-below
	LeftHanded       gsprop.Bool `prop:"access:rw"`
	CursorMode       gsprop.Bool `prop:"access:rw"`
	ForceProportions gsprop.Bool `prop:"access:rw"`
	MouseEnterRemap  gsprop.Bool `prop:"access:rw"` // need remap when mouse enter new screen

	KeyUpAction   gsprop.String `prop:"access:rw"`
	KeyDownAction gsprop.String `prop:"access:rw"`

	Suppress                gsprop.Uint `prop:"access:rw"`
	StylusPressureSensitive gsprop.Uint `prop:"access:rw"`
	EraserPressureSensitive gsprop.Uint `prop:"access:rw"`
	StylusRawSample         gsprop.Uint `prop:"access:rw"`
	EraserRawSample         gsprop.Uint `prop:"access:rw"`
	StylusThreshold         gsprop.Uint `prop:"access:rw"`
	EraserThreshold         gsprop.Uint `prop:"access:rw"`

	ActionInfos ActionInfos // TODO: remove this field

	devInfos      dxWacoms
	setting       *gio.Settings
	stylusSetting *gio.Settings
	eraserSetting *gio.Settings

	pointerX      int16
	pointerY      int16
	outputInfos   []*OutputInfo
	outputInfosMu sync.Mutex
	mapToOutput   *OutputInfo
	setAreaMutex  sync.Mutex
	xConn         *x.Conn
	exit          chan int
}

func newWacom(service *dbusutil.Service) *Wacom {
	var w = new(Wacom)

	w.service = service
	w.setting = gio.NewSettings(wacomSchema)
	w.LeftHanded.Bind(w.setting, wacomKeyLeftHanded)
	w.CursorMode.Bind(w.setting, wacomKeyCursorMode)
	w.ForceProportions.Bind(w.setting, wacomKeyForceProportions)
	w.Suppress.Bind(w.setting, wacomKeySuppress)
	w.MouseEnterRemap.Bind(w.setting, wacomKeyMouseEnterRemap)

	// stylus settings
	w.stylusSetting = gio.NewSettings(wacomStylusSchema)
	w.KeyUpAction.Bind(w.stylusSetting, wacomKeyUpAction)
	w.KeyDownAction.Bind(w.stylusSetting, wacomKeyDownAction)
	w.StylusPressureSensitive.Bind(w.stylusSetting, wacomKeyPressureSensitive)
	w.StylusRawSample.Bind(w.stylusSetting, wacomKeyRawSample)
	w.StylusThreshold.Bind(w.stylusSetting, wacomKeyThreshold)

	// eraser settings
	w.eraserSetting = gio.NewSettings(wacomEraserSchema)
	w.EraserPressureSensitive.Bind(w.eraserSetting, wacomKeyPressureSensitive)
	w.EraserRawSample.Bind(w.eraserSetting, wacomKeyRawSample)
	w.EraserThreshold.Bind(w.eraserSetting, wacomKeyThreshold)

	w.updateDXWacoms()

	err := w.initX()
	if err != nil {
		logger.Warning("initX error:", err)
	}
	w.handleScreenChanged()
	go w.listenXRandrEvents()
	w.exit = make(chan int)
	go w.checkLoop()
	return w
}

func (w *Wacom) init() {
	if !w.Exist {
		return
	}

	w.enableCursorMode()
	w.enableLeftHanded()
	w.setStylusButtonAction(btnNumUpKey, w.KeyUpAction.Get())
	w.setStylusButtonAction(btnNumDownKey, w.KeyDownAction.Get())
	w.setPressureSensitive()
	w.setSuppress()
	w.setRawSample()
	w.setThreshold()
}

func (w *Wacom) initX() error {
	var err error
	w.xConn, err = x.NewConn()
	if err != nil {
		return err
	}

	_, err = randr.QueryVersion(w.xConn, randr.MajorVersion,
		randr.MinorVersion).Reply(w.xConn)
	if err != nil {
		return err
	}

	return nil
}

func (w *Wacom) listenXRandrEvents() {
	conn := w.xConn
	root := conn.GetDefaultScreen().Root
	err := randr.SelectInputChecked(conn, root, randr.NotifyMaskScreenChange).Check(conn)
	if err != nil {
		logger.Warning(err)
		return
	}

	rrExtData := conn.GetExtensionData(randr.Ext())
	eventChan := make(chan x.GenericEvent, 10)
	conn.AddEventChan(eventChan)
	for ev := range eventChan {
		switch ev.GetEventCode() {
		case randr.ScreenChangeNotifyEventCode + rrExtData.FirstEvent:
			event, _ := randr.NewScreenChangeNotifyEvent(ev)
			logger.Debugf("event: %#v", event)
			w.handleScreenChanged()
		}
	}
}

func (w *Wacom) handleScreenChanged() {
	conn := w.xConn
	root := conn.GetDefaultScreen().Root
	resources, err := randr.GetScreenResources(conn, root).Reply(conn)
	if err != nil {
		logger.Warning(err)
		return
	}

	var outputInfos []*OutputInfo
	for _, output := range resources.Outputs {
		outputInfo, err := getOutputInfo(conn, output, resources.ConfigTimestamp)
		if err != nil {
			continue
		}
		outputInfos = append(outputInfos, outputInfo)
	}
	w.outputInfosMu.Lock()
	w.outputInfos = outputInfos
	w.outputInfosMu.Unlock()
}

func (w *Wacom) updatePointerPos() {
	conn := w.xConn
	root := conn.GetDefaultScreen().Root
	reply, err := x.QueryPointer(conn, root).Reply(conn)
	if err != nil {
		logger.Debug(err)
		return
	}
	X := reply.RootX
	y := reply.RootY

	if X != w.pointerX || y != w.pointerY {
		// pointer pos changed
		w.pointerX = X
		w.pointerY = y
		// logger.Debugf("x: %d, y: %d", x, y)
	}
}

func (w *Wacom) checkLoop() {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case _, ok := <-ticker.C:
			if !ok {
				logger.Error("Invalid ticker event")
				return
			}

			if !w.Exist {
				// logger.Debug("tick no wacom device")
				continue
			}
			//logger.Debug("tick")
			w.updateAreaAndMapToOutput()
		case <-w.exit:
			ticker.Stop()
			logger.Debug("checkLoop return")
			return
		}
	}
}

// update Area and MapToOutput settings
func (w *Wacom) updateAreaAndMapToOutput() {
	// check if need remap when mouse enter new screen
	if !w.MouseEnterRemap.Get() {
		return
	}
	w.updatePointerPos()
	inOutput := w.pointerInOutput()
	if inOutput == nil {
		// error state
		logger.Warning("pointerInOutput result is nil")
		return
	}

	if w.mapToOutput == nil || *inOutput != *w.mapToOutput {
		// first run this function or screen area changed
		w.mapToOutput = inOutput
		w.setArea()
	}

	w.PropsMu.Lock()
	if w.setPropMapOutput(inOutput.Name) {
		// map to output changed
		w.setMapToOutput()
	}
	w.PropsMu.Unlock()
}

func (w *Wacom) pointerInOutput() *OutputInfo {
	w.outputInfosMu.Lock()
	defer w.outputInfosMu.Unlock()

	if len(w.outputInfos) == 1 {
		return w.outputInfos[0]
	}

	for _, outputInfo := range w.outputInfos {
		if outputInfo.Contains(w.pointerX, w.pointerY) {
			return outputInfo
		}
	}
	return nil
}

func (w *Wacom) handleDeviceChanged() {
	w.updateDXWacoms()
	w.init()
}

func (w *Wacom) updateDXWacoms() {
	w.devInfos = dxWacoms{}
	for _, info := range getWacomInfos(false) {
		tmp := w.devInfos.get(info.Id)
		if tmp != nil {
			continue
		}
		w.devInfos = append(w.devInfos, info)
	}

	w.PropsMu.Lock()
	var v string
	if len(w.devInfos) == 0 {
		w.setPropExist(false)
	} else {
		w.setPropExist(true)
		v = w.devInfos.string()
	}
	w.setPropDeviceList(v)
	w.PropsMu.Unlock()
}

func (w *Wacom) setStylusButtonAction(btnNum int, action string) {
	value, ok := actionMap[action]
	if !ok {
		logger.Warningf("Invalid button action %q, actionMap: %v", action, actionMap)
		return
	}
	// set button action for stylus
	for _, v := range w.devInfos {
		if v.QueryType() == dxinput.WacomTypeStylus {
			err := v.SetButton(btnNum, value)
			if err != nil {
				logger.Warningf("Set button mapping for '%v - %v' failed: %v",
					v.Id, v.Name, err)
			}
		}
	}
}

func (w *Wacom) enableLeftHanded() {
	var rotate string = "none"
	if w.LeftHanded.Get() {
		rotate = "half"
	}
	// set rotate for stylus and eraser
	// Rotation is a tablet-wide option:
	// rotation of one tool affects all other tools associated with the same tablet.
	for _, v := range w.devInfos {
		devType := v.QueryType()
		if devType == dxinput.WacomTypeStylus || devType == dxinput.WacomTypeEraser {
			err := v.SetRotate(rotate)
			if err != nil {
				logger.Warningf("Set rotate for '%v - %v' failed: %v",
					v.Id, v.Name, err)
			}
		}
	}
}

func (w *Wacom) enableCursorMode() {
	var mode string = "Absolute"
	if w.CursorMode.Get() {
		mode = "Relative"
	}

	// set mode for stylus and eraser
	// NOTE: set mode Relative for pad  will cause X error
	for _, v := range w.devInfos {
		devType := v.QueryType()
		if devType == dxinput.WacomTypeStylus || devType == dxinput.WacomTypeEraser {
			err := v.SetMode(mode)
			if err != nil {
				logger.Warningf("Set mode for '%v - %v' failed: %v",
					v.Id, v.Name, err)
			}
		}
	}
}

func getPressureCurveControlPoints(level int) []int {
	// level 1 ~ 7 Soften ~ Firmer
	const seg = 6.0
	const d = 30.0
	x := (100-2*d)/seg*(float64(level)-1) + d
	y := 100 - x
	x1 := x - d
	y1 := y - d
	x2 := x + d
	y2 := y + d
	return []int{int(x1), int(y1), int(x2), int(y2)}
}

func (w *Wacom) getPressureCurveArray(devType int) ([]int, error) {
	// level is float value
	var level uint32
	switch devType {
	case dxinput.WacomTypeStylus:
		level = w.StylusPressureSensitive.Get()
	case dxinput.WacomTypeEraser:
		level = w.EraserPressureSensitive.Get()
	default:
		return nil, fmt.Errorf("Invalid wacom device type")
	}
	logger.Debug("pressure level:", level)
	if 1 <= level && level <= 7 {
		points := getPressureCurveControlPoints(int(level))
		return points, nil
	} else {
		return nil, fmt.Errorf("Invalid pressure sensitive level %v, range: [1, 7]", level)
	}
}

func (w *Wacom) setPressureSensitiveForType(devType int) {
	for _, v := range w.devInfos {
		if v.QueryType() == devType {
			array, err := w.getPressureCurveArray(devType)
			if err != nil {
				logger.Warning(err)
				continue
			}

			logger.Debug("set curve array:", array)
			err = v.SetPressureCurve(array[0], array[1], array[2], array[3])
			if err != nil {
				logger.Warningf("Set pressure curve for '%v - %v' failed: %v",
					v.Id, v.Name, err)
			}
		}
	}
}

func (w *Wacom) setPressureSensitive() {
	w.setPressureSensitiveForType(dxinput.WacomTypeStylus)
	w.setPressureSensitiveForType(dxinput.WacomTypeEraser)
}

func (w *Wacom) setSuppress() {
	delta := int(w.Suppress.Get())
	for _, dev := range w.devInfos {
		err := dev.SetSuppress(delta)
		if err != nil {
			logger.Debugf("Set suppress for '%v - %v' to %v failed: %v",
				dev.Id, dev.Name, delta, err)
		}
	}
}

func (w *Wacom) setMapToOutput() {
	output := w.MapOutput
	if output == "" {
		output = "desktop"
	}

	logger.Debugf("setMapToOutput %q", output)
	for _, v := range w.devInfos {
		err := v.MapToOutput(output)
		if err != nil {
			logger.Warningf("Map output for '%v - %v' to %v failed: %v",
				v.Id, v.Name, output, err)
		}
	}
}

func (w *Wacom) setArea() {
	w.setAreaMutex.Lock()
	defer w.setAreaMutex.Unlock()

	if w.mapToOutput == nil {
		return
	}

	logger.Debug("setArea")
	for _, v := range w.devInfos {
		err := w._setArea(v)
		if err != nil {
			logger.Warning(err)
		}
	}
}

func (w *Wacom) _setArea(dw *dxinput.Wacom) error {
	err := dw.ResetArea()
	if err != nil {
		return err
	}

	if !w.ForceProportions.Get() {
		logger.Debugf("device %d ResetArea", dw.Id)
		return nil
	}

	x1, y1, x2, y2, err := dw.GetArea()
	if err != nil {
		return err
	}
	if x1 != 0 || y1 != 0 {
		return fmt.Errorf("wacom device %d top left corner of the area is not at the origin", dw.Id)
	}

	screenWidth := float64(w.mapToOutput.W)
	screenHeight := float64(w.mapToOutput.H)

	tabletWidth := float64(x2)
	tabletHeight := float64(y2)

	screenRatio := screenWidth / screenHeight
	logger.Debugf("screeRatio %v", screenRatio)
	tabletRatio := tabletWidth / tabletHeight

	if screenRatio > tabletRatio {
		// cut off the bottom of the drawing area
		tabletHeight = tabletWidth / screenWidth * screenHeight
	} else if screenRatio < tabletRatio {
		// cut off the right part of the drawing area
		tabletWidth = tabletHeight / screenHeight * screenWidth
	}

	logger.Debugf("device %d setArea %d,%d", dw.Id, int(tabletWidth), int(tabletHeight))
	return dw.SetArea(0, 0, int(tabletWidth), int(tabletHeight))
}

func (w *Wacom) getRawSample(devType int) (uint32, error) {
	var rawSample uint32
	switch devType {
	case dxinput.WacomTypeStylus:
		rawSample = w.StylusRawSample.Get()
	case dxinput.WacomTypeEraser:
		rawSample = w.EraserRawSample.Get()
	default:
		return 0, fmt.Errorf("Invalid wacom device type")
	}
	return rawSample, nil
}

func (w *Wacom) setRawSampleForType(devType int) {
	for _, v := range w.devInfos {
		if v.QueryType() == devType {
			rawSample, err := w.getRawSample(devType)
			if err != nil {
				logger.Warning(err)
				continue
			}
			err = v.SetRawSample(rawSample)
			if err != nil {
				logger.Warningf("Set raw sample for '%v - %v' to %v failed: %v",
					v.Id, v.Name, rawSample, err)
			}
		}
	}
}

func (w *Wacom) setRawSample() {
	w.setRawSampleForType(dxinput.WacomTypeStylus)
	w.setRawSampleForType(dxinput.WacomTypeEraser)
}

func (w *Wacom) getThreshold(devType int) (int, error) {
	var threshold int
	switch devType {
	case dxinput.WacomTypeStylus:
		threshold = int(w.StylusThreshold.Get())
	case dxinput.WacomTypeEraser:
		threshold = int(w.EraserThreshold.Get())
	default:
		return 0, fmt.Errorf("Invalid wacom device type")
	}
	return threshold, nil
}

func (w *Wacom) setThresholdForType(devType int) {
	for _, v := range w.devInfos {
		if v.QueryType() == devType {
			threshold, err := w.getThreshold(devType)
			if err != nil {
				logger.Warning(err)
				continue
			}
			err = v.SetThreshold(threshold)
			if err != nil {
				logger.Warningf("Set threshold for '%v - %v' to %v failed: %v",
					v.Id, v.Name, threshold, err)
			}
		}
	}
}

func (w *Wacom) setThreshold() {
	w.setThresholdForType(dxinput.WacomTypeStylus)
	w.setThresholdForType(dxinput.WacomTypeEraser)
}

func (w *Wacom) destroy() {
	w.xConn.Close()
	close(w.exit)
}
