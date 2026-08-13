// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package langselector

import (
	"fmt"
	"os"
	"os/user"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/securityloader"
)

var defaultDestList = []securityloader.Destination{
	{
		DBusName:      "org.deepin.dde.LocaleHelper1",
		DBusPath:      "/org/deepin/dde/LocaleHelper1",
		DBusInterface: "org.deepin.dde.LocaleHelper1",
	},
	{
		DBusName:      "org.deepin.dde.Lastore1",
		DBusPath:      "/org/deepin/dde/Lastore1",
		DBusInterface: "org.deepin.dde.Lastore1.Manager",
	},
}

func DoSecurityLoader(args []string) {
	// 构建 destList，包括 lastore 和 accounts
	destList := buildDestList()

	_, loaded, err := securityloader.Handshake(args, destList)
	if err == nil {
		return
	}
	if loaded {
		logger.Errorf("security loader handshake failed, refusing to start: %q", err.Error())
		os.Exit(1)
	}
	logger.Warningf("security loader handshake skipped: %q", err.Error())
}

// buildDestList 构建需要授权的 D-Bus 接口列表
func buildDestList() []securityloader.Destination {
	destList := make([]securityloader.Destination, len(defaultDestList))
	copy(destList, defaultDestList)

	// 获取当前用户的 Accounts.User 路径
	userPath, err := getCurrentUserAccountsPath()
	if err != nil {
		logger.Warning("failed to get current user accounts path:", err)
		return destList
	}

	destList = append(destList, securityloader.Destination{
		DBusName:      "org.deepin.dde.Accounts1",
		DBusPath:      userPath,
		DBusInterface: "org.deepin.dde.Accounts1.User",
	})

	return destList
}

// getCurrentUserAccountsPath 获取当前用户在 Accounts 服务中的路径
func getCurrentUserAccountsPath() (string, error) {
	systemConn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}

	// 获取当前用户 UID
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}

	// 通过 D-Bus 调用 FindUserById
	obj := systemConn.Object("org.deepin.dde.Accounts1", "/org/deepin/dde/Accounts1")
	var userPath string
	err = obj.Call("org.deepin.dde.Accounts1.FindUserById", 0, currentUser.Uid).Store(&userPath)
	if err != nil {
		return "", fmt.Errorf("dbus call FindUserById failed: %w", err)
	}

	if userPath == "" {
		return "", fmt.Errorf("received empty user path for uid: %s", currentUser.Uid)
	}

	return userPath, nil
}
