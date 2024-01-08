// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package airplane_mode

import (
	"bytes"
	"encoding/binary"
	"os"
	"syscall"
)

var endian binary.ByteOrder = getByteOrder()

type rfkillType uint8

const (
	rfkillTypeAll rfkillType = iota
	rfkillTypeWifi
	rfkillTypeBT
)

type rfkillOp uint8

const (
	rfkillOpAdd rfkillOp = iota
	rfkillOpDel
	rfkillOpChange
	rfkillOpChangeAll
)

type rfkillState uint8

const (
	rfkillStateUnblock rfkillState = iota
	rfkillStateBlock
)

// RfkillEvent rfkill event
type RfkillEvent struct {
	Index uint32
	Typ   rfkillType
	Op    rfkillOp
	Soft  rfkillState
	Hard  rfkillState
}

func (mgr *Manager) listenRfkill() {
	fd, err := syscall.Open("/dev/rfkill", syscall.O_RDWR, 0777)
	if err != nil {
		logger.Warning("failed to open /dev/rfkill:", err)
		return
	}
	defer syscall.Close(fd)

	event := &RfkillEvent{}
	b := make([]byte, 512)

	for {
		n, err := syscall.Read(fd, b)
		if err != nil {
			logger.Warning("failed to read /dev/rfkill:", err)
			continue
		}
		if n != binary.Size(event) {
			logger.Warning("wrong size:", n)
			continue
		}

		info := bytes.NewBuffer(b)
		err = binary.Read(info, endian, event)
		if err != nil {
			logger.Warning("failed to read event info:", err)
			continue
		}

		mgr.handleBTRfkillEvent(event)
	}
}

// 只处理bluetooth设备
func (mgr *Manager) handleBTRfkillEvent(event *RfkillEvent) {
	if event.Typ != rfkillTypeBT {
		return
	}
	mgr.btDevicesMu.Lock()
	defer mgr.btDevicesMu.Unlock()
	if event.Op == rfkillOpDel {
		delete(mgr.btRfkillDevices, event.Index)
	} else {
		mgr.btRfkillDevices[event.Index] = device{
			typ:  rfkillTypeBT,
			soft: event.Soft,
			hard: event.Hard,
		}
	}
	btCnt := len(mgr.btRfkillDevices)
	blockBtCnt := 0
	softBtBlockCnt := 0
	for _, device := range mgr.btRfkillDevices {
		if device.soft == rfkillStateBlock || device.hard == rfkillStateBlock {
			blockBtCnt++
			if device.soft == rfkillStateBlock {
				softBtBlockCnt++
			}
		}
	}
	btBlocked := btCnt == blockBtCnt
	btSoftBlocked := btCnt == softBtBlockCnt
	mgr.setPropHasAirplaneMode(btCnt != 0 || mgr.hasNmWirelessDevices)
	mgr.setPropBluetoothEnabled(btBlocked && btCnt != 0)
	logger.Debug("refresh bluetooth blocked state:", btBlocked)
	if mgr.hasNmWirelessDevices {
		mgr.setPropEnabled(btBlocked && mgr.WifiEnabled)
		logger.Debug("refresh all blocked state:", btBlocked && mgr.WifiEnabled)
		// 仅保存 soft block 的状态
		mgr.config.SetBlocked(rfkillTypeAll, btSoftBlocked && mgr.WifiEnabled)
	} else {
		mgr.setPropEnabled(btBlocked && btCnt != 0)
		logger.Debug("refresh all blocked state:", btBlocked)
		// 仅保存 soft block 的状态
		mgr.config.SetBlocked(rfkillTypeAll, btSoftBlocked)
	}
	mgr.config.SetBlocked(rfkillTypeBT, btSoftBlocked)
	// save rfkill key event result to config file
	err := mgr.config.SaveConfig()
	if err != nil {
		logger.Warningf("save rfkill config file failed, err: %v", err)
	}
	logger.Debugf("rfkill state, bluetooth: %v, wifi: %v, airplane: %v", mgr.BluetoothEnabled, mgr.WifiEnabled, mgr.Enabled)
}

// rfkillAction2 write rfkill
// write op to /dev/rfkill so kernel will exec ref command
func rfkillAction(typ rfkillType, action rfkillState) error {
	// open rfkill file
	file, err := os.OpenFile("/dev/rfkill", os.O_RDWR, 0644)
	if err != nil {
		logger.Warningf("cant open rfkill, err: %v", err)
		return err
	}
	defer file.Close()
	// create event action
	event := &RfkillEvent{
		Typ:  typ,
		Op:   rfkillOpChangeAll,
		Soft: action,
	}
	// write command to file
	err = binary.Write(file, endian, event)
	if err != nil {
		logger.Warningf("set rfkill state failed, type: %v, action: %v, err: %v", typ, action, err)
		return err
	}
	logger.Infof("set rfkill state success, type: %v, action: %v", typ, action)
	return nil
}

type rfkillModuleState struct {
	count   int
	blocked bool
}

func getRfkillState(typ rfkillType) (rfkillModuleState, error) {
	state := rfkillModuleState{
		count:   0,
		blocked: true,
	}
	// open rfkill file
	file, err := os.Open("/dev/rfkill")
	if err != nil {
		logger.Warningf("cant open rfkill, err: %v", err)
		return state, err
	}
	// close file
	defer file.Close()
	// get fd
	fd := int(file.Fd())
	defer syscall.Close(fd)
	// set non-block
	err = syscall.SetNonblock(fd, true)
	if err != nil {
		logger.Warningf("cant set non-block, err: %v", err)
		return state, err
	}
	// create event action
	event := &RfkillEvent{
		Typ: typ,
	}
	// create reader
	buf := make([]byte, 512)
	for {
		// call to read rfkill info
		_, err = syscall.Read(fd, buf)
		if err != nil {
			// since fd is non-block, eagain means read end
			if err == syscall.EAGAIN {
				break
			}
			logger.Warningf("read rfkill failed, err: %v", err)
			return state, err
		}
		// unmarshal to event
		info := bytes.NewBuffer(buf)
		// read event from info
		err = binary.Read(info, endian, event)
		if err != nil {
			logger.Warningf("binary read rfkill failed, err: %v", err)
			return state, err
		}
		// check type here
		// if type is all type, all type match
		// if type is not all type, ref type match
		// or ignore this device
		if typ != rfkillTypeAll && typ != event.Typ {
			continue
		}
		// add count
		state.count++
		// check block state
		// if exist at least one device is non block,
		// this module is blocked now
		if event.Hard == 0 && event.Soft == 0 {
			state.blocked = false
		}
		continue
	}
	logger.Infof("module state: %v", state)
	return state, nil
}
