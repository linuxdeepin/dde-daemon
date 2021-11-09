/*
 * Copyright (C) 2013 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     ganjing <ganjing@uniontech.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package uadp

// #cgo LDFLAGS: -ltc-api
// #include <trusted_cryptography_module.h>
import "C"

import (
	"strconv"
	"sync"
	"unsafe"

	dbus "github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/procfs"
)

const (
	dbusServiceName = "com.deepin.daemon.Uadp"
	dbusPath        = "/com/deepin/daemon/Uadp"
	dbusInterface   = "com.deepin.daemon.Uadp"
)

// trusted cryptography element
type TCElement struct {
	tcHandle      C.TC_HANDLE
	keyPersist    uint32
	encryptBuffer C.TC_BUFFER
	decryptBuffer C.TC_BUFFER
}

type Manager struct {
	service       *dbusutil.Service
	systemSigLoop *dbusutil.SignalLoop
	tcBuffer      map[string]*TCElement

	mu sync.Mutex
}

func NewManager(service *dbusutil.Service) (*Manager, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		service:  service,
		tcBuffer: make(map[string]*TCElement),
	}

	manager.systemSigLoop = dbusutil.NewSignalLoop(sysBus, 10)
	manager.systemSigLoop.Start()
	return manager, nil
}

func (m *Manager) destroy() {
	m.systemSigLoop.Stop()
}

func (m *Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) getExePath(sender dbus.Sender) (string, error) {
	pid, err := m.service.GetConnPID(string(sender))
	if err != nil {
		logger.Warning("failed to get PID:", err)
		return "", err
	}
	process := procfs.Process(pid)
	executablePath, err := process.Exe()
	if err != nil {
		logger.Warning("failed to get executablePath:", err)
		return "", err
	}
	return executablePath, nil
}

func (m *Manager) startTC(sender dbus.Sender) string {
	executablePath, err := m.getExePath(sender)
	if err != nil {
		return ""
	}

	var ret C.TC_RC
	var element *TCElement
	element = new(TCElement)

	ret = C.start_tc((*C.TC_HANDLE)(unsafe.Pointer(&element.tcHandle)), (*C.uint32_t)(unsafe.Pointer(&element.keyPersist)))
	logger.Warningf("---------------->keyPersist:%0x", element.keyPersist)
	if ret != C.TC_SUCCESS {
		C.stop_tc(&element.tcHandle)
		return ""
	} else {
		suffix := strconv.FormatUint(uint64(element.keyPersist), 10)
		keyIndex := executablePath + "-" + suffix
		m.tcBuffer[keyIndex] = element
		return keyIndex
	}
}
