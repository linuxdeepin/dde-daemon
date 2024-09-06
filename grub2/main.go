// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub2

import (
	"os"
	"os/exec"
	"time"

	"github.com/linuxdeepin/dde-api/inhibit_hint"
	"github.com/linuxdeepin/dde-daemon/grub_common"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

var _g *Grub2

func RunAsDaemon() {
	allowNoCheckAuth()
	service, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Fatal("failed to new system service", err)
	}
	_g = NewGrub2(service)
	ihObj := inhibit_hint.New("lastore-daemon")
	ihObj.SetName("Control Center")
	ihObj.SetIcon("preferences-system")

	err = service.Export(dbusPath, _g)
	if err != nil {
		logger.Fatal("failed to export grub2:", err)
	}

	err = service.Export(themeDBusPath, _g.theme)
	if err != nil {
		logger.Fatal("failed to export grub2 theme:", err)
	}

	err = service.Export(editAuthDBusPath, _g.editAuth)
	if err != nil {
		logger.Fatal("failed to export grub2 edit auth:", err)
	}

	// err = ihObj.Export(service)
	// if err != nil {
	// 	logger.Warning("failed to export inhibit hint:", err)
	// }

	err = service.Export(dbusInhibitorPath, ihObj)
	if err != nil {
		logger.Warning("failed to export inhibit hint:", err)
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		logger.Fatal("failed to request name:", err)
	}

	service.SetAutoQuitHandler(5*time.Minute, _g.canSafelyExit)
	service.Wait()
}

func PrepareGfxmodeDetect() error {
	params, err := grub_common.LoadGrubParams()
	if err != nil {
		logger.Warning(err)
	}

	gfxmodes, err := grub_common.GetGfxmodesFromXRandr()
	if err != nil {
		logger.Debug("failed to gfxmodes from XRandr:", err)
	}
	gfxmodes.SortDesc()
	logger.Debug("gfxmodes:", gfxmodes)
	gfxmodesStr := joinGfxmodesForDetect(gfxmodes)
	getModifyFuncPrepareGfxmodeDetect(gfxmodesStr)(params)

	err = os.WriteFile(grub_common.GfxmodeDetectReadyPath, nil, 0644)
	if err != nil {
		return err
	}

	err = writeGrubParams(params)
	if err != nil {
		return err
	}

	cmd := exec.Command(adjustThemeCmd, "-fallback-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		logger.Warning("failed to adjust theme:", err)
	}

	return nil
}

func GetOSNum() (uint32, error) {
	fileContent, err := os.ReadFile(grubScriptFile)
	if err != nil {
		logger.Error(err)
		return 0, err
	}
	entries, err := parseEntries(string(fileContent))
	if err != nil {
		logger.Error(err)
		return 0, err
	}

	return getOSNum(entries), nil
}
