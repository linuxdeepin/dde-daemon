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
	"errors"
	"fmt"
	"strings"
	"unsafe"

	dbus "github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
)

func (m *Manager) EncryptData(sender dbus.Sender, data []byte) *dbus.Error {
	keyIndex := m.startTC(sender)
	if keyIndex == "" {
		return dbusutil.ToError(fmt.Errorf("Caller[%s] has not hold a device handle yet.", sender))
	}

	m.mu.Lock()
	element, _ := m.tcBuffer[keyIndex]
	m.mu.Unlock()

	toString := string(data[:])
	cs := C.CString(toString)
	defer C.free(unsafe.Pointer(cs))

	var ret C.TC_RC
	ret = C.encrypt((*C.TC_HANDLE)(unsafe.Pointer(&element.tcHandle)), (*C.uint)(unsafe.Pointer(&element.keyPersist)), cs, C.int(len(data)), &element.encryptBuffer)
	// 加密完后就直接断开与TPM设备的连接，释放资源
	C.stop_tc((*C.TC_HANDLE)(unsafe.Pointer(&element.tcHandle)))

	if ret != C.TC_SUCCESS {
		return dbusutil.ToError(fmt.Errorf("Caller[%s] encrypt data failed.", keyIndex))
	} else {
		return nil
	}
}

func (m *Manager) DecryptData(sender dbus.Sender) (data []byte, busErr *dbus.Error) {
	var keyIndex string
	for key, _ := range m.tcBuffer {
		// TODO： 此处逻辑需要修改。模糊匹配， 目前没有实现处理应用多开的场景。
		if strings.Contains(key, string(sender)) {
			keyIndex = key
			break
		}
	}

	m.mu.Lock()
	element, ok := m.tcBuffer[keyIndex]
	m.mu.Unlock()

	if !ok {
		return []byte{}, dbusutil.ToError(fmt.Errorf("Caller:%s has not encrypted any data.", sender))
	}

	var ret C.TC_RC
	ret = C.decrypt((*C.TC_HANDLE)(unsafe.Pointer(&element.tcHandle)), (*C.uint)(unsafe.Pointer(&element.keyPersist)), &element.encryptBuffer, &element.decryptBuffer)
	// 解密完后就直接断开与TPM设备的连接，释放资源
	C.stop_tc((*C.TC_HANDLE)(unsafe.Pointer(&element.tcHandle)))

	if ret != C.TC_SUCCESS {
		return []byte{}, dbusutil.ToError(errors.New("Failed to Decrypt."))
	} else {
		info := (*[1 << 30]byte)(unsafe.Pointer(element.decryptBuffer.buffer))[:element.decryptBuffer.size:element.decryptBuffer.size]
		return info, nil
	}
}
