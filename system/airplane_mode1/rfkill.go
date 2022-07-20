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

		mgr.handleRfkillEvent(event)
	}
}

// handleRfkillEvent when rfkill event arrived, should refresh all rfkill state
func (mgr *Manager) handleRfkillEvent(event *RfkillEvent) {
	if event.Op == rfkillOpDel {
		delete(mgr.devices, event.Index)
	} else {
		mgr.devices[event.Index] = device{
			typ:  event.Typ,
			soft: event.Soft,
			hard: event.Hard,
		}
	}

	deviceCnt := len(mgr.devices)
	blockCnt := 0
	softBlockCnt := 0
	curTypeDeviceCnt := 0
	curTypeBlockCnt := 0
	curTypeSoftBlockCnt := 0
	for _, device := range mgr.devices {
		if device.soft == rfkillStateBlock || device.hard == rfkillStateBlock {
			blockCnt++

			if device.soft == rfkillStateBlock {
				softBlockCnt++
			}
		}

		if device.typ != event.Typ {
			continue
		}

		curTypeDeviceCnt++
		if device.soft == rfkillStateBlock || device.hard == rfkillStateBlock {
			curTypeBlockCnt++

			if device.soft == rfkillStateBlock {
				curTypeSoftBlockCnt++
			}
		}
	}

	allBlocked := false
	allSoftBlocked := false
	if deviceCnt != 0 {
		allBlocked = blockCnt == deviceCnt
		allSoftBlocked = softBlockCnt == deviceCnt
	}

	curTypeBlocked := false
	curTypeSoftBlocked := false
	if curTypeDeviceCnt != 0 {
		curTypeBlocked = curTypeBlockCnt == curTypeDeviceCnt
		curTypeSoftBlocked = curTypeSoftBlockCnt == curTypeDeviceCnt
	}

	mgr.setPropHasAirplaneMode(deviceCnt != 0)

	mgr.setPropEnabled(allBlocked)
	logger.Debug("refresh all blocked state:", allBlocked)

	// check is module is blocked
	switch event.Typ {
	case rfkillTypeWifi:
		mgr.setPropWifiEnabled(curTypeBlocked)
		logger.Debug("refresh wifi blocked state:", curTypeBlocked)
	case rfkillTypeBT:
		mgr.setPropBluetoothEnabled(curTypeBlocked)
		logger.Debug("refresh bluetooth blocked state:", curTypeBlocked)
	default:
		logger.Info("unsupported type:", event.Typ)
	}
	// 仅保存 soft block 的状态
	mgr.config.SetBlocked(rfkillTypeAll, allSoftBlocked)
	mgr.config.SetBlocked(event.Typ, curTypeSoftBlocked)
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
