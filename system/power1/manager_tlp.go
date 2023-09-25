// SPDX-FileCopyrightText: 2018 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"bufio"
	"errors"
	"github.com/godbus/dbus/v5"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/utils"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const lowBatteryThreshold = 20.0

// tlp config file
const tlpConfigFile = "/etc/tlp.d/50-tlp.conf"
const tlpConfigFileOder = "/etc/default/tlp"
const tlpBin = "/usr/sbin/tlp"

const (
	tlpConfigAuto     = 1
	tlpConfigEnabled  = 2
	tlpConfigDisabled = 3
)

func isTlpBinOk() bool {
	_, err := exec.LookPath("tlp")
	if err == nil {
		return true
	}
	return utils.IsFileExist(tlpBin)
}

func setTlpConfig(key string, val string) {
	path := tlpConfigPath()
	lines, err := loadTlpConfig(path)
	if err != nil {
		logger.Warning(err)
		return
	}

	dict := make(map[string]string)
	dict[key] = val
	lines, changed := modifyTlpConfig(lines, dict)
	if changed {
		logger.Debug("write tlp Config")
		err = writeTlpConfig(lines, path)
		if err != nil {
			logger.Warning(err)
		}
	}
}

func setTlpConfigMode(mode int) (changed bool, err error) {
	path := tlpConfigPath()
	lines, err := loadTlpConfig(path)
	if err != nil {
		logger.Warning(err)
		// ignore not exist error
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	dict := make(map[string]string)
	switch mode {
	case tlpConfigAuto:
		dict["TLP_ENABLE"] = "1"
		dict["TLP_PERSISTENT_DEFAULT"] = "0"
	case tlpConfigEnabled:
		dict["TLP_ENABLE"] = "1"
		dict["TLP_DEFAULT_MODE"] = "BAT"
		dict["TLP_PERSISTENT_DEFAULT"] = "1"
	case tlpConfigDisabled:
		dict["TLP_ENABLE"] = "1"
		dict["TLP_DEFAULT_MODE"] = "AC"
		dict["TLP_PERSISTENT_DEFAULT"] = "1"
	}
	lines, changed = modifyTlpConfig(lines, dict)
	if changed {
		logger.Debug("write tlp Config")
		err = writeTlpConfig(lines, path)
		if err != nil {
			return false, err
		}
	}

	return changed, nil
}

func reloadTlpService() error {
	if !isTlpBinOk() {
		return errors.New("tlp is not installed")
	}

	systemBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	sysManager := systemd1.NewManager(systemBus)
	_, err = sysManager.ReloadUnit(dbus.FlagNoAutoStart, "tlp.service", "replace")
	return err
}

func modifyTlpConfig(lines []string, dict map[string]string) ([]string, bool) {
	var changed bool
	for idx := range lines {
		line := lines[idx]
		line = strings.TrimPrefix(line, "#")
		for key, value := range dict {
			if strings.HasPrefix(line, key) {
				newLine := key + "=" + value
				if line != newLine {
					changed = true
					lines[idx] = newLine
				}
				delete(dict, key)
			}
		}
		if len(dict) == 0 {
			break
		}
	}
	if len(dict) > 0 {
		for key, value := range dict {
			newLine := key + "=" + value
			lines = append(lines, newLine)
		}
		changed = true
	}
	return lines, changed
}

func tlpConfigPath() (configFile string) {
	if !isTlpBinOk() {
		logger.Warning("tlp is not installed")
		return
	}
	if utils.IsFileExist(tlpConfigFileOder) {
		return tlpConfigFileOder
	} else {
		return tlpConfigFile
	}
}

func loadTlpConfig(path string) ([]string, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	defer f.Close()
	if err != nil {
		return nil, err
	}

	var lines []string
	scanner := bufio.NewScanner(bufio.NewReader(f))
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}
	if scanner.Err() != nil {
		return nil, scanner.Err()
	}

	if len(lines) == 0 {
		lines = append(lines, "# Generate by dde-system-daemon")
	}

	return lines, nil
}

func writeTlpConfig(lines []string, path string) error {
	tempFile, err := writeTlpConfigTemp(lines, path)
	if err != nil {
		if tempFile != "" {
			os.Remove(tempFile)
		}
		return err
	}
	return os.Rename(tempFile, path)
}

func writeTlpConfigTemp(lines []string, path string) (string, error) {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}

	f, err := os.CreateTemp(dir, "tlp.conf")

	defer f.Close()
	logger.Debug("writeTlpConfig temp file", f.Name())
	if err != nil {
		return "", err
	}

	err = f.Chmod(0644)
	if err != nil {
		return f.Name(), err
	}

	bufWriter := bufio.NewWriter(f)
	for _, line := range lines {
		_, err := bufWriter.WriteString(line)
		if err != nil {
			logger.Warning(err)
		}
		err = bufWriter.WriteByte('\n')
		if err != nil {
			logger.Warning(err)
		}
	}
	return f.Name(), bufWriter.Flush()
}
