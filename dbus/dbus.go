// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dbus

import (
	"github.com/godbus/dbus/v5"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

// IsSessionBusActivated check the special session bus name whether activated
func IsSessionBusActivated(dest string) bool {
	if !lib.UniqueOnSession(dest) {
		return true
	}

	bus, _ := dbus.SessionBus()
	releaseDBusName(bus, dest)
	return false
}

// IsSystemBusActivated check the special system bus name whether activated
func IsSystemBusActivated(dest string) bool {
	if !lib.UniqueOnSystem(dest) {
		return true
	}

	bus, _ := dbus.SystemBus()
	releaseDBusName(bus, dest)
	return false
}

func releaseDBusName(bus *dbus.Conn, name string) {
	if bus != nil {
		_, _ = bus.ReleaseName(name)
	}
}

// GetSessionType 通过 UID 从 login1 获取 session type，比使用 PID 更安全
// 返回 session type: "x11", "wayland", "tty" 等
func GetSessionType(service *dbusutil.Service, sender dbus.Sender) (string, error) {
	// 使用 GetConnUID 替代 GetConnPID，更安全
	uid, err := service.GetConnUID(string(sender))
	if err != nil {
		return "", err
	}

	// 获取系统总线连接
	systemConn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}

	// 创建 login1 Manager
	loginManager := login1.NewManager(systemConn)

	// 通过 UID 获取用户对象路径
	userPath, err := loginManager.GetUser(0, uid)
	if err != nil {
		return "", err
	}

	// 创建 User 对象
	user, err := login1.NewUser(systemConn, userPath)
	if err != nil {
		return "", err
	}

	// 获取用户的 Display session（主要的图形会话）
	sessionInfo, err := user.Display().Get(0)
	if err != nil {
		return "", err
	}

	// 创建 Session 对象
	session, err := login1.NewSession(systemConn, sessionInfo.Path)
	if err != nil {
		return "", err
	}

	// 获取 session type (x11, wayland, tty 等)
	sessionType, err := session.Type().Get(0)
	if err != nil {
		return "", err
	}

	return sessionType, nil
}
