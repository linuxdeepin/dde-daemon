/*
 * Copyright (C) 2020 ~ 2020 Deepin Technology Co., Ltd.
 *
 * Author:     wubowen <wubowen@uniontech.com>
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

package main

import (
	"strings"

	"github.com/godbus/dbus"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/keyfile"
	"pkg.deepin.io/lib/procfs"
)

const (
	supportVirsConf  = "/usr/share/dde-daemon/supportVirsConf.ini"
	virsGroupAppName = "AppName"
	virsKeySupport   = "support"
)

var (
	defaultSupportedVirtualMachines []string = []string{"hvm", "bochs", "virt", "vmware", "kvm", "cloud", "invented"}
	defaultWhiteListDatas           []string = []string{"print"}
	supportedVirtualMachines        []string
	whitelistDatas                  []string
)

func init() {
	//为了尽可能小的影响性能优化，将获取数据放到协程
	go func() {
		//将有效的支持虚拟机数据存入supportedVirtualMachines，以供后面截图的时候直接调用
		data := readSupConfigFile(virsGroupAppName, virsKeySupport)
		if data != nil {
			supportedVirtualMachines = getValidSupData(data)
		} else {
			supportedVirtualMachines = defaultSupportedVirtualMachines
		}
		logger.Info("support virtual : ", supportedVirtualMachines)
	}()
}

//从配置文件读取支持App的关键字段
//当获取不到"/usr/share/dde-daemon/supportVirsConf.ini"数据的时候，使用默认值
func readSupConfigFile(key, value string) []string {
	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(supportVirsConf)
	if err != nil {
		logger.Warning("load version file failed, err: ", err)
		return nil
	}
	ret, err := kf.GetStringList(key, value)
	if err != nil {
		logger.Warning("get version type failed, err: ", err)
		return nil
	}

	return ret
}

func getValidSupData(supApps []string) []string {
	//统计有效(非空)数据个数
	validLen := 0
	for _, value := range supApps {
		if len(strings.TrimSpace(value)) == 0 {
			continue
		}
		validLen++
	}
	ret := make([]string, validLen)
	for _, value := range supApps {
		if len(strings.TrimSpace(value)) == 0 {
			continue
		}
		ret[validLen-1] = strings.ToLower(strings.TrimSpace(value))
		validLen--
	}

	return ret
}

//获取App二进制名称，将exe和cmdline拼接成一个string
func getActivePidInfo(pid uint32) (execPath string, err error) {
	value := procfs.Process(pid)
	execPath, err = value.Exe()
	if err != nil {
		logger.Warning(err)
		return "", err
	}
	cmdlines, err1 := value.Cmdline()
	if err1 != nil {
		logger.Warning(err1)
		return "", err1
	}
	for i := 0; i < len(cmdlines); i++ {
		execPath += cmdlines[i]
	}

	return strings.ToLower(execPath), err
}

//判断是否是虚拟机
func isVirtual(exe string, supApps []string) bool {
	for _, vir := range supApps {
		//exe中是否包含，去掉空格的虚拟机相关字段
		if strings.Contains(exe, vir) {
			logger.Info("current top app is virtual or cloud")
			return true
		}
	}

	return false
}

func isPidVirtual(supApps []string, pid uint32) (bool, error) {
	//获取App二进制名称
	execPath, err := getActivePidInfo(pid)
	if err != nil {
		logger.Warning(err)
		return false, err
	}

	return isVirtual(execPath, supApps), nil
}

func (d *Daemon) IsPidVirtualMachine(pid uint32) (isVM bool, busErr *dbus.Error) {
	ret, err := isPidVirtual(supportedVirtualMachines, pid)
	if err != nil {
		return false, nil
	}
	logger.Info("IsPidVirtualMachine, ret:", ret)

	return ret, dbusutil.ToError(err)
}
