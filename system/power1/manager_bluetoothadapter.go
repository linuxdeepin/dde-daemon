// SPDX-FileCopyrightText: 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later
package power
import (
	"os/exec"
	"regexp"
	"strings"
)

func getDevices() (devices []string) {
	re := regexp.MustCompile(`hci\d+`)
	out, err := exec.Command("hciconfig", "-a").CombinedOutput()
	if err != nil {
		logger.Warning(err)
		return
	}
	var dataLines = strings.Split(string(out), "\n")
	if len(dataLines) == 0 {
		logger.Warning("there is no device")
		return
	}
	for line := range dataLines {
		matches := re.FindStringSubmatch(dataLines[line])
		if len(matches) != 0 {
			devices = append(devices, matches[0])
		}
	}
	return
}

func setHciconfig(cmd string) {
	devices := getDevices()
	for index := range devices {
		_, err := exec.Command("hciconfig", devices[index], cmd).CombinedOutput()
		if err != nil {
			logger.Warning(err)
		}
	}
}
