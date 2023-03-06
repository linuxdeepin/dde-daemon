// Code generated by "dbusutil-gen em -type Daemon"; DO NOT EDIT.

package main

import (
	"github.com/linuxdeepin/go-lib/dbusutil"
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
			Name:   "DeleteCustomWallPaper",
			Fn:     v.DeleteCustomWallPaper,
			InArgs: []string{"username", "file"},
		},
		{
			Name:    "GetCustomWallPapers",
			Fn:      v.GetCustomWallPapers,
			InArgs:  []string{"username"},
			OutArgs: []string{"outArg0"},
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
			Name:    "SaveCustomWallPaper",
			Fn:      v.SaveCustomWallPaper,
			InArgs:  []string{"username", "file"},
			OutArgs: []string{"outArg0"},
		},
		{
			Name:   "ScalePlymouth",
			Fn:     v.ScalePlymouth,
			InArgs: []string{"scale"},
		},
		{
			Name:   "SetLogindTTY",
			Fn:     v.SetLogindTTY,
			InArgs: []string{"NAutoVTs", "resetCustom", "live"},
		},
		{
			Name:   "SetLongPressDuration",
			Fn:     v.SetLongPressDuration,
			InArgs: []string{"duration"},
		},
	}
}
