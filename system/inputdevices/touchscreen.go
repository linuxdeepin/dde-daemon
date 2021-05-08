/*
 * Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:     zsien <i@zsien.cn>
 *
 * Maintainer: zsien <i@zsien.cn>
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
package inputdevices

import (
	"github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/utils"
)

type Touchscreen struct {
	service *dbusutil.Service

	id         uint32
	UUID       string
	Name       string
	Phys       string
	DevNode    string
	BusType    string
	Serial     string
	OutputName string
	Width      float64
	Height     float64
}

const touchscreenDBusPath = "/com/deepin/system/InputDevices/Touchscreen/"
const touchscreenDBusInterface = "com.deepin.system.InputDevices.Touchscreen"

func newTouchscreen(dev *libinputDevice, service *dbusutil.Service, id uint32) *Touchscreen {
	t := &Touchscreen{
		service: service,

		id:         id,
		Name:       dev.GetName(),
		Phys:       dev.GetPhys(),
		DevNode:    dev.GetDevNode(),
		BusType:    dev.GetProperty("ID_BUS"),
		Serial:     dev.GetProperty("ID_SERIAL"),
		OutputName: dev.GetOutputName(),
	}

	t.UUID = genTouchscreenUUID(t.Phys, t.Serial)
	t.Width, t.Height = dev.GetSize()

	return t
}

func (t *Touchscreen) GetInterfaceName() string {
	return touchscreenDBusInterface
}

func (t *Touchscreen) export(path dbus.ObjectPath) error {
	err := t.service.Export(path, t)
	if err != nil {
		logger.Warning(err)
		return err
	}

	return nil
}

func (t *Touchscreen) stopExport() error {
	t.service.StopExport(t)
	return nil
}

func genTouchscreenUUID(phys, serial string) string {
	res, _ := utils.SumStrMd5(phys + serial)
	return res
}
