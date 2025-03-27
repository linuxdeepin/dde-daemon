// SPDX-FileCopyrightText: 2014 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "C"
import (
	"github.com/linuxdeepin/dde-daemon/display1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
	x "github.com/linuxdeepin/go-x11-client"
	"os"
)

var logger = log.NewLogger("greeter-display-daemon")

func main() {
	logger.Warning("greeter-display-daemon running")
	display1.SetGreeterMode(true)
	// init x conn
	xConn, err := x.NewConn()
	if err != nil {
		logger.Warning(err)
		os.Exit(1)
	}
	// TODO
	display1.Init(xConn, false, false)
	logger.Debug("greeter mode")
	service, err := dbusutil.NewSessionService()
	if err != nil {
		logger.Warning(err)
	}
	err = display1.Start(service)
	if err != nil {
		logger.Warning(err)
	}
	err = display1.StartPart2()
	if err != nil {
		logger.Warning(err)
	}
	service.Wait()
}
