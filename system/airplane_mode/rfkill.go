package airplane_mode

import (
	"bytes"
	"encoding/binary"
	"os"
	"syscall"
)

// rfkillAction2 write rfkill
// write op to /dev/rfkill so kernel will exec ref command
func rfkillAction(typ RadioType, action RadioAction) error {
	// open rfkill file
	file, err := os.OpenFile("/dev/rfkill", os.O_RDWR, 0644)
	if err != nil {
		logger.Warningf("cant open rfkill, err: %v", err)
		return err
	}
	defer file.Close()
	// create event action
	event := &RfkillEvent{
		Typ:  typ.ToRfkillType(),
		Op:   3,
		Soft: action.ToRfkillAction(),
	}
	// create order
	order := getByteOrder()
	// write command to file
	err = binary.Write(file, order, event)
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

func rfkillState(typ RadioType) (rfkillModuleState, error) {
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
		Typ: typ.ToRfkillType(),
	}
	// create order
	order := getByteOrder()
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
		err = binary.Read(info, order, event)
		if err != nil {
			logger.Warningf("binary read rfkill failed, err: %v", err)
			return state, err
		}
		// check type here
		// if type is all type, all type match
		// if type is not all type, ref type match
		// or ignore this device
		if typ != AllRadioType && typ.ToRfkillType() != event.Typ {
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
