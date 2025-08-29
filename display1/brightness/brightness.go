// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package brightness

import (
	"fmt"
	"math"

	"github.com/godbus/dbus/v5"
	backlight "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.backlighthelper1"
	displayBl "github.com/linuxdeepin/go-lib/backlight/display"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/multierr"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

var _useWayland bool

func SetUseWayland(value bool) {
	_useWayland = value
}

const (
	BrightnessSetterAuto      = 0 // auto
	BrightnessSetterGamma     = 1 // gamma
	BrightnessSetterBacklight = 2 // backlight
)

var logger = log.NewLogger("daemon/display/brightness")

var helper backlight.Backlight

func InitBacklightHelper() {
	var err error
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return
	}
	helper = backlight.NewBacklight(sysBus)
}

func Set(brightness float64, temperature int, setter int, isBuiltin bool, outputId uint32, conn *x.Conn) error {
	if brightness < 0 {
		brightness = 0
	} else if brightness > 1 {
		brightness = 1
	}

	output := randr.Output(outputId)

	// 亮度和色温分开设置，亮度用背光，色温用 gamma
	setBlGamma := func() error {
		var errs error
		err := setBacklight(brightness, output, conn)
		if err != nil {
			errs = multierr.Append(errs, err)
		}

		err = setOutputCrtcGamma(gammaSetting{
			brightness:  1,
			temperature: temperature,
		}, output, conn)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
		return errs
	}

	// 亮度和色温都用 gamma 值设置
	setGamma := func() error {
		return setOutputCrtcGamma(gammaSetting{
			brightness:  brightness,
			temperature: temperature,
		}, output, conn)
	}

	setFn := setGamma
	switch setter {
	case BrightnessSetterBacklight:
		setFn = setBlGamma
	case BrightnessSetterAuto:
		if isBuiltin && supportBacklight() {
			setFn = setBlGamma
		}
		//case BrightnessSetterGamma
	}
	return setFn()
}

// unused function
//func Get(setter int, isButiltin bool, outputId uint32, conn *x.Conn) (float64, error) {
//	output := randr.Output(outputId)
//	switch setter {
//	case BrightnessSetterBacklight:
//		return getBacklightOnlyOne()
//	case BrightnessSetterGamma:
//		return 1, nil
//	}
//
//	// case BrightnessSetterAuto
//	if isButiltin {
//		if supportBacklight(output, conn) {
//			return getBacklight(output, conn)
//		}
//	}
//	return 1, nil
//}

func GetMaxBacklightBrightness() int {
	if len(controllers) == 0 {
		return 0
	}
	maxBrightness := controllers[0].MaxBrightness
	for _, controller := range controllers {
		if maxBrightness > controller.MaxBrightness {
			maxBrightness = controller.MaxBrightness
		}
	}
	return maxBrightness
}

func GetBacklightController(outputId uint32, conn *x.Conn) (*displayBl.Controller, error) {
	// TODO
	//output := randr.Output(outputId)
	//return getBacklightController(output, conn)
	return nil, nil
}

func supportBacklight() bool {
	if helper == nil {
		return false
	}
	return len(controllers) > 0
}

func setOutputCrtcGamma(setting gammaSetting, output randr.Output, conn *x.Conn) error {
	if _useWayland {
		return nil
	}

	outputInfo, err := randr.GetOutputInfo(conn, output, x.CurrentTime).Reply(conn)
	if err != nil {
		fmt.Printf("Get output(%v) failed: %v\n", output, err)
		return err
	}

	if outputInfo.Crtc == 0 || outputInfo.Connection != randr.ConnectionConnected {
		fmt.Printf("output(%s) no crtc or disconnected\n", outputInfo.Name)
		return fmt.Errorf("output(%v) unready", output)
	}

	gamma, err := randr.GetCrtcGammaSize(conn, outputInfo.Crtc).Reply(conn)
	if err != nil {
		fmt.Printf("Failed to get gamma size: %v\n", err)
		return err
	}

	if gamma.Size == 0 {
		return fmt.Errorf("output(%v) has invalid gamma size", output)
	}

	red, green, blue := initGammaRamp(int(gamma.Size))
	fillColorRamp(red, green, blue, setting)
	return randr.SetCrtcGammaChecked(conn, outputInfo.Crtc,
		red, green, blue).Check(conn)
}

func initGammaRamp(size int) (red, green, blue []uint16) {
	red = make([]uint16, size)
	green = make([]uint16, size)
	blue = make([]uint16, size)

	for i := 0; i < size; i++ {
		value := uint16(float64(i) / float64(size) * (math.MaxUint16 + 1))
		red[i] = value
		green[i] = value
		blue[i] = value
	}
	return
}

func genGammaRamp(size uint16, brightness float64) (red, green, blue []uint16) {
	red = make([]uint16, size)
	green = make([]uint16, size)
	blue = make([]uint16, size)

	step := uint16(65535 / uint32(size))
	for i := uint16(0); i < size; i++ {
		red[i] = uint16(float64(step*i) * brightness)
		green[i] = uint16(float64(step*i) * brightness)
		blue[i] = uint16(float64(step*i) * brightness)
	}
	return
}

var controllers displayBl.Controllers

func init() {
	var err error
	controllers, err = displayBl.List()
	if err != nil {
		fmt.Println("failed to list backlight controller:", err)
	}
}

func setBacklight(value float64, output randr.Output, conn *x.Conn) error {
	for _, controller := range controllers {
		err := _setBacklight(value, controller)
		if err != nil {
			fmt.Printf("WARN: failed to set backlight %s: %v", controller.Name, err)
		}
	}
	return nil
}

func _setBacklight(value float64, controller *displayBl.Controller) error {
	br := int32(float64(controller.MaxBrightness) * value)
	const backlightTypeDisplay = 1
	fmt.Printf("help set brightness %q max %v value %v br %v\n",
		controller.Name, controller.MaxBrightness, value, br)
	return helper.SetBrightness(0, backlightTypeDisplay, controller.Name, br)
}
