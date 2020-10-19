package airplane_mode

import (
	"bytes"
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

func (d *RfkillDev) isBlocked() bool {
	return d.hardBlocked || d.softBlocked
}

func enableBt(enabled bool) error {

	//add bluetoochctl command to aviod adapter statuw abnormal when using rfkill causeing reconnect failed
	action := "off"
	if enabled {
		action = "on"
		exec.Command("rfkill", "unblock", "bluetooth").Run()
		err := exec.Command("/usr/bin/bluetoothctl", "power", action).Run()
		if err != nil {
			return err
		}

		return nil
	}

	err := exec.Command("/usr/bin/bluetoothctl", "power", action).Run()
	if err != nil {
		return err
	}
	exec.Command("rfkill", "block", "bluetooth").Run()

	return nil
}

func getBtEnabled() (bool, error) {
	devices, err := listBt()
	if err != nil {
		return false, err
	}

	blockedCount := 0
	for _, device := range devices {
		if device.isBlocked() {
			blockedCount++
		}
	}
	return blockedCount < len(devices), nil
}

func restartBt(enabled bool) error {
	action := "stop"
	if enabled {
		action = "restart"
	}
	err := exec.Command("service", "bluetooth", action).Run()
	if err != nil {
		logger.Debug("exec.Command RestartBluetooth enabled")
	}
	return err
}

func listBt() ([]*RfkillDev, error) {
	var devices []*RfkillDev
	fileInfoList, err := ioutil.ReadDir(classRfkillDir)
	if err != nil {
		return nil, err
	}
	for _, fileInfo := range fileInfoList {
		d, err := newRfkillDev(filepath.Join(classRfkillDir, fileInfo.Name()))
		if err != nil {
			return nil, err
		}
		if d.Type != "bluetooth" {
			continue
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func readFile(filename string) (string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(content)), nil
}

const (
	activeBlock = "1"
)

func newRfkillDev(p string) (*RfkillDev, error) {
	var d RfkillDev
	buf, err := readFile(filepath.Join(p, "name"))
	if err != nil {
		return nil, err
	}
	d.name = buf

	buf, err = readFile(filepath.Join(p, "index"))
	if err != nil {
		return nil, err
	}
	d.index = buf

	buf, err = readFile(filepath.Join(p, "type"))
	if err != nil {
		return nil, err
	}
	d.Type = buf

	buf, err = readFile(filepath.Join(p, "hard"))
	if err != nil {
		return nil, err
	}
	if buf == activeBlock {
		d.hardBlocked = true
	}

	buf, err = readFile(filepath.Join(p, "soft"))
	if err != nil {
		return nil, err
	}
	if buf == activeBlock {
		d.softBlocked = true
	}

	d.devicePath, err = filepath.EvalSymlinks(filepath.Join(p, "device"))
	if err != nil {
		return nil, err
	}

	return &d, nil
}
