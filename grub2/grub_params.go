// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
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

	"github.com/linuxdeepin/dde-daemon/grub_common"
)

const (
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

func genGrubParamsContent(params map[string]string) []byte {
	// copy to avoid modifying the original map
	paramsCopy := make(map[string]string, len(params))
	for k, v := range params {
		paramsCopy[k] = v
	}

	// Ensure the following critical settings can always be overridden by the latest configuration
	for _, key := range []string{
		grub_common.DeepinGfxmodeDetect,
		grub_common.DeepinGfxmodeAdjusted,
		grub_common.DeepinGfxmodeNotSupported,
	} {
		if _, ok := paramsCopy[key]; !ok {
			paramsCopy[key] = ""
		}
	}
	// Only the following settings are allowed in /etc/default/11_dde.cfg; all others will be removed
	allowedKeys := map[string]struct{}{
		grub_common.DeepinGfxmodeDetect:       {},
		grub_common.DeepinGfxmodeAdjusted:     {},
		grub_common.DeepinGfxmodeNotSupported: {},
		grubBackground: {},
		grubDefault:    {},
		grubGfxmode:    {},
		grubTheme:      {},
		grubTimeout:    {},
	}
	for key := range paramsCopy {
		if _, ok := allowedKeys[key]; !ok {
			delete(paramsCopy, key)
		}
	}

	keys := make(sort.StringSlice, 0, len(paramsCopy))
	for k := range paramsCopy {
		keys = append(keys, k)
	}
	keys.Sort()

	// write buf
	var buf bytes.Buffer
	buf.WriteString("# Written by " + dbusServiceName + "\n")
	for _, k := range keys {
		buf.WriteString(k + "=" + paramsCopy[k] + "\n")
	}
	// if you want let the update-grub exit with error code,
	// uncomment the next line.
	//buf.WriteString("=\n")
	return buf.Bytes()
}

func writeGrubParams(params map[string]string) error {
	logger.Debug("write grub params")
	content := genGrubParamsContent(params)

	err := os.WriteFile(grub_common.DDEGrubParamsFile, content, 0644)
	if err != nil {
		return err
	}

	return nil
}
