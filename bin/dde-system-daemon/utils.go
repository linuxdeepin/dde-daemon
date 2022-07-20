package main

import (
	"fmt"

	"github.com/godbus/dbus"
)

const (
	nmService      = "org.freedesktop.NetworkManager"
	nmSettingsPath = "/org/freedesktop/NetworkManager/Settings"
	nmSettingsIFC  = nmService + ".Settings"

	methodNMReloadConns = nmSettingsIFC + ".ReloadConnections"
)

var (
	nmSettingsObj dbus.BusObject
)

func reloadConnections() error {
	obj, err := newSettingsBus()
	if err != nil {
		return err
	}
	var success bool
	err = obj.Call(methodNMReloadConns, 0).Store(&success)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("reload connections failed")
	}
	return nil
}

func newSettingsBus() (dbus.BusObject, error) {
	if nmSettingsObj != nil {
		return nmSettingsObj, nil
	}
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	nmSettingsObj = conn.Object(nmService, nmSettingsPath)
	return nmSettingsObj, nil
}

func startBacklightHelperAsync(conn *dbus.Conn) {
	go func() {
		obj := conn.Object("org.deepin.daemon.helper.Backlight1", "/org/deepin/daemon/helper/Backlight1")
		err := obj.Call("org.freedesktop.DBus.Peer.Ping", 0).Err

		if err != nil {
			logger.Warning(err)
		}
	}()
}
