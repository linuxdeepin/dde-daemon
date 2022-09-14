// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

type HideModeType int32

const (
	HideModeKeepShowing HideModeType = iota
	HideModeKeepHidden
	HideModeAutoHide // invalid
	HideModeSmartHide
)

func (t HideModeType) String() string {
	switch t {
	case HideModeKeepShowing:
		return "Keep showing mode"
	case HideModeKeepHidden:
		return "Keep hidden mode"
	case HideModeAutoHide:
		return "Auto hide mode"
	case HideModeSmartHide:
		return "Smart hide mode"
	default:
		return "Unknown mode"
	}
}

type HideStateType int32

const (
	HideStateUnknown HideStateType = iota
	HideStateShow
	HideStateHide
)

func (s HideStateType) String() string {
	switch s {
	case HideStateShow:
		return "Show"
	case HideStateHide:
		return "Hide"
	default:
		return "Unknown"
	}
}

type DisplayModeType int32

const (
	DisplayModeFashionMode DisplayModeType = iota
	DisplayModeEfficientMode
	DisplayModeClassicMode
)

func (t DisplayModeType) String() string {
	switch t {
	case DisplayModeFashionMode:
		return "Fashion mode"
	case DisplayModeEfficientMode:
		return "Efficient mode"
	case DisplayModeClassicMode:
		return "Classic mode"
	default:
		return "Unknown mode"
	}
}

type positionType int32

const (
	positionTop positionType = iota
	positionRight
	positionBottom
	positionLeft
)

func (p positionType) String() string {
	switch p {
	case positionTop:
		return "Top"
	case positionRight:
		return "Right"
	case positionBottom:
		return "Bottom"
	case positionLeft:
		return "Left"
	default:
		return "Unknown"
	}
}

type Rect struct {
	X, Y          int32
	Width, Height uint32
}

func NewRect() *Rect {
	return &Rect{}
}

func (r *Rect) Pieces() (int, int, int, int) {
	return int(r.X), int(r.Y), int(r.Width), int(r.Height)
}

type forceQuitAppType uint8

const (
	forceQuitAppEnabled     forceQuitAppType = iota // 开启
	forceQuitAppDisabled                            // 关闭
	forceQuitAppDeactivated                         // 置灰
)
