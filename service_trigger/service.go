/*
 *  Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:
 *
 * Maintainer:
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

package service_trigger

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus"
)

type ServiceMonitor struct {
	Type string
	DBus *ServiceMonitorDBus
}

// ServiceMonitorDBus type DBus
type ServiceMonitorDBus struct {
	BusType   string // System or Session
	Sender    string
	Interface string
	Signal    string
	Path      string // optional
}

type Service struct {
	filename string
	basename string
	Monitor  ServiceMonitor

	Name        string
	Description string
	Exec        []string
	execFn      func(signal *dbus.Signal)
}

func (service *Service) getDBusMatchRule() string {
	dbusField := service.Monitor.DBus

	rule := "type='signal'"
	rule += fmt.Sprintf(",sender='%s'", dbusField.Sender)
	rule += fmt.Sprintf(",interface='%s'", dbusField.Interface)
	rule += fmt.Sprintf(",member='%s'", dbusField.Signal)

	if dbusField.Path != "" {
		rule += fmt.Sprintf(",path='%s'", dbusField.Path)
	}
	return rule
}

func (service *Service) check() error {
	if service.Monitor.Type != "DBus" {
		return fmt.Errorf("unknown Monitor.Type %q" + service.Monitor.Type)
	}

	if service.Monitor.Type == "DBus" {
		err := service.checkDBus()
		if err != nil {
			return err
		}
	}

	if service.Name == "" {
		return errors.New("field Name is empty")
	}

	if len(service.Exec) == 0 {
		return errors.New("field Exec is empty")
	}

	return nil
}

func (service *Service) checkDBus() error {
	dbusField := service.Monitor.DBus
	if dbusField == nil {
		return errors.New("field Monitor.DBus is nil")
	}

	if !(dbusField.BusType == "System" ||
		dbusField.BusType == "Session") {
		return errors.New("field Monitor.DBus.BusType is invalid")
	}

	if dbusField.Path != "" {
		if !dbus.ObjectPath(dbusField.Path).IsValid() {
			return errors.New("field Monitor.DBus.Path is invalid")
		}
	}
	if dbusField.Sender == "" {
		return errors.New("field Monitor.DBus.Sender is empty")
	}

	if dbusField.Interface == "" {
		return errors.New("field Monitor.DBus.Interface is empty")
	}

	if dbusField.Signal == "" {
		return errors.New("field Monitor.DBus.Signal is empty")
	}
	return nil
}

func loadService(filename string) (*Service, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var service Service
	err = json.Unmarshal(data, &service)
	if err != nil {
		return nil, err
	}

	err = service.check()
	if err != nil {
		return nil, err
	}

	service.filename = filename
	service.basename = strings.TrimSuffix(filepath.Base(filename), serviceFileExt)
	return &service, nil
}

func (service *Service) String() string {
	if service == nil {
		return "<Service nil>"
	}
	return fmt.Sprintf("<Service %s %q>", service.basename, service.Name)
}
