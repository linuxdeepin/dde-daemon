// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub2

import (
	"bytes"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

const (
	grubParamsFile     = "/etc/default/grub.d/11_dde.cfg"
	themesDir          = "/boot/grub/themes"
	themesTmpDir       = themesDir + ".tmp"
	defaultThemeDir    = themesDir + "/deepin"
	defaultThemeTmpDir = themesTmpDir + "/deepin"
	fallbackThemeDir   = defaultThemeDir + "-fallback"

	grubBackground = "GRUB_BACKGROUND"
	grubDefault    = "GRUB_DEFAULT"
	grubGfxmode    = "GRUB_GFXMODE"
	grubTheme      = "GRUB_THEME"
	grubTimeout    = "GRUB_TIMEOUT"

	defaultGrubTheme       = defaultThemeDir + "/theme.txt"
	fallbackGrubTheme      = fallbackThemeDir + "/theme.txt"
	defaultGrubBackground  = defaultThemeDir + "/background.jpg"
	fallbackGrubBackground = fallbackThemeDir + "/background.jpg"
	defaultGrubDefault     = "0"
	defaultGrubDefaultInt  = 0
	defaultGrubGfxMode     = "auto"
	defaultGrubTimeoutInt  = 5
)

func decodeShellValue(in string) string {
	output, err := exec.Command("/bin/sh", "-c", "echo -n "+in).Output()
	if err != nil {
		// fallback
		return strings.Trim(in, "\"")
	}
	return string(output)
}

func getTimeout(params map[string]string) int {
	timeoutStr := decodeShellValue(params[grubTimeout])
	timeoutInt, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return defaultGrubTimeoutInt
	}
	return timeoutInt
}

func getGfxMode(params map[string]string) (val string) {
	val = decodeShellValue(params[grubGfxmode])
	if val == "" {
		val = defaultGrubGfxMode
	}
	return
}

func getDefaultEntry(params map[string]string) (val string) {
	val = decodeShellValue(params[grubDefault])
	if val == "" {
		val = defaultGrubDefault
	}
	return
}

func getTheme(params map[string]string) string {
	return decodeShellValue(params[grubTheme])
}

func getGrubParamsContent(params map[string]string) []byte {
	keys := make(sort.StringSlice, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	keys.Sort()

	// write buf
	var buf bytes.Buffer
	buf.WriteString("# Written by " + dbusServiceName + "\n")
	for _, k := range keys {
		buf.WriteString(k + "=" + params[k] + "\n")
	}
	// if you want let the update-grub exit with error code,
	// uncomment the next line.
	//buf.WriteString("=\n")
	return buf.Bytes()
}

func writeGrubParams(params map[string]string) error {
	logger.Debug("write grub params")
	content := getGrubParamsContent(params)

	err := os.WriteFile(grubParamsFile, content, 0644)
	if err != nil {
		return err
	}

	return nil
}
