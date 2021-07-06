/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     weizhixiang <weizhixiang@uniontech.com>
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

package bluetooth

import (
	"os"
	"os/exec"
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
)

const (
	dbusServiceName = "com.deepin.daemon.Bluetooth"
	dbusPath        = "/com/deepin/daemon/Bluetooth"
	dbusInterface   = "com.deepin.daemon.Bluetooth"

	bluetoothInitScript = "/usr/share/dde-daemon/bluetooth/bluetooth_init.sh"
)

var (
	bluetoothConfig   = os.Getenv("HOME") + "/.config/deepin/bluetooth.json"
)

type Bluetooth struct {
	service 		*dbusutil.Service
}

func newBluetooth(service *dbusutil.Service) *Bluetooth {
	return &Bluetooth{
		service: service,
	}
}

func (b *Bluetooth) bluetoothInit() {
	var handle = "up"
	_, err := os.Stat(bluetoothConfig)
	if err != nil {
		if os.IsNotExist(err) {
			handle = "down"
		}
	}

	err = exec.Command("/bin/sh",  bluetoothInitScript, handle).Run()
	if err != nil {
		logger.Errorf("init bluetooth %s err: %s\n", handle, err)
	}
}

func (d *Bluetooth) GetInterfaceName() string {
	return dbusInterface
}

type Daemon struct {
	*loader.ModuleBase
}

var (
	_m     *Bluetooth
	logger = log.NewLogger(dbusServiceName)
)

func init() {
	loader.Register(NewDaemon())
}

func NewDaemon() *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("bluetooth", daemon, logger)
	return daemon
}

func (d *Daemon) GetDependencies() []string {
	return []string{}
}

func (d *Daemon) Start() error {
	logger.BeginTracing()
	logger.Info("start bluetooth daemon")
	service := loader.GetService()
	_m = newBluetooth(service)
	err := service.Export(dbusPath, _m)
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	go _m.bluetoothInit()

	return nil
}

func (d *Daemon) Stop() error {
	logger.Info("stop bluetooth daemon")
	if _m == nil {
		return nil
	}

	service := loader.GetService()
	err := service.StopExport(_m)
	if err != nil {
		return err
	}

	_m = nil
	return nil
}
