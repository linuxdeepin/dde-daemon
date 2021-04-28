/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
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

package network

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	nmdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.networkmanager"
	"pkg.deepin.io/dde/daemon/iw"
	"pkg.deepin.io/dde/daemon/network/nm"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/utils"
)

func isStringInArray(s string, list []string) bool {
	for _, i := range list {
		if i == s {
			return true
		}
	}
	return false
}

func stringArrayBut(list []string, ignoreList ...string) (newList []string) {
	for _, s := range list {
		if !isStringInArray(s, ignoreList) {
			newList = append(newList, s)
		}
	}
	return
}

func appendStrArrayUnique(a1 []string, a2 ...string) (a []string) {
	a = a1
	for _, s := range a2 {
		if !isStringInArray(s, a) {
			a = append(a, s)
		}
	}
	return
}

func removeStrArray(a1 []string, a2 ...string) (a []string) {
	for _, s := range a2 {
		for _, v := range a1 {
			if v == s {
				continue
			}
			a = append(a, v)
		}
	}
	return
}

func isDBusPathInArray(path dbus.ObjectPath, pathList []dbus.ObjectPath) bool {
	for _, i := range pathList {
		if i == path {
			return true
		}
	}
	return false
}

func isInterfaceNil(v interface{}) bool {
	return utils.IsInterfaceNil(v)
}

func isInterfaceEmpty(v interface{}) bool {
	if isInterfaceNil(v) {
		return true
	}
	switch v.(type) {
	case [][]interface{}: // ipv6Addresses
		if vd, ok := v.([][]interface{}); ok {
			if len(vd) == 0 {
				return true
			}
		}
	}
	return false
}

func marshalJSON(v interface{}) (jsonStr string, err error) {
	b, err := json.Marshal(v)
	if err != nil {
		logger.Error(err)
		return
	}
	jsonStr = string(b)
	return
}

func unmarshalJSON(jsonStr string, v interface{}) (err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	if err != nil {
		logger.Error(err)
	}
	return
}

func isUint32ArrayEmpty(a []uint32) (empty bool) {
	empty = true
	for _, v := range a {
		if v != 0 {
			empty = false
			break
		}
	}
	return
}

// convert local path to uri, etc "/the/path" -> "file:///the/path"
func toUriPath(path string) (uriPath string) {
	return utils.EncodeURI(path, utils.SCHEME_FILE)
}

// convert uri to local path, etc "file:///the/path" -> "/the/path"
func toLocalPath(path string) (localPath string) {
	return utils.DecodeURI(path)
}

// convert local path to uri, etc "/the/path" -> "file:///the/path"
func toUriPathFor8021x(path string) (uriPath string) {
	// the uri for 8021x cert files is specially, we just need append
	// suffix "file://" for it
	if !utils.IsURI(path) {
		uriPath = "file://" + path
	} else {
		uriPath = path
	}
	return
}

// convert uri to local path, etc "file:///the/path" -> "/the/path"
func toLocalPathFor8021x(path string) (uriPath string) {
	// the uri for 8021x cert files is specially, we just need remove
	// suffix "file://" from it
	if utils.IsURI(path) {
		uriPath = strings.TrimPrefix(path, "file://")
	} else {
		uriPath = path
	}
	return
}

// byte array should end with null byte
func strToByteArrayPath(path string) (bytePath []byte) {
	bytePath = []byte(path)
	bytePath = append(bytePath, 0)
	return
}
func byteArrayToStrPath(bytePath []byte) (path string) {
	if len(bytePath) < 1 {
		return
	}
	path = string(bytePath[:len(bytePath)-1])
	return
}

// strToUuid convert any given string to md5, and then to uuid, for
// example, a device address string "00:12:34:56:ab:cd" will be
// converted to "086e214c-1f20-bca4-9816-c0a11c8c0e02"
func strToUuid(str string) (uuid string) {
	md5, _ := utils.SumStrMd5(str)
	return doStrToUuid(md5)
}
func doStrToUuid(str string) (uuid string) {
	str = strings.ToLower(str)
	for i := 0; i < len(str); i++ {
		if (str[i] >= '0' && str[i] <= '9') ||
			(str[i] >= 'a' && str[i] <= 'f') {
			uuid = uuid + string(str[i])
		}
	}
	if len(uuid) < 32 {
		misslen := 32 - len(uuid)
		uuid = strings.Repeat("0", misslen) + uuid
	}
	uuid = fmt.Sprintf("%s-%s-%s-%s-%s", uuid[0:8], uuid[8:12], uuid[12:16], uuid[16:20], uuid[20:32])
	return
}

// execute program and read or write to it stdin/stdout pipe
func execWithIO(name string, arg ...string) (process *os.Process, stdin io.WriteCloser, stdout, stderr io.ReadCloser, err error) {
	cmd := exec.Command(name, arg...)
	stdin, _ = cmd.StdinPipe()
	stdout, _ = cmd.StdoutPipe()
	stderr, _ = cmd.StderrPipe()

	err = cmd.Start()
	if err != nil {
		return
	}
	go cmd.Wait()

	process = cmd.Process
	return
}

func isWirelessDeviceSupportHotspot(macAddress string) bool {
	devices, err := iw.ListWirelessInfo()
	if err != nil {
		logger.Warning("Failed to detect hotspot:", macAddress, err)
		return false
	}
	dev := devices.Get(macAddress)
	if dev == nil {
		logger.Warning("Failed to find device:", macAddress)
		return false
	}
	return dev.SupportedHotspot()
}

func getAutoConnectConnUuidListByConnType(connType string) ([]string, error) {
	var uuidSlice []string
	// get connections slice from settings
	connPaths, err := nmSettings.ListConnections(0)
	if err != nil {
		logger.Warningf("get network connections failed, err: %v", err)
		return nil, err
	}
	// get system bus
	systemBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning(err)
		return nil, err
	}
	// get uuid list from list according to type
	for _, connPath := range connPaths {
		conn, err := nmdbus.NewConnectionSettings(systemBus, connPath)
		if err != nil {
			logger.Warning(err)
			continue
		}
		settings, err := conn.GetSettings(0)
		if err != nil {
			logger.Warning(err)
			continue
		}
		// check if type is wanted
		if getSettingConnectionType(settings) != connType {
			continue
		}
		// check if is auto connect
		autoConnect := getSettingConnectionAutoconnect(settings)
		if !autoConnect {
			continue
		}
		// add uuid
		uuid := getSettingConnectionUuid(settings)
		if uuid != "" {
			uuidSlice = append(uuidSlice, uuid)
		}
	}
	return uuidSlice, nil
}

func isNetworkAvailable() (bool, error) {
	state, err := nmManager.State().Get(0)
	if err != nil {
		return false, err
	}
	return state >= nm.NM_STATE_CONNECTED_SITE, nil
}

func enableNetworking() error {
	enabled, err := nmManager.NetworkingEnabled().Get(0)
	if err != nil {
		return err
	}

	if enabled {
		return nil
	}

	return nmManager.Enable(0, true)
}

// check if need hotspot enable
func enableHotspot() bool {
	buf, err := ioutil.ReadFile("/var/lib/deepin/hotspot")
	if err != nil {
		logger.Debugf("enable hotspot read failed, err: %v",err)
		return false
	}
	// if enable is set
	if string(buf) == "enable" {
		return true
	}
	// if not
	return false
}
