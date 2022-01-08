package airplane_mode

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"
)

const (
	classRfkillDir = "/sys/class/rfkill"
)

type RfkillDev struct {
	name        string
	index       string
	hardBlocked bool
	softBlocked bool
	Type        string
	devicePath  string
}

// isBlocked check if device is blocked
func (d *RfkillDev) isBlocked() bool {
	return d.hardBlocked || d.softBlocked
}

type RfkillModule int

const (
	RfkillBluetooth RfkillModule = iota
	RfkillWlan
	RfkillAll
)

// String module name
func (module RfkillModule) String() string {
	var name string
	switch module {
	case RfkillBluetooth:
		name = "bluetooth"
	case RfkillWlan:
		name = "wlan"
	case RfkillAll:
		name = "all"
	}
	return name
}

// rfkillAction use to block or unblock rfkill
func rfkillAction(module RfkillModule, blocked bool) error {
	// check if is block or unblock
	action := "unblock"
	if blocked {
		action = "block"
	}
	// create
	cmd := exec.Command("rfkill", action, module.String())
	logger.Debugf("run rfkill command, command: %v", cmd.String())
	// run and wait
	buf, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warningf("run rfkill failed, msg: %v, err: %v", string(buf), err)
		return err
	}
	return nil
}

// check if module is block
// at least exist one rfkill config is non-block, this module is not block
func isModuleBlocked(module RfkillModule) bool {
	// check current dir
	fileInfoSl, err := ioutil.ReadDir(classRfkillDir)
	if err != nil {
		return false
	}
	logger.Debugf("rfkill dir: %v", fileInfoSl)
	// check every class
	for _, fileInfo := range fileInfoSl {
		// read rfkill device file info
		dev, err := newRfkillDev(filepath.Join(classRfkillDir, fileInfo.Name()))
		if err != nil {
			logger.Debugf("read rfkill state failed, filename: %v, err: %v", fileInfo.Name(), err)
			continue
		}
		// check type
		if dev.Type != module.String() {
			continue
		}
		logger.Debugf("dev name is %v, dev type is %v, block: %v", dev.name, dev.Type, dev.isBlocked())
		// check if is block, if is non-block
		// means this module is not blocked
		if !dev.isBlocked() {
			return false
		}
	}
	return true
}

const (
	activeBlock = "1"
)

func newRfkillDev(devDir string) (*RfkillDev, error) {
	logger.Debugf("read rfkill file: %v", devDir)
	var d RfkillDev
	buf, err := readFile(filepath.Join(devDir, "name"))
	if err != nil {
		return nil, err
	}
	d.name = buf

	buf, err = readFile(filepath.Join(devDir, "index"))
	if err != nil {
		return nil, err
	}
	d.index = buf

	buf, err = readFile(filepath.Join(devDir, "type"))
	if err != nil {
		return nil, err
	}
	d.Type = buf

	buf, err = readFile(filepath.Join(devDir, "hard"))
	if err != nil {
		return nil, err
	}
	if buf == activeBlock {
		d.hardBlocked = true
	}

	buf, err = readFile(filepath.Join(devDir, "soft"))
	if err != nil {
		return nil, err
	}
	if buf == activeBlock {
		d.softBlocked = true
	}

	d.devicePath, err = filepath.EvalSymlinks(filepath.Join(devDir, "device"))
	if err != nil {
		return nil, err
	}

	return &d, nil
}
