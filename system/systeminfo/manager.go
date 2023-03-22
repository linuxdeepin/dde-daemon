// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package systeminfo

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/jouyouyun/hardware/dmi"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

//go:generate dbusutil-gen em -type Manager

// 缓存lshw 指令获取到的数据 key/value --> class/data
var lshwmap map[string]string = make(map[string]string)

const (
	dbusServiceName = "com.deepin.system.SystemInfo"
	dbusPath        = "/com/deepin/system/SystemInfo"
	dbusInterface   = dbusServiceName

	KB = 1 << 10
	MB = 1 << 20
	GB = 1 << 30
	TB = 1 << 40
	EB = 1 << 50
)

type Manager struct {
	service         *dbusutil.Service
	PropsMu         sync.RWMutex
	MemorySize      uint64
	MemorySizeHuman string
	CurrentSpeed    uint64
	DMIInfo         dmi.DMI
	DisplayDriver   string
	VideoDriver     string
}

type lshwXmlList struct {
	Items []lshwXmlNode `xml:"node"`
}

type lshwXmlNode struct {
	Description string `xml:"description"`
	Size        uint64 `xml:"size"`
}

func formatFileSize(fileSize uint64) (size string) {
	if fileSize < KB {
		return fmt.Sprintf("%.2fB", float64(fileSize)/float64(1))
	} else if fileSize < MB {
		return fmt.Sprintf("%.2fKB", float64(fileSize)/float64(KB))
	} else if fileSize < GB {
		return fmt.Sprintf("%.2fMB", float64(fileSize)/float64(MB))
	} else if fileSize < TB {
		return fmt.Sprintf("%.2fGB", float64(fileSize)/float64(GB))
	} else if fileSize < EB {
		return fmt.Sprintf("%.2fTB", float64(fileSize)/float64(TB))
	} else { //if fileSize < (1024 * 1024 * 1024 * 1024 * 1024 * 1024)
		return fmt.Sprintf("%.2fEB", float64(fileSize)/float64(EB))
	}
}

func (m *Manager) GetInterfaceName() string {
	return dbusInterface
}

func NewManager(service *dbusutil.Service) *Manager {
	var m = &Manager{
		service: service,
	}
	return m
}

func setCmdStd(cmd *exec.Cmd) (stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
	stdOut = &bytes.Buffer{}
	stdErr = &bytes.Buffer{}
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	return
}

func isStrEmpty(str string) (out bool) {
	return len(str) <= 0 || str == ""
}

func getLshwData(class string, keyword string, format string) (result string) {
	if isStrEmpty(class) {
		logger.Info("class is Empty")
		return "Input is not allowed to be empty."
	}

	if isStrEmpty(lshwmap[class]) {
		var cmd *exec.Cmd
		if !isStrEmpty(format) {
			cmd = exec.Command("lshw", "-c", class, "-sanitize", format)
		} else {
			cmd = exec.Command("lshw", "-c", class, "-sanitize")
		}

		display, cmderr := setCmdStd(cmd)
		err := cmd.Run()
		// 缓存
		lshwmap[class] = display.String()

		if err != nil {
			logger.Info("lshw error: ", err, cmderr)
		}

		// 没有手动指定 keyword，无需 grep
		if isStrEmpty(keyword) {
			return display.String()
		}

		// 指定了format
		if !isStrEmpty(format) && !isStrEmpty(display.String()) {
			return lshwmap[class]
		}
	} else {
		if isStrEmpty(keyword) {
			return lshwmap[class]
		}
	}

	cmd := exec.Command("grep", keyword)
	cmd.Stdin = bytes.NewBufferString(lshwmap[class]) // 把上面的执行结果放到grep 的输入中
	stdout, stderr := setCmdStd(cmd)
	err := cmd.Run()

	if err != nil {
		logger.Info("lshw error: ", err, stderr)
	}

	return stdout.String()
}

func parseXml(bytes []byte) (result lshwXmlNode, err error) {
	logger.Debug("ParseXml bytes: ", string(bytes))
	var list lshwXmlList
	err = xml.Unmarshal(bytes, &list)
	if err != nil {
		logger.Error(err)
		return result, err
	}
	len := len(list.Items)
	for i := 0; i < len; i++ {
		data := list.Items[i]
		logger.Debug("Description : ", data.Description, " , size : ", data.Size)
		if strings.ToLower(data.Description) == "system memory" {
			result = data
		}
	}
	return result, err
}

func (m *Manager) setMemorySize(value uint64) {
	m.MemorySize = value
	err := m.service.EmitPropertyChanged(m, "MemorySize", m.MemorySize)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) setMemorySizeHuman(value string) {
	m.MemorySizeHuman = value
	err := m.service.EmitPropertyChanged(m, "MemorySizeHuman", m.MemorySizeHuman)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) calculateMemoryViaLshw() error {
	cmdOutBuf := []byte(getLshwData("memory", "", "-xml"))
	ret, err1 := parseXml(cmdOutBuf)
	if err1 != nil {
		logger.Error(err1)
		return err1
	}
	memory := formatFileSize(ret.Size)
	m.PropsMu.Lock()
	//set property value
	m.setMemorySize(ret.Size)
	m.setMemorySizeHuman(memory)
	m.PropsMu.Unlock()
	logger.Debug("system memory : ", ret.Size)
	return nil
}

func GetCurrentSpeed(systemBit int) (uint64, error) {
	ret, err := getCurrentSpeed(systemBit)
	return ret, err
}

func getCurrentSpeed(systemBit int) (uint64, error) {
	var ret uint64 = 0
	cmdOutBuf, err := runDmidecode()
	if err != nil {
		return ret, err
	}
	ret, err = parseCurrentSpeed(cmdOutBuf, systemBit)
	if err != nil {
		logger.Error(err)
		return ret, err
	}
	logger.Debug("GetCurrentSpeed :", ret)
	return ret, err
}

func runDmidecode() (string, error) {
	cmd := exec.Command("dmidecode", "-t", "processor")
	out, err := cmd.Output()
	if err != nil {
		logger.Error(err)
	}
	return string(out), err
}

//From string parse "Current Speed"
func parseCurrentSpeed(bytes string, systemBit int) (result uint64, err error) {
	logger.Debug("parseCurrentSpeed data: ", bytes)
	lines := strings.Split(bytes, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "Current Speed:") {
			continue
		}
		items := strings.Split(line, "Current Speed:")
		ret := ""
		if len(items) == 2 {
			//Current Speed: 3200 MHz
			ret = items[1]
			value, err := strconv.ParseUint(strings.TrimSpace(filterUnNumber(ret)), 10, systemBit)
			if err != nil {
				logger.Error(err)
				return result, err
			}
			result = value
		}
		break
	}
	return result, err
}

//仅保留字符串中的数字
func filterUnNumber(value string) string {
	reg, err := regexp.Compile("[^0-9]+")
	if err != nil {
		logger.Fatal(err)
	}
	return reg.ReplaceAllString(value, "")
}

//执行命令：/usr/bin/getconf LONG_BIT 获取系统位数
func (m *Manager) systemBit() string {
	output, err := exec.Command("/usr/bin/getconf", "LONG_BIT").Output()
	if err != nil {
		return "64"
	}

	v := strings.TrimRight(string(output), "\n")
	return v
}
