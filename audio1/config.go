// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

var (
	fileLocker  sync.Mutex
	configCache *config
	configFile  = filepath.Join(basedir.GetUserConfigDir(), "deepin/dde-daemon/audio.json")
)

type config struct {
	Profiles   map[string]string // Profiles[cardName] = activeProfile
	Sink       string
	Source     string
	SinkPort   string
	SourcePort string

	SinkVolume   float64
	SourceVolume float64
}

func (c *config) string() string {
	data, _ := json.Marshal(c)
	return string(data)
}

func (c *config) equal(b *config) bool {
	if c == nil && b == nil {
		return true
	}
	if c == nil || b == nil {
		return false
	}
	return c.Sink == b.Sink &&
		c.Source == b.Source &&
		c.SinkPort == b.SinkPort &&
		c.SourcePort == b.SourcePort &&
		c.SinkVolume == b.SinkVolume &&
		c.SourceVolume == b.SourceVolume &&
		mapStrStrEqual(c.Profiles, b.Profiles)
}

func readConfig() (*config, error) {
	fileLocker.Lock()
	defer fileLocker.Unlock()

	if configCache != nil {
		return configCache, nil
	}

	var info config
	content, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(content, &info)
	if err != nil {
		return nil, err
	}

	configCache = &info
	return configCache, nil
}

func saveConfig(info *config) error {
	fileLocker.Lock()
	defer fileLocker.Unlock()

	if configCache.equal(info) {
		logger.Debug("[saveConfigInfo] config info not changed")
		return nil
	} else {
		logger.Debug("[saveConfigInfo] will save:", info.string())
	}

	content, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(configFile), 0755)
	if err != nil {
		return err
	}
	err = os.WriteFile(configFile, content, 0644)
	if err != nil {
		return err
	}

	configCache = info
	return nil
}

func removeConfig() {
	fileLocker.Lock()
	defer fileLocker.Unlock()

	err := os.Remove(configFile)
	if err != nil && !os.IsNotExist(err) {
		logger.Warning(err)
	}

	err = os.Remove(globalConfigKeeperFile)
	if err != nil && !os.IsNotExist(err) {
		logger.Warning(err)
	}

	err = os.Remove(globalPrioritiesFilePath)
	if err != nil && !os.IsNotExist(err) {
		logger.Warning(err)
	}
}

func mapStrStrEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		if w, ok := b[k]; !ok || v != w {
			return false
		}
	}
	return true
}
