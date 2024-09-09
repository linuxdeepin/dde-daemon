// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package clipboard

import (
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/linuxdeepin/go-lib/log"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/linuxdeepin/go-x11-client/ext/xfixes"
)

func getSelectionOwner(conn *x.Conn, selection x.Atom) (x.Window, error) {
	reply, err := x.GetSelectionOwner(conn, selection).Reply(conn)
	if err != nil {
		return 0, err
	}
	return reply.Owner, nil
}

// deprecated
func changeWindowEventMask(conn *x.Conn, win x.Window, evMask uint32) error {
	const valueMask = x.CWEventMask
	return x.ChangeWindowAttributesChecked(conn, win, valueMask, []uint32{evMask}).Check(conn)
}

func createWindow(xConn *x.Conn) (x.Window, error) {
	xid, err := xConn.AllocID()
	if err != nil {
		return 0, err
	}

	win := x.Window(xid)

	const valueMask = x.CWEventMask
	valueList := []uint32{x.EventMaskPropertyChange}
	screen := xConn.GetDefaultScreen()
	err = x.CreateWindowChecked(xConn,
		x.None,        // depth
		win,           // window
		screen.Root,   // parent
		0, 0, 1, 1, 0, // x,y, w, h, bw
		x.WindowClassInputOnly, // class
		x.None,                 // visual
		valueMask, valueList).Check(xConn)
	if err != nil {
		return 0, err
	}
	return win, nil
}

func sendClientMsg(xConn *x.Conn, win x.Window, typeAtom x.Atom, data x.ClientMessageData) error {
	var event = x.ClientMessageEvent{
		Format: 32,
		Window: win,
		Type:   typeAtom,
		Data:   data,
	}

	w := x.NewWriter()
	x.WriteClientMessageEvent(w, &event)
	const evMask = x.EventMaskSubstructureNotify | x.EventMaskSubstructureRedirect
	rootWin := xConn.GetDefaultScreen().Root
	return x.SendEventChecked(xConn, false, rootWin, evMask, w.Bytes()).Check(xConn)
}

func announceManageSelection(xConn *x.Conn, owner x.Window, selection x.Atom, time x.Timestamp) error {
	var data x.ClientMessageData
	data.SetData32(&[5]uint32{
		uint32(time),
		uint32(selection),
		uint32(owner),
	})
	rootWin := xConn.GetDefaultScreen().Root
	atomManager, err := xConn.GetAtom("MANAGER")
	if err != nil {
		return err
	}
	return sendClientMsg(xConn, rootWin, atomManager, data)
}

// 获取有关 atom 的良好描述
func getAtomDesc(xConn *x.Conn, atom x.Atom) string {
	numStr := strconv.Itoa(int(atom))
	if logger.GetLogLevel() < log.LevelDebug {
		return numStr
	}
	name, err := xConn.GetAtomName(atom)
	if err != nil {
		logger.Warning(err)
		return numStr
	}
	return name + "|" + numStr
}

func selReqEventToString(ev *x.SelectionRequestEvent, xConn *x.Conn) string {
	return fmt.Sprintf("SelectionRequest{ Time: %d, Owner: %d, Requestor: %d, Selection: %s, Target: %s, Property: %s}",
		ev.Time, ev.Owner, ev.Requestor,
		getAtomDesc(xConn, ev.Selection),
		getAtomDesc(xConn, ev.Target),
		getAtomDesc(xConn, ev.Property))
}

func selNotifyEventToString(ev *x.SelectionNotifyEvent, xConn *x.Conn) string {
	return fmt.Sprintf("SelectionNotify{ Time: %d, Requestor: %d, Selection: %s, Target: %s, Property: %s}",
		ev.Time, ev.Requestor,
		getAtomDesc(xConn, ev.Selection),
		getAtomDesc(xConn, ev.Target),
		getAtomDesc(xConn, ev.Property))
}

func destroyNotifyEventToString(ev *x.DestroyNotifyEvent) string {
	return fmt.Sprintf("DestroyNotify{ Event: %d, Window: %d }", ev.Event, ev.Window)
}

func selClearEventToString(ev *x.SelectionClearEvent) string {
	return fmt.Sprintf("SelectionClear{ Time: %d, Owner: %d, Selection: %d }",
		ev.Time, ev.Owner, ev.Selection)
}

func xfixesSelNotifyEventToString(ev *xfixes.SelectionNotifyEvent) string {
	var subtypeStr string
	switch ev.Subtype {
	case xfixes.SelectionEventSetSelectionOwner:
		subtypeStr = "set-selection-owner"
	case xfixes.SelectionEventSelectionClientClose:
		subtypeStr = "client-close"
	case xfixes.SelectionEventSelectionWindowDestroy:
		subtypeStr = "window-destroy"
	}
	return fmt.Sprintf("xfixes.SelectionNotify{ Time: %d, SelectionTime: %d, Window: %d, Owner: %d, Selection: %d, subtype: %s}",
		ev.Timestamp, ev.SelectionTimestamp, ev.Window, ev.Owner, ev.Selection, subtypeStr)
}

func propNotifyEventToString(ev *x.PropertyNotifyEvent, xConn *x.Conn) string {
	var stateStr string
	switch ev.State {
	case x.PropertyNewValue:
		stateStr = "new"
	case x.PropertyDelete:
		stateStr = "delete"
	default:
		stateStr = "?"
	}
	return fmt.Sprintf("PropertyNotify{ Time: %d, State: %s,  Window: %d , Atom: %s }",
		ev.Time, stateStr, ev.Window, getAtomDesc(xConn, ev.Atom))
}

func getAtomListFormReply(reply *x.GetPropertyReply) ([]x.Atom, error) {
	if reply.Format != 32 {
		return nil, errors.New("bad format")
	}

	count := len(reply.Value) / 4
	ret := make([]x.Atom, count)
	for i := 0; i < count; i++ {
		ret[i] = x.Atom(x.Get32(reply.Value[i*4:]))
	}
	return ret, nil
}

func getBytesMd5sum(data []byte) string {
	h := md5.New()
	_, err := h.Write(data)
	if err != nil {
		logger.Warning("Write error:", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func emptyDir(dir string) error {
	fileInfoList, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, fileInfo := range fileInfoList {
		err = os.RemoveAll(filepath.Join(dir, fileInfo.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}
