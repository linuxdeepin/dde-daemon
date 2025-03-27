// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"encoding/gob"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

var (
	// v1 ~ v4 旧版本配置文件，~/.config/deepin/startdde/display.json
	configFile string
	// v5 v6 版本配置文件， ~/.config/deepin/startdde/display_v5.json
	configFileV5 string
	// ~/.config/deepin/startdde/config.version
	// 之前最新版本号 5.0
	configVersionFile string
	// 用户级别配置文件 ~/.config/deepin/startdde/display-user.json
	userConfigFile string
)

func init() {
	cfgDir := getCfgDir()
	configFile = filepath.Join(cfgDir, "display.json")
	configFileV5 = filepath.Join(cfgDir, "display_v5.json")
	configVersionFile = filepath.Join(cfgDir, "config.version")
	userConfigFile = filepath.Join(cfgDir, "display-user.json")
}

func getCfgDir() string {
	return filepath.Join(basedir.GetUserConfigDir(), "deepin/startdde")
}

func updateSysMonitorConfigsName(configs SysMonitorConfigs, monitorMap map[uint32]*Monitor) {
	for _, mc := range configs {
		for _, m := range monitorMap {
			if mc.UUID == m.uuid {
				mc.Name = m.Name
				break
			}
		}
	}
}

func loadBuiltinMonitorConfig(filename string) (string, error) {
	// #nosec G304
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

type ConnectInfo struct {
	Connects           map[string]bool
	LastConnectedTimes map[string]time.Time
}

func readConnectInfoCache(file string) (*ConnectInfo, error) {
	// #nosec G304
	fp, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = fp.Close()
	}()

	var info ConnectInfo
	decoder := gob.NewDecoder(fp)
	err = decoder.Decode(&info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// 加载 v5 或 v6 配置，v6 优先
func loadConfigV5V6(filename string) (*ConfigV6, error) {
	// #nosec G304
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var c ConfigV6
	err = json.Unmarshal(data, &c)
	if err != nil {
		logger.Warning(err)
	}

	// 以 v6 配置格式解析失败，尝试以 v5 配置格式解析。
	if c.ConfigV5 == nil {
		err = json.Unmarshal(data, &c.ConfigV5)
		if err != nil {
			return nil, err
		}
	}
	return &c, nil
}

// 加载旧配置文件,可能影响 m.DisplayMode 和系统配置中的 DisplayMode。
func loadOldConfig(m *Manager) (*ConfigV6, error) {
	var configV5 ConfigV5
	cfgVer, err := getConfigVersion(configVersionFile)
	if err == nil {
		if cfgVer == "3.3" {
			// 3.3配置文件转换
			cfg0, err := loadConfigV3D3(configFile)
			if err == nil {
				configV5 = cfg0.toConfig(m)
			} else if !os.IsNotExist(err) {
				logger.Warning(err)
			}
		} else if cfgVer == "4.0" {
			// 4.0配置文件转换
			cfg0, err := loadConfigV4(configFile)
			if err == nil {
				configV5 = cfg0.toConfig(m)
			} else if !os.IsNotExist(err) {
				logger.Warning(err)
			}
		}
	} else if !os.IsNotExist(err) {
		logger.Warning(err)
	}

	if len(configV5) > 0 {
		// 加载 v5 之前配置文件成功
		configV6 := &ConfigV6{ConfigV5: configV5}
		return configV6, nil
	}
	// NOTE: v5 到 v6 并没有更新 configVersionFile 的内容, 也使用同一个配置文件。
	configV6, err := loadConfigV5V6(configFileV5)
	if err != nil {
		// 加载 v5 和 v6 配置文件都失败
		return nil, err
	}
	// 加载 v5 或 v6 配置文件成功
	return configV6, nil
}

func (m *Manager) saveBuiltinMonitorConfig(name string) (err error) {
	m.sysConfig.mu.Lock()

	m.sysConfig.Config.Cache.BuiltinMonitor = name
	err = m.saveSysConfigNoLock("builtin monitor")

	m.sysConfig.mu.Unlock()
	return
}
