/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package keybinding

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"dbus/com/deepin/daemon/network"

	ddbus "pkg.deepin.io/dde/daemon/dbus"
	"pkg.deepin.io/lib/dbus"
)

var (
	// enable by 'go build -ldflags "-X pkg.deepin.io/dde/daemon/keybinding.ManageWireless=enabled"'
	ManageWireless = "disabled"
)

func toggleWireless() error {
	net, err := Network.NewNetworkManager("com.deepin.daemon.Network",
		"/com/deepin/daemon/Network")
	if err != nil {
		return err
	}
	defer Network.DestroyNetworkManager(net)

	if !ddbus.IsSystemBusActivated(net.DestName) {
		return fmt.Errorf("Network service no activation")
	}

	list := getWirelessDevice(net.Devices.Get())
	enabled := false
	for _, dev := range list {
		ok, _ := net.IsDeviceEnabled(dbus.ObjectPath(dev))
		if ok {
			enabled = true
			break
		}
	}

	for _, dev := range list {
		net.EnableDevice(dbus.ObjectPath(dev), !enabled)
	}
	return nil
}

type deviceInfo struct {
	Path string `json:"Path"`
}

type wirelessDevice struct {
	Devices []deviceInfo `json:"wireless"`
}

func getWirelessDevice(value string) []string {
	var wireless wirelessDevice
	err := json.Unmarshal([]byte(value), &wireless)
	if err != nil {
		return nil
	}
	var list []string
	for _, dev := range wireless.Devices {
		list = append(list, dev.Path)
	}
	return list
}

func getRfkillWlanState() (int, error) {
	dir := "/sys/class/rfkill"
	fileInfoList, err := ioutil.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	for _, fileInfo := range fileInfoList {
		typeFile := filepath.Join(dir, fileInfo.Name(), "type")
		typeBytes, err := readTinyFile(typeFile)
		if err != nil {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(typeBytes), []byte("wlan")) {
			stateFile := filepath.Join(dir, fileInfo.Name(), "state")
			stateBytes, err := readTinyFile(stateFile)
			if err != nil {
				return 0, err
			}
			stateBytes = bytes.TrimSpace(stateBytes)
			state, err := strconv.Atoi(string(stateBytes))
			if err != nil {
				return 0, err
			}
			return state, nil

		}
	}
	return 0, errors.New("not found rfkill with type wlan")
}

func readTinyFile(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, 8)
	n, err := f.Read(buf)

	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}
