// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/godbus/dbus/v5"
	accounts "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.accounts1"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

const (
	stdXAuthFileMod = 0600
)

func fix(conn *dbus.Conn, userPath string) error {
	userObj, err := accounts.NewUser(conn, dbus.ObjectPath(userPath))
	if err != nil {
		return err
	}
	homeDir, err := userObj.HomeDir().Get(0)
	if err != nil {
		return err
	}
	uidStr, err := userObj.Uid().Get(0)
	if err != nil {
		return err
	}

	uid, err := strconv.Atoi(uidStr)
	if err != nil {
		return err
	}
	xAuthFile := os.Getenv("XAUTHORITY")
	if len(xAuthFile) == 0 {
		xAuthFile = filepath.Join(homeDir, ".Xauthority")
	}
	fileInfo, err := os.Stat(xAuthFile)
	if err != nil {
		if os.IsNotExist(err) {
			return createXAuthFile(xAuthFile, uid)
		}
		return err
	}

	// fix mod
	if fileInfo.Mode() != stdXAuthFileMod {
		err = os.Chmod(xAuthFile, stdXAuthFileMod)
		if err != nil {
			return err
		}
	}

	sysStat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("failed to convert fileInfo.Sys() to *syscall.Stat_t")
	}

	// fix own
	if int(sysStat.Uid) != uid {
		err = os.Chown(xAuthFile, uid, uid)
		if err != nil {
			return err
		}
	}

	return nil
}

func createXAuthFile(filename string, uid int) error {
	fh, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer fh.Close()

	_, err = fh.Write([]byte{0})
	if err != nil {
		return err
	}

	err = fh.Chmod(stdXAuthFileMod)
	if err != nil {
		return err
	}

	err = fh.Chown(uid, uid)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		log.Fatal(err)
	}
	accountsObj := accounts.NewAccounts(sysBus)
	userList, err := accountsObj.UserList().Get(0)
	if err != nil {
		log.Fatal(err)
	}
	for _, userPath := range userList {
		err := fix(sysBus, userPath)
		if err != nil {
			log.Println(err)
		}
	}
}
