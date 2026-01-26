// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package inputdevices

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	"github.com/linuxdeepin/dde-daemon/common/dsync"
	kwin "github.com/linuxdeepin/go-dbus-factory/session/org.kde.kwin"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

const (
	imWheelBin = "imwheel"

	dsettingsAppID        = "org.deepin.dde.daemon"
	dsettingsInputdevices = "org.deepin.dde.daemon.inputdevices"

	dconfigKeyWheelSpeed = "wheelSpeed"
)

type devicePathInfo struct {
	Path string
	Type string
}
type devicePathInfos []*devicePathInfo

// 是否是treeland环境
var hasTreeLand = false

type Manager struct {
	Infos      devicePathInfos // readonly
	WheelSpeed dconfig.Uint32  `prop:"access:rw"`

	dsgInputdevices   *dconfig.DConfig
	imWheelConfigFile string

	kbd        *Keyboard
	mouse      *Mouse
	trackPoint *TrackPoint
	tpad       *Touchpad
	wacom      *Wacom

	kwinManager kwin.InputDeviceManager
	kwinIdList  []dbusutil.SignalHandlerId

	sessionSigLoop *dbusutil.SignalLoop
	syncConfig     *dsync.Config
}

func NewManager(service *dbusutil.Service) *Manager {
	if os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		hasTreeLand = true
	}
	var m = new(Manager)
	m.imWheelConfigFile = filepath.Join(basedir.GetUserHomeDir(), ".imwheelrc")

	m.Infos = devicePathInfos{
		&devicePathInfo{
			Path: kbdDBusInterface,
			Type: "keyboard",
		},
		&devicePathInfo{
			Path: mouseDBusInterface,
			Type: "mouse",
		},
		&devicePathInfo{
			Path: trackPointDBusInterface,
			Type: "trackpoint",
		},
		&devicePathInfo{
			Path: touchPadDBusInterface,
			Type: "touchpad",
		},
	}

	if !hasTreeLand {
		m.kwinManager = kwin.NewInputDeviceManager(service.Conn())
	}

	if err := m.initDConfig(); err != nil {
		logger.Errorf("Failed to initialize dconfig: %v", err)
		panic("DConfig initialization failed - cannot continue without dconfig support")
	}

	m.kbd = newKeyboard(service)
	m.wacom = newWacom(service)

	m.tpad = newTouchpad(service)

	m.mouse = newMouse(service, m.tpad, m.sessionSigLoop)

	m.trackPoint = newTrackPoint(service)

	m.sessionSigLoop = dbusutil.NewSignalLoop(service.Conn(), 10)
	m.syncConfig = dsync.NewConfig("peripherals", &syncConfig{m: m},
		m.sessionSigLoop, dbusPath, logger)

	return m
}

func (m *Manager) setWheelSpeed() {
	min := 1
	max := 100
	speed := m.WheelSpeed.Get()
	// speed range is [1,100]
	if speed < uint32(min) {
		speed = uint32(min)
	} else if speed > uint32(max) {
		speed = uint32(max)
	}
	logger.Debug("setWheelSpeed", speed)

	// 为了避免imwheel对kwin影响，先杀死imwheel
	err := exec.Command("pkill", "-ef", imWheelBin).Run()
	if err != nil {
		logger.Warning(err)
	}

	// TODO: treeland环境没有kwin
	if !hasTreeLand {
		err = m.setWaylandWheelSpeed(speed)
		if err == nil {
			logger.Info("set Wayland WheelSpeed finish")
			return
		}
	}

	logger.Info("can not set WheelSpeed by Wayland interface, use imwheel")
	// 通过kwin设置wheel speed失败时候才通过命令设置
	err = writeImWheelConfig(m.imWheelConfigFile, speed)
	if err != nil {
		logger.Warning("failed to write imwheel config file:", err)
		return
	}

	err = controlImWheel(speed)
	if err != nil {
		logger.Warning("failed to control imwheel:", err)
		return
	}
}

func controlImWheel(speed uint32) error {
	if speed == 1 {
		// quit
		return exec.Command(imWheelBin, "-k", "-q").Run()
	}
	// restart
	return exec.Command(imWheelBin, "-k", "-b", "4 5").Run()
}

func writeImWheelConfig(file string, speed uint32) error {
	logger.Debugf("writeImWheelConfig file:%q, speed: %d", file, speed)

	const header = `# written by ` + dbusServiceName + `
".*"
Control_L,Up,Control_L|Button4
Control_R,Up,Control_R|Button4
Control_L,Down,Control_L|Button5
Control_R,Down,Control_R|Button5
Shift_L,Up,Shift_L|Button4
Shift_R,Up,Shift_R|Button4
Shift_L,Down,Shift_L|Button5
Shift_R,Down,Shift_R|Button5
`
	fh, err := os.Create(file)
	if err != nil {
		return err
	}
	defer fh.Close()
	writer := bufio.NewWriter(fh)

	_, err = writer.Write([]byte(header))
	if err != nil {
		return err
	}

	//  Delay Before Next KeyPress Event
	delay := 240000 / speed
	_, err = fmt.Fprintf(writer, "None,Up,Button4,%d,0,%d\n", speed, delay)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(writer, "None,Down,Button5,%d,0,%d\n", speed, delay)
	if err != nil {
		return err
	}

	err = writer.Flush()
	if err != nil {
		return err
	}

	return fh.Sync()
}

func (m *Manager) init() {
	// 初始化dconfig并同步配置
	if err := initDConfig(); err != nil {
		logger.Warning("dconfig init failed, using default settings")
	}

	m.kbd.init()

	//TODO: treeland环境暂不支持如下设备
	if !hasTreeLand {
		m.wacom.init()
		m.tpad.init()
		m.mouse.init()

		// 设置全局mouse引用，用于属性同步
		SetGlobalMouse(m.mouse)

		m.trackPoint.init()
	}

	m.setWheelSpeed()

	m.sessionSigLoop.Start()
}

func (m *Manager) initDConfig() error {
	// 创建配置管理器实例
	err := error(nil)
	m.dsgInputdevices, err = dconfig.NewDConfig(dsettingsAppID, dsettingsInputdevices, "")
	if err != nil {
		return err
	}

	m.WheelSpeed.Bind(m.dsgInputdevices, dconfigKeyWheelSpeed)

	m.dsgInputdevices.ConnectConfigChanged(dconfigKeyWheelSpeed, func(i interface{}) {
		m.setWheelSpeed()
	})

	logger.Info("DConfig initialization completed successfully")
	return nil
}
