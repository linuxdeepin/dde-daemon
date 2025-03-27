// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package xsettings

import (
	"os"

	"github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/util/wm/ewmh"
)

const (
	settingPropScreen   = "_XSETTINGS_S0"
	settingPropSettings = "_XSETTINGS_SETTINGS"

	xsDataOrder  = 0
	xsDataSerial = 0
	xsDataFormat = 8
)

func getSelectionOwner(prop string, conn *x.Conn) (x.Window, error) {
	atom, err := getAtomByProp(prop, conn)
	if err != nil {
		return 0, err
	}

	reply, err := x.GetSelectionOwner(conn, atom).Reply(conn)
	if err != nil {
		return 0, err
	}

	return reply.Owner, nil
}

func isSelectionOwned(prop string, wid x.Window, conn *x.Conn) bool {
	owner, err := getSelectionOwner(prop, conn)
	if err != nil {
		return false
	}

	if owner == 0 || owner != wid {
		return false
	}

	return true
}

func getAtomByProp(prop string, conn *x.Conn) (x.Atom, error) {
	return conn.GetAtom(prop)
}

func getSettingPropValue(owner x.Window, conn *x.Conn) ([]byte, error) {
	atom, err := getAtomByProp(settingPropSettings, conn)
	if err != nil {
		return nil, err
	}

	reply, err := x.GetProperty(conn, false, owner,
		atom, atom, 0, 10240).Reply(conn)
	if err != nil {
		return nil, err
	}

	return reply.Value, nil
}

func changeSettingProp(owner x.Window, data []byte, conn *x.Conn) error {
	atom, err := getAtomByProp(settingPropSettings, conn)
	if err != nil {
		return err
	}

	return x.ChangePropertyChecked(conn, x.PropModeReplace,
		owner, atom, atom,
		xsDataFormat, data).Check(conn)
}

func createSettingWindow(conn *x.Conn) (x.Window, error) {
	screenAtom, err := getAtomByProp(settingPropScreen, conn)
	if err != nil {
		return 0, err
	}

	xid, err := conn.AllocID()
	if err != nil {
		return 0, err
	}
	wid := x.Window(xid)

	root := conn.GetDefaultScreen().Root
	err = x.CreateWindowChecked(conn, 0, wid, root,
		0, 0, 1, 1, 0,
		x.WindowClassInputOnly, x.CopyFromParent,
		0, nil).Check(conn)
	if err != nil {
		return 0, err
	}

	err = changeWindowPid(conn, wid)
	if err != nil {
		return 0, err
	}

	err = x.SetSelectionOwnerChecked(conn, wid, screenAtom,
		x.CurrentTime).Check(conn)
	if err != nil {
		return 0, err
	}

	return wid, nil
}

func changeWindowPid(conn *x.Conn, wid x.Window) error {
	pid := uint32(os.Getpid())
	return ewmh.SetWMPidChecked(conn, wid, pid).Check(conn)
}

func pad(n int) int {
	return x.Pad(n)
}
