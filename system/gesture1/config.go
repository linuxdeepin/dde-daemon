// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package gesture1

import (
	"encoding/json"
	"io/ioutil"

	"github.com/linuxdeepin/go-lib/utils"
)

type Config struct {
	LongPressDistance float64 `json:"longpress_distance"`
	Verbose           int     `json:"verbose"`
}

func loadConfig(filename string) (*Config, error) {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var conf Config
	err = json.Unmarshal(contents, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

func getConfigPath() string {
	suffix := "dde-daemon/gesture/conf.json"
	filename := "/etc/" + suffix
	if utils.IsFileExist(filename) {
		return filename
	}
	return "/usr/share/" + suffix
}
