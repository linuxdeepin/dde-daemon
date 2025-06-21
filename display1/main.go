// SPDX-FileCopyrightText: 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"os"
	"os/exec"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/loader"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gsettings"
	"github.com/linuxdeepin/go-lib/log"
	x "github.com/linuxdeepin/go-x11-client"
)

var logger = log.NewLogger("daemon/display")

var _xConn *x.Conn

type daemon struct {
	*loader.ModuleBase
}

func init() {
	loader.Register(NewModule(logger))
}
func NewModule(logger *log.Logger) *daemon {
	var d = new(daemon)
	d.ModuleBase = loader.NewModuleBase("display", d, logger)
	return d
}

func (*daemon) GetDependencies() []string {
	return []string{"xsettings"}
}

var _mainBeginTime time.Time

func logDebugAfter(msg string) {
	elapsed := time.Since(_mainBeginTime)
	logger.Debugf("after %s, %s", elapsed, msg)
}

func (*daemon) Start() error {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		_useWayland = true
		logger.Warning("in wayland mode, not support wayland")
		return nil
	}

	service := loader.GetService()

	_mainBeginTime = time.Now()

	// init x conn
	xConn, err := x.NewConn()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}
	_xConn = xConn

	_inVM, err = isInVM()

	Init(xConn, _useWayland, _inVM)

	err = Start(service)
	if err != nil {
		logger.Warning("start display part1 failed:", err)
	}

	// 启动 display 模块的后一部分
	go func() {
		err := StartPart2()
		if err != nil {
			logger.Warning("start display part2 failed:", err)
		}
	}()

	err = gsettings.StartMonitor()
	if err != nil {
		logger.Warning("gsettings start monitor failed:", err)
	}

	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}
	sysSignalLoop := dbusutil.NewSignalLoop(sysBus, 10)
	sysSignalLoop.Start()

	go func() {
		logger.Info("systemd-notify --ready")
		cmd := exec.Command("systemd-notify", "--ready")
		cmd.Run()
	}()
	return nil
}

func isInVM() (bool, error) {
	cmd := exec.Command("systemd-detect-virt", "-v", "-q")
	err := cmd.Start()
	if err != nil {
		return false, err
	}

	err = cmd.Wait()
	return err == nil, nil
}

func (*daemon) Stop() error {
	distory()
	return nil
}
