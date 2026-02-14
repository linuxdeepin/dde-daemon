//go:build noscheduler
// +build noscheduler

// SPDX-FileCopyrightText: 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"github.com/linuxdeepin/dde-daemon/loader"
	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("daemon/system/scheduler")

func init() {
	// 当使用 noscheduler 标签时，不注册scheduler模块
	logger.Info("scheduler module is disabled by build tag")
}

type Module struct {
	*loader.ModuleBase
}

func (m *Module) GetDependencies() []string {
	return nil
}

func (m *Module) Start() error {
	// 模块被禁用，不执行任何操作
	logger.Info("scheduler module is disabled, skip starting")
	return nil
}

func (m *Module) Stop() error {
	// 模块被禁用，不执行任何操作
	return nil
}

func newModule(logger *log.Logger) *Module {
	m := new(Module)
	m.ModuleBase = loader.NewModuleBase("scheduler", m, logger)
	return m
}
