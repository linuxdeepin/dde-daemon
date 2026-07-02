// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package brightness

import (
	"fmt"
	"math"
	"sync"

	"github.com/godbus/dbus/v5"
	backlight "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.backlighthelper1"
	displayBl "github.com/linuxdeepin/go-lib/backlight/display"
	"github.com/linuxdeepin/go-lib/log"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

var _useWayland bool

func SetUseWayland(value bool) {
	_useWayland = value
}

const (
	SetterAuto      = 0 // auto
	SetterGamma     = 1 // gamma
	SetterBacklight = 2 // backlight
	SetterDDCCI     = 3
	SetterDRM       = 4
)

var logger = log.NewLogger("daemon/display/brightness")

var helper backlight.Backlight
var ddcciHelper backlight.DDCCI

func InitBacklightHelper() {
	var err error
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return
	}
	helper = backlight.NewBacklight(sysBus)
	ddcciHelper = backlight.NewDDCCI(sysBus)
}

// SetOutputGama 设置 Gamma 亮度和色温
func SetOutputGama(brightness float64, temperature int, outputId uint32, conn *x.Conn, uuid string) error {
	output := randr.Output(outputId)
	return setOutputCrtcGamma(gammaSetting{
		brightness:  brightness,
		temperature: temperature,
	}, output, conn)
}

// SupportBacklight 检查是否支持背光调节
func SupportBacklight() bool {
	return supportBacklight()
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

func _setBacklight(value float64, controller *displayBl.Controller) error {
	br := int32(float64(controller.MaxBrightness) * value)

	v, ok := GetBacklightCurveValue(value, controller)
	if ok {
		logger.Debugf("Brightness curve value: %v", v)
		br = v
	}

	const backlightTypeDisplay = 1
	fmt.Printf("help set brightness %q max %v value %v br %v\n",
		controller.Name, controller.MaxBrightness, value, br)
	return helper.SetBrightness(0, backlightTypeDisplay, controller.Name, br)
}

// backlightControllerCache 背光控制器缓存
var (
	cachedBacklightControllers displayBl.Controllers
	backlightControllersMu     sync.RWMutex
)

// getBacklightControllers 获取缓存的背光控制器列表
func getBacklightControllers() displayBl.Controllers {
	backlightControllersMu.RLock()
	if len(cachedBacklightControllers) > 0 {
		defer backlightControllersMu.RUnlock()
		return cachedBacklightControllers
	}
	backlightControllersMu.RUnlock()

	backlightControllersMu.Lock()
	defer backlightControllersMu.Unlock()

	// 双重检查
	if len(cachedBacklightControllers) > 0 {
		return cachedBacklightControllers
	}

	var err error
	cachedBacklightControllers, err = displayBl.List()
	if err != nil {
		logger.Warningf("Failed to list backlight controllers: %v", err)
		return nil
	}
	return cachedBacklightControllers
}

// SetBacklight 设置背光亮度（供 TransitionManager 回调使用）
func SetBacklight(brightness float64) error {
	controllers := getBacklightControllers()
	if len(controllers) == 0 {
		return fmt.Errorf("no backlight controllers available")
	}

	for _, controller := range controllers {
		err := _setBacklight(brightness, controller)
		if err != nil {
			logger.Warningf("Failed to set backlight %s: %v", controller.Name, err)
		}
	}
	return nil
}

// GetBacklightCurrentValue 获取当前背光亮度百分比 (0.0 - 1.0)
func GetBacklightCurrentValue() (float64, error) {
	controllers := getBacklightControllers()
	if len(controllers) == 0 {
		return 0.5, fmt.Errorf("no backlight controllers available")
	}

	// 使用第一个控制器的当前亮度
	controller := controllers[0]
	currentBrightness, err := controller.GetActualBrightness()
	if err != nil {
		return 0.5, fmt.Errorf("failed to get current brightness: %v", err)
	}

	if controller.MaxBrightness <= 0 {
		return 0.5, fmt.Errorf("invalid max brightness: %d", controller.MaxBrightness)
	}

	return float64(currentBrightness) / float64(controller.MaxBrightness), nil
}

// RefreshAndSupportDDCCIBrightness 刷新 DDCCI 显示列表并返回指定显示器是否支持
// DDCCI 亮度调节。调用方应在独立 goroutine 中调用（例如 time.AfterFunc 回调），
// 因为 RefreshDisplays 探测 I2C 总线较慢。
func RefreshAndSupportDDCCIBrightness(edidBase64 string) bool {
	if helper == nil {
		return false
	}
	res, err := helper.CheckCfgSupport(0, "ddcci")
	if err != nil {
		logger.Warningf("brightness: failed to check ddc/ci support: %v", err)
		return false
	}
	if !res {
		return false
	}
	logger.Debug("refresh ddcci display")
	if err = ddcciHelper.RefreshDisplays(0); err != nil {
		logger.Warningf("brightness: failed to refresh ddc/ci display list: %v", err)
		return false
	}
	res, err = ddcciHelper.CheckSupport(0, edidBase64)
	if err != nil {
		logger.Warningf("brightness: failed to check ddc/ci support: %v", err)
		return false
	}
	return res
}

func SetDDCCIBrightness(value float64, edidBase64 string) error {
	if helper == nil {
		return fmt.Errorf("brightness: backlight helper not initialized")
	}
	res, err := helper.CheckCfgSupport(0, "ddcci")
	if err != nil {
		logger.Warningf("brightness: failed to check ddc/ci support: %v", err)
		return err
	}
	if !res {
		logger.Warning("brightness: check ddc/ci config not support")
		return nil
	}
	// 限制 value 在 [0, 1] 范围内，避免 DDC/CI percent 超出 0-100 规范
	if value < 0 {
		value = 0
	} else if value > 1 {
		value = 1
	}
	percent := int32(value * 100)
	logger.Debugf("brightness: ddcci set brightness %d", percent)
	return ddcciHelper.SetBrightness(0, edidBase64, percent)
}

func getDDCCIBrightness(edidBase64 string) (float64, error) {
	br, err := ddcciHelper.GetBrightness(0, edidBase64)
	if err != nil {
		return 1, err
	} else {
		return float64(br) / 100.0, err
	}
}

// GetDDCCIBrightness 获取 DDCCI 亮度值（公开接口）
func GetDDCCIBrightness(edidBase64 string) (float64, error) {
	return getDDCCIBrightness(edidBase64)
}
