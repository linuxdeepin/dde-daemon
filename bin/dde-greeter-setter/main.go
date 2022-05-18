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

package main

import (
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("GreeterSetter")

func main() {
	service, err := dbusutil.NewSystemService()
	if err != nil {
		logger.Errorf("failed to new system service")
		return
	}

	var m = &Manager{
		service: service,
	}

	// v20
	err = service.ExportExt(dbusPathV20, dbusInterfaceV20, m)
	if err != nil {
		logger.Error("failed to export:", err)
		return
	}
	err = service.RequestName(dbusServiceNameV20)
	if err != nil {
		logger.Error("failed to request name:", err)
		return
	}
	// v23
	err = service.ExportExt(dbusPathV23, dbusInterfaceV23, m)
	if err != nil {
		logger.Error("failed to export:", err)
		return
	}
	err = service.RequestName(dbusServiceNameV23)
	if err != nil {
		logger.Error("failed to request name:", err)
		return
	}

	service.SetAutoQuitHandler(time.Second*30, nil)
	service.Wait()
}
