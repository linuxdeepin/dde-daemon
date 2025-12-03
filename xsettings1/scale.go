// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/userenv"
	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

const (
	EnvDeepinWineScale           = "DEEPIN_WINE_SCALE"
	gsKeyScaleFactor             = "scale-factor"
	gsKeyWindowScale             = "window-scale"
	gsKeyGtkCursorThemeSize      = "gtk-cursor-theme-size"
	gsKeyGtkCursorThemeName      = "gtk-cursor-theme-name"
	gsKeyGtkCursorThemeSizeBase  = "gtk-cursor-theme-size-base"
	gsKeyIndividualScaling       = "individual-scaling"
	qtThemeSection               = "Theme"
	qtThemeKeyScreenScaleFactors = "ScreenScaleFactors"
	qtThemeKeyScaleFactor        = "ScaleFactor"
	qtThemeKeyScaleLogicalDpi    = "ScaleLogicalDpi"
)

// 设置单个缩放值的关键方法
func (m *XSManager) setScaleFactor(scale float64, emitSignal bool) {
	logger.Debug("setScaleFactor", scale)
	m.xsettingsConfig.SetValue(gsKeyScaleFactor, scale)

	// if 1.7 < scale < 2, window scale = 2
	windowScale := int32(math.Trunc((scale+0.3)*10) / 10)
	if windowScale < 1 {
		windowScale = 1
	}
	oldWindowScale, _ := m.xsettingsConfig.GetValueInt64(gsKeyWindowScale)
	if int32(oldWindowScale) != windowScale {
		m.xsettingsConfig.SetValue(gsKeyWindowScale, windowScale)
	}

	// 设置系统缩放之后，需要同时缩放光标大小
	baseCursorSizeInt, err := m.xsettingsConfig.GetValueInt(gsKeyGtkCursorThemeSizeBase)
	if err != nil || baseCursorSizeInt <= 0 {
		baseCursorSizeInt = 24
	}
	baseCursorSize := float64(baseCursorSizeInt)

	cursorSize := int32(baseCursorSize * scale)
	m.xsettingsConfig.SetValue(gsKeyGtkCursorThemeSize, cursorSize)
	// set cursor size for deepin-metacity
	gsWrapGDI := gio.NewSettings("com.deepin.wrap.gnome.desktop.interface")
	gsWrapGDI.SetInt("cursor-size", cursorSize)
	gsWrapGDI.Unref()

	// set cursor size for deepin-kwin
	conn, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
	} else {
		if err := conn.Object("com.deepin.wm",
			"/com/deepin/wm").SetProperty("com.deepin.wm.cursorSize", dbus.MakeVariant(cursorSize)); err != nil {
			logger.Warning(err)
		}
	}

	m.emitSignalSetScaleFactor(true, emitSignal)
}

func parseScreenFactors(str string) map[string]float64 {
	pairs := strings.Split(str, ";")
	result := make(map[string]float64)
	for _, value := range pairs {
		kv := strings.SplitN(value, "=", 2)
		if len(kv) != 2 {
			continue
		}

		value, err := strconv.ParseFloat(kv[1], 64)
		if err != nil {
			logger.Warning(err)
			continue
		}

		result[kv[0]] = value
	}

	return result
}

func joinScreenScaleFactors(v map[string]float64) string {
	pairs := make([]string, len(v))
	idx := 0
	for key, value := range v {
		pairs[idx] = fmt.Sprintf("%s=%.2f", key, value)
		idx++
	}
	return strings.Join(pairs, ";")
}

func getQtThemeFile() string {
	return filepath.Join(basedir.GetUserConfigDir(), "deepin/qt-theme.ini")
}

func cleanUpDdeEnv() error {
	ue, err := userenv.Load()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	needSave := false
	for _, key := range []string{
		"QT_SCALE_FACTOR",
		"QT_SCREEN_SCALE_FACTORS",
		"QT_AUTO_SCREEN_SCALE_FACTOR",
		"QT_FONT_DPI",
		EnvDeepinWineScale,
	} {
		if _, ok := ue[key]; ok {
			delete(ue, key)
			needSave = true
		}
	}

	if needSave {
		err = userenv.Save(ue)
	}
	return err
}

func (m *XSManager) setScreenScaleFactorsForQt(factors map[string]float64) error {
	filename := getQtThemeFile()
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(filename)
	if err != nil && !os.IsNotExist(err) {
		logger.Warning("failed to load qt-theme.ini:", err)
	}

	var value string
	switch len(factors) {
	case 0:
		return errors.New("factors is empty")
	case 1:
		value = strconv.FormatFloat(getMapFirstValueSF(factors), 'f', 2, 64)
	default:
		value = joinScreenScaleFactors(factors)
		value = strconv.Quote(value)
	}
	kf.SetValue(qtThemeSection, qtThemeKeyScreenScaleFactors, value)
	kf.DeleteKey(qtThemeSection, qtThemeKeyScaleFactor)
	kf.SetValue(qtThemeSection, qtThemeKeyScaleLogicalDpi, "-1,-1")

	err = os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		return err
	}

	err = kf.SaveToFile(filename)
	if err != nil {
		return err
	}

	err = m.updateGreeterQtTheme(kf)
	return err
}

func getMapFirstValueSF(m map[string]float64) float64 {
	for _, value := range m {
		return value
	}
	return 0
}

var (
	_sessionConn *dbus.Conn
)

// 不发送通知版本, 设置流程会转到 setScreenScaleFactors
func (m *XSManager) setScaleFactorWithoutNotify(scale float64) error {
	err := m.setScreenScaleFactors(singleToMapSF(scale), false)
	return err
}

func getSingleScaleFactor(factors map[string]float64) float64 {
	if len(factors) == 0 {
		return 1
	}
	if len(factors) == 1 {
		return getMapFirstValueSF(factors)
	}
	v, ok := factors["ALL"]
	if ok {
		return v
	}
	return 1
}

func singleToMapSF(value float64) map[string]float64 {
	return map[string]float64{
		"ALL": value,
	}
}

// 设置多屏的缩放比例的关键方法，factors 中必须含有主屏的数据。
func (m *XSManager) setScreenScaleFactors(factors map[string]float64, emitSignal bool) error {
	logger.Debug("setScreenScaleFactors", factors)
	for _, f := range factors {
		if f <= 0 {
			return errors.New("invalid value")
		}
	}
	if len(factors) == 0 {
		return errors.New("factors is empty")
	}

	// 同时要设置单值的
	singleFactor := getSingleScaleFactor(factors)
	m.setScaleFactor(singleFactor, emitSignal)

	// 关键保存位置
	factorsJoined := joinScreenScaleFactors(factors)
	if !reflect.DeepEqual(factors, m.getScreenScaleFactors()) {
		m.xsettingsConfig.SetValue(gsKeyIndividualScaling, factorsJoined)
	}

	err := m.setScreenScaleFactorsForQt(factors)
	if err != nil {
		return err
	}

	err = cleanUpDdeEnv()
	if err != nil {
		logger.Warning("failed to clean up dde env", err)
	}

	return err
}

func (m *XSManager) getScreenScaleFactors() map[string]float64 {
	factorsJoined, _ := m.xsettingsConfig.GetValueString(gsKeyIndividualScaling)
	return parseScreenFactors(factorsJoined)
}

func (m *XSManager) emitSignalSetScaleFactor(done, emitSignal bool) {
	if !emitSignal {
		return
	}
	signalName := "SetScaleFactorStarted"
	if done {
		signalName = "SetScaleFactorDone"
	}
	err := m.service.Emit(m, signalName)
	if err != nil {
		logger.Warning(err)
	}
}

func getPlymouthTheme(file string) (string, error) {
	var kf = keyfile.NewKeyFile()
	err := kf.LoadFromFile(file)
	if err != nil {
		return "", err
	}

	return kf.GetString("Daemon", "Theme")
}

func (m *XSManager) updateGreeterQtTheme(kf *keyfile.KeyFile) error {
	tempFile, err := os.CreateTemp("", "startdde-qt-theme-")
	if err != nil {
		return err
	}
	defer func() {
		err := tempFile.Close()
		if err != nil {
			logger.Warning(err)
		}
		err = os.Remove(tempFile.Name())
		if err != nil {
			logger.Warning(err)
		}
	}()

	kf.SetValue(qtThemeSection, qtThemeKeyScaleLogicalDpi, "96,96")
	err = kf.SaveToWriter(tempFile)
	if err != nil {
		return err
	}

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	err = m.greeter.UpdateGreeterQtTheme(0, dbus.UnixFD(tempFile.Fd()))
	return err
}
