// SPDX-FileCopyrightText: 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"github.com/linuxdeepin/dde-daemon/common/scale"
	"github.com/linuxdeepin/dde-daemon/loader"
	"github.com/linuxdeepin/go-lib/log"
	x "github.com/linuxdeepin/go-x11-client"
	"os"
)

var logger = log.NewLogger("xsettings")

type daemon struct {
	*loader.ModuleBase
	xsManager *XSManager
}

func init() {
	loader.Register(NewModule(logger))
}
func NewModule(logger *log.Logger) *daemon {
	var d = new(daemon)
	d.ModuleBase = loader.NewModuleBase("xsettings", d, logger)
	return d
}

func (*daemon) GetDependencies() []string {
	return []string{"display"}
}

func (d *daemon) Start() error {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		logger.Warning("in wayland mode, not support wayland")
		return nil
	}
	service := loader.GetService()
	xConn, err := x.NewConn()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}
	var recommendedScaleFactor float64
	recommendedScaleFactor = scale.GetRecommendedScaleFactor(xConn)

	d.xsManager, err = Start(xConn, recommendedScaleFactor, service)
	if err != nil {
		logger.Warning(err)
	}
	return nil
}

func (d *daemon) Stop() error {
	d.xsManager.sysDaemon.RemoveAllHandlers()
	d.xsManager.service.StopExport(d.xsManager)
	d.xsManager.sessionSigLoop.Stop()
	return nil
}
