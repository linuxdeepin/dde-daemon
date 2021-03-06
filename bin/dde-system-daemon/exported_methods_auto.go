// Code generated by "dbusutil-gen em -type Daemon"; DO NOT EDIT.

package main

import (
	"pkg.deepin.io/lib/dbusutil"
)

func (v *Daemon) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name:    "BluetoothGetDeviceTechnologies",
			Fn:      v.BluetoothGetDeviceTechnologies,
			InArgs:  []string{"adapter", "device"},
			OutArgs: []string{"technologies"},
		},
		{
			Name:   "ClearTty",
			Fn:     v.ClearTty,
			InArgs: []string{"number"},
		},
		{
			Name: "ClearTtys",
			Fn:   v.ClearTtys,
		},
		{
			Name:    "IsPidVirtualMachine",
			Fn:      v.IsPidVirtualMachine,
			InArgs:  []string{"pid"},
			OutArgs: []string{"isVM"},
		},
		{
			Name:    "NetworkGetConnections",
			Fn:      v.NetworkGetConnections,
			OutArgs: []string{"data"},
		},
		{
			Name:   "NetworkSetConnections",
			Fn:     v.NetworkSetConnections,
			InArgs: []string{"data"},
		},
		{
			Name:   "ScalePlymouth",
			Fn:     v.ScalePlymouth,
			InArgs: []string{"scale"},
		},
		{
			Name:   "SetLongPressDuration",
			Fn:     v.SetLongPressDuration,
			InArgs: []string{"duration"},
		},
	}
}
