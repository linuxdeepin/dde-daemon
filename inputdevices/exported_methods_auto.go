// Code generated by "dbusutil-gen em -type Keyboard,Mouse,Touchpad,TrackPoint,Wacom,Manager"; DO NOT EDIT.

package inputdevices

import (
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (v *Keyboard) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name:   "AddLayoutOption",
			Fn:     v.AddLayoutOption,
			InArgs: []string{"option"},
		},
		{
			Name:   "AddUserLayout",
			Fn:     v.AddUserLayout,
			InArgs: []string{"layout"},
		},
		{
			Name: "ClearLayoutOption",
			Fn:   v.ClearLayoutOption,
		},
		{
			Name:   "DeleteLayoutOption",
			Fn:     v.DeleteLayoutOption,
			InArgs: []string{"option"},
		},
		{
			Name:   "DeleteUserLayout",
			Fn:     v.DeleteUserLayout,
			InArgs: []string{"layout"},
		},
		{
			Name:    "GetLayoutDesc",
			Fn:      v.GetLayoutDesc,
			InArgs:  []string{"layout"},
			OutArgs: []string{"outArg0"},
		},
		{
			Name:    "LayoutList",
			Fn:      v.LayoutList,
			OutArgs: []string{"outArg0"},
		},
		{
			Name: "Reset",
			Fn:   v.Reset,
		},
		{
			Name: "ToggleNextLayout",
			Fn:   v.ToggleNextLayout,
		},
	}
}
func (v *Manager) GetExportedMethods() dbusutil.ExportedMethods {
	return nil
}
func (v *Mouse) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name: "Reset",
			Fn:   v.Reset,
		},
	}
}
func (v *Touchpad) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name: "Reset",
			Fn:   v.Reset,
		},
	}
}
func (v *TrackPoint) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name: "Reset",
			Fn:   v.Reset,
		},
	}
}
func (v *Wacom) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name: "Reset",
			Fn:   v.Reset,
		},
	}
}
