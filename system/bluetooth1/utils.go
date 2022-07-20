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

package bluetooth

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/linuxdeepin/go-lib/keyfile"
)

func isStringInArray(str string, list []string) bool {
	for _, tmp := range list {
		if tmp == str {
			return true
		}
	}
	return false
}

func marshalJSON(v interface{}) (strJSON string) {
	byteJSON, err := json.Marshal(v)
	if err != nil {
		logger.Error(err)
		return
	}
	strJSON = string(byteJSON)
	return
}

// readBoolFile 读取一个很小的文件，一般情况下内容只有 0 和 1。
func readBoolFile(filename string) (bool, error) {
	// #nosec G304
	f, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = f.Close()
	}()
	var buf [4]byte
	n, err := f.Read(buf[:])
	if err != nil {
		return false, err
	}
	content := bytes.TrimSpace(buf[:n])
	// 文件内容应该是 0 或 1
	if bytes.Equal(content, []byte("0")) {
		return false, nil
	}
	return true, nil
}

const (
	bluetoothPrefixDir = "/var/lib/bluetooth"
	kfSectionGeneral   = "General"
	kfKeyTechnologies  = "SupportedTechnologies"
)

func doGetDeviceTechnologies(filename string) ([]string, error) {
	var kf = keyfile.NewKeyFile()
	err := kf.LoadFromFile(filename)
	if err != nil {
		return nil, err
	}
	return kf.GetStringList(kfSectionGeneral, kfKeyTechnologies)
}
