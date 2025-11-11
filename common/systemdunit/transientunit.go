// SPDX-FileCopyrightText: 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package systemdunit

import (
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

type TransientUnit struct {
	Dbus        *dbus.Conn
	Commands    []string
	UnitName    string
	Type        string
	Description string
	Environment []string
	unit        systemd1.Unit
}

type execStart struct {
	Path             string   // the binary path to execute
	Args             []string // an array with all arguments to pass to the executed command, starting with argument 0
	UncleanIsFailure bool     // a boolean whether it should be considered a failure if the process exits uncleanly
}

func CheckUnitExist(conn *dbus.Conn, name string) bool {
	systemd := systemd1.NewManager(conn)
	_, err := systemd.GetUnit(0, name)
	if err != nil {
		return false
	}
	return true
}

func (t *TransientUnit) StartTransientUnit() error {
	systemd := systemd1.NewManager(t.Dbus)
	if CheckUnitExist(t.Dbus, t.UnitName) {
		err := systemd.ResetFailedUnit(0, t.UnitName)
		if err != nil {
			return fmt.Errorf("failed to reset failed unit: %v", err)
		}
	}

	var properties []systemd1.Property
	var aux []systemd1.PropertyCollection
	properties = append(properties, systemd1.Property{"Type", dbus.MakeVariant(t.Type)})
	properties = append(properties, systemd1.Property{"Description", dbus.MakeVariant(t.Description)})
	if len(t.Environment) > 0 {
		properties = append(properties, systemd1.Property{"Environment", dbus.MakeVariant(t.Environment)})
	}
	properties = append(properties, systemd1.Property{"ExecStart", dbus.MakeVariant([]execStart{{
		Path:             t.Commands[0],
		Args:             t.Commands,
		UncleanIsFailure: false,
	},
	})})
	_, err := systemd.StartTransientUnit(0, t.UnitName, "replace", properties, aux)
	if err != nil {
		return fmt.Errorf("failed to start transient unit: %v", err)
	}
	unitPath, err := systemd.GetUnit(0, t.UnitName)
	if err != nil {
		return fmt.Errorf("failed to get unitpath: %v", err)
	}
	unit, err := systemd1.NewUnit(t.Dbus, unitPath)
	if err != nil {
		return fmt.Errorf("failed to unit: %v", err)
	}
	t.unit = unit
	return nil
}

func (t *TransientUnit) WaitforFinish(sigLoop *dbusutil.SignalLoop) bool {
	if t.unit == nil {
		return false
	}
	t.unit.InitSignalExt(sigLoop, true)
	var result = make(chan string)
	t.unit.ConnectPropertiesChanged(func(interfaceName string, changedProperties map[string]dbus.Variant, invalidatedProperties []string) {
		_, ok := changedProperties["ActiveState"]
		if ok {
			val := changedProperties["ActiveState"].String()
			result <- val
		}
	})
	defer func() {
		t.unit.RemoveAllHandlers()
		close(result)
	}()
	for {
		res := <-result
		if strings.Contains(res, "failed") {
			return false
		} else if strings.Contains(res, "inactive") {
			return true
		}
	}
}
