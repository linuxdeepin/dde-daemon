// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package clipboard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/linuxdeepin/dde-daemon/clipboard1/mocks"
	x "github.com/linuxdeepin/go-x11-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventCaptor(t *testing.T) {
	conn, err := x.NewConn()
	if err != nil {
		t.Skip("failed to connect x")
	}
	defer func() {
		conn.Close()
	}()

	win, err := createWindow(conn)
	require.NoError(t, err)
	assert.NotZero(t, win)

	err = changeWindowEventMask(conn, win, x.EventMaskPropertyChange)
	require.NoError(t, err)

	eventChan := make(chan x.GenericEvent, 10)
	conn.AddEventChan(eventChan)
	ec := newEventCaptor()
	defer close(ec.quit)
	go func() {
		for ev := range eventChan {
			code := ev.GetEventCode()
			switch code {
			case x.PropertyNotifyEventCode:
				event, _ := x.NewPropertyNotifyEvent(ev)
				ec.handleEvent(event)
			}
		}
	}()

	prop, err := conn.GetAtom("PROP_1")
	require.NoError(t, err)

	pne, err := ec.capturePropertyNotifyEvent(func() error {
		return x.ChangePropertyChecked(conn, x.PropModeReplace, win, prop, x.AtomCardinal, 32, nil).Check(conn)
	}, func(event *x.PropertyNotifyEvent) bool {
		return event.Window == win &&
			event.Atom == prop &&
			event.State == x.PropertyNewValue
	})
	require.NoError(t, err)
	assert.Equal(t, win, pne.Window)
	assert.Equal(t, prop, pne.Atom)
	assert.EqualValues(t, x.PropertyNewValue, pne.State)
}

// test createWindow, getSelectionOwner
func Test_getSelectionOwner(t *testing.T) {
	conn, err := x.NewConn()
	if err != nil {
		t.Skip("failed to connect x")
	}
	defer func() {
		conn.Close()
	}()

	win, err := createWindow(conn)
	require.NoError(t, err)
	assert.NotZero(t, win)

	sel, err := conn.GetAtom("CLIPBOARD_test")
	require.NoError(t, err)
	assert.NotZero(t, sel)

	err = x.SetSelectionOwnerChecked(conn, win, sel, x.CurrentTime).Check(conn)
	require.NoError(t, err)

	owner, err := getSelectionOwner(conn, sel)
	assert.Equal(t, win, owner)
	assert.NoError(t, err)
}

func Test_getBytesMd5sum(t *testing.T) {
	sum := getBytesMd5sum([]byte("hello world"))
	assert.Equal(t, "5eb63bbbe01eeed093cb22bb8f5acdc3", sum)
}

func Test_emptyDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "dde-daemon-clipboard-test")
	if err != nil {
		assert.FailNow(t, "failed to create temp dir: %v", err)
	}
	t.Log("dir:", dir)
	err = os.WriteFile(filepath.Join(dir, "1"), []byte("abc"), 0644)
	assert.NoError(t, err)

	err = os.Mkdir(filepath.Join(dir, "d1"), 0755)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "d1/1"), []byte("abc"), 0644)
	assert.NoError(t, err)

	err = emptyDir(dir)
	assert.NoError(t, err)

	err = os.Remove(dir)
	assert.NoError(t, err)
}

func Test_setSelectionOwner(t *testing.T) {
	win := x.Window(1)
	selection := x.Atom(2)
	ts := x.Timestamp(3)
	xc := &mocks.XClient{}
	xc.On("SetSelectionOwner", win, selection, ts).Return(nil).Twice()
	xc.On("GetSelectionOwner", selection).Return(win, nil).Once()

	err := setSelectionOwner(xc, win, selection, ts)
	assert.NoError(t, err)

	xc.On("GetSelectionOwner", selection).Return(x.Window(2), nil).Once()
	err = setSelectionOwner(xc, win, selection, ts)
	assert.Error(t, err)
	xc.AssertExpectations(t)
}

func Test_getAtomListFormReply(t *testing.T) {
	w := x.NewWriter()
	w.Write4b(1)
	w.Write4b(2)
	w.Write4b(3)

	reply := &x.GetPropertyReply{
		Format:   32,
		Value:    w.Bytes(),
		ValueLen: 3,
	}

	atomList, err := getAtomListFormReply(reply)
	assert.Equal(t, []x.Atom{1, 2, 3}, atomList)
	assert.NoError(t, err)

	reply.Format = 0
	atomList, err = getAtomListFormReply(reply)
	assert.Nil(t, atomList)
	assert.Error(t, err)
}
