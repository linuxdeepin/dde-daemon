/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package grub2

import (
	"io/ioutil"
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

	err = service.ExportExt(dbusPathV20, dbusInterfaceV20, _g)
	if err != nil {
		logger.Fatal("failed to export grub2:", err)
	}

	err = service.ExportExt(dbusPathV23, dbusInterfaceV23, _g)
	if err != nil {
		logger.Fatal("failed to export grub2:", err)
	}

	err = service.ExportExt(themeDBusPathV20, themeDBusInterfaceV20, _g.theme)
	if err != nil {
		logger.Fatal("failed to export grub2 theme:", err)
	}

	err = service.ExportExt(themeDBusPathV23, themeDBusInterfaceV23, _g.theme)
	if err != nil {
		logger.Fatal("failed to export grub2 theme:", err)
	}

	err = service.ExportExt(editAuthDBusPathV20, editAuthDBusInterfaceV20, _g.editAuth)
	if err != nil {
		logger.Fatal("failed to export grub2 edit auth:", err)
	}

	err = service.ExportExt(editAuthDBusPathV23, editAuthDBusInterfaceV23, _g.editAuth)
	if err != nil {
		logger.Fatal("failed to export grub2 edit auth:", err)
	}

	// err = ihObj.Export(service)
	// if err != nil {
	// 	logger.Warning("failed to export inhibit hint:", err)
	// }

	err = service.ExportExt(dbusInhibitorPathV20, dbusInhibitorInterfaceV20, ihObj)
	if err != nil {
		logger.Warning("failed to export inhibit hint:", err)
	}

	err = service.ExportExt(dbusInhibitorPathV23, dbusInhibitorInterfaceV23, ihObj)
	if err != nil {
		logger.Warning("failed to export inhibit hint:", err)
	}

	err = service.RequestName(dbusServiceNameV20)
	if err != nil {
		logger.Fatal("failed to request name:", err)
	}

	err = service.RequestName(dbusServiceNameV23)
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

	err = ioutil.WriteFile(grub_common.GfxmodeDetectReadyPath, nil, 0644)
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
	fileContent, err := ioutil.ReadFile(grubScriptFile)
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
