// Code generated by "dbusutil-gen em -type Manager"; DO NOT EDIT.

package gesture1

import (
	"github.com/linuxdeepin/go-lib/dbusutil"
)

func (v *Manager) GetExportedMethods() dbusutil.ExportedMethods {
	return dbusutil.ExportedMethods{
		{
			Name:    "GetEdgeMoveStopDuration",
			Fn:      v.GetEdgeMoveStopDuration,
			OutArgs: []string{"duration"},
		},
		{
			Name:    "GetLongPressDuration",
			Fn:      v.GetLongPressDuration,
			OutArgs: []string{"duration"},
		},
		{
			Name:    "GetShortPressDuration",
			Fn:      v.GetShortPressDuration,
			OutArgs: []string{"duration"},
		},
		{
			Name:   "SetEdgeMoveStopDuration",
			Fn:     v.SetEdgeMoveStopDuration,
			InArgs: []string{"duration"},
		},
		{
			Name:   "SetLongPressDuration",
			Fn:     v.SetLongPressDuration,
			InArgs: []string{"duration"},
		},
		{
			Name:   "SetShortPressDuration",
			Fn:     v.SetShortPressDuration,
			InArgs: []string{"duration"},
		},
		{
			Name:    "GetGestureAvaiableActions",
			Fn:      v.GetGestureAvaiableActions,
			InArgs:  []string{"name", "fingers"},
			OutArgs: []string{"actions"},
		},
		{
			Name:   "SetGesture",
			Fn:     v.SetGesture,
			InArgs: []string{"name", "direction", "fingers", "action"},
		},
	}
}
