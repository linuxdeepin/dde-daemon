// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package systeminfo

import (
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/loader"
	systeminfo "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.systeminfo1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

//go:generate dbusutil-gen em -type SystemInfo

const (
	dbusServiceName = "org.deepin.dde.SystemInfo1"
	dbusPath        = "/org/deepin/dde/SystemInfo1"
	dbusInterface   = dbusServiceName
)

type SystemInfo struct {
	// Current deepin version, ex: "2015 Desktop"
	Version string
	// Distribution ID
	DistroID string
	// Distribution Description
	DistroDesc string
	// Distribution Version
	DistroVer string
	// CPU information
	Processor string
	// Disk capacity
	DiskCap uint64
	// Memory size
	MemoryCap uint64
	// System type: 32bit or 64bit
	SystemType int64
	// CPU max MHz
	CPUMaxMHz float64
	//Current Speed : when cpu max mhz is 0 use
	CurrentSpeed uint64
}

type Daemon struct {
	info          *SystemInfo
	PropsMu       sync.RWMutex
	systeminfo    systeminfo.SystemInfo
	sigSystemLoop *dbusutil.SignalLoop
	*loader.ModuleBase
}

var logger = log.NewLogger("daemon/systeminfo")

func NewDaemon(logger *log.Logger) *Daemon {
	daemon := new(Daemon)
	daemon.ModuleBase = loader.NewModuleBase("systeminfo", daemon, logger)
	return daemon
}

func (d *Daemon) GetDependencies() []string {
	return []string{}
}

func (d *Daemon) Start() error {
	if d.info != nil {
		return nil
	}
	service := loader.GetService()

	d.info = NewSystemInfo()
	d.initSysSystemInfo()
	err := service.Export(dbusPath, d.info)
	if err != nil {
		d.info = nil
		logger.Error(err)
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		d.info = nil
		logger.Error(err)
		return err
	}
	return nil
}

func (d *Daemon) Stop() error {
	if d.info == nil {
		return nil
	}

	service := loader.GetService()
	_ = service.StopExport(d.info)
	d.info = nil

	return nil
}

func (d *Daemon) initSysSystemInfo() {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return
	}
	d.systeminfo = systeminfo.NewSystemInfo(sysBus)
	d.sigSystemLoop = dbusutil.NewSignalLoop(sysBus, 10)
	d.sigSystemLoop.Start()
	d.systeminfo.InitSignalExt(d.sigSystemLoop, true)

	// 通过 demicode 获取 "CPU 频率", 接收 org.deepin.dde.SystemInfo1 的属性 CurrentSpeed 改变信号
	err = d.systeminfo.CurrentSpeed().ConnectChanged(func(hasValue bool, value uint64) {
		logger.Infof("demicode hasValue : %t, CurrentSpeed : %d", hasValue, value)
		if !hasValue {
			return
		}
		d.PropsMu.Lock()
		d.info.CurrentSpeed = value
		//假如此时 cpu max mhz 还是0, 且 value 不是0, 则给 d.infoCPUMaxMHz 再赋值
		if isFloatEqual(d.info.CPUMaxMHz, 0.0) && value != 0 {
			d.info.CPUMaxMHz = float64(value)
		}
		d.PropsMu.Unlock()
	})

	if err != nil {
		logger.Warning("systeminfo.CurrentSpeed().ConnectChanged err : ", err)
	}

	d.PropsMu.Lock()
	d.info.CurrentSpeed, err = d.systeminfo.CurrentSpeed().Get(0)
	if err != nil {
		logger.Warning("get systeminfo.CurrentSpeed err : ", err)
		d.PropsMu.Unlock()
		return
	}
	d.info.CPUMaxMHz = float64(d.info.CurrentSpeed)
	d.PropsMu.Unlock()
	logger.Info("d.info.CurrentSpeed : ", d.info.CurrentSpeed, " , d.info.CPUMaxMHz : ", d.info.CPUMaxMHz)
}

func NewSystemInfo() *SystemInfo {
	var info SystemInfo
	tmp, _ := doReadCache(cacheFile)
	if tmp != nil && tmp.isValidity() {
		info = *tmp
		time.AfterFunc(time.Second*10, func() {
			info.init()
			_ = doSaveCache(&info, cacheFile)
		})
		return &info
	}

	info.init()
	go func() {
		_ = doSaveCache(&info, cacheFile)
	}()
	return &info
}

func (info *SystemInfo) init() {
	var err error
	info.Processor, err = GetCPUInfo("/proc/cpuinfo")
	if err != nil {
		logger.Warning("Get cpu info failed:", err)
	}

	info.Version, err = getVersion()
	if err != nil {
		logger.Warning("Get version failed:", err)
	}

	info.DistroID, info.DistroDesc, info.DistroVer, err = getDistro()
	if err != nil {
		logger.Warning("Get distribution failed:", err)
	}

	info.MemoryCap, err = getMemoryFromFile("/proc/meminfo")
	if err != nil {
		logger.Warning("Get memory capacity failed:", err)
	}

	if systemBit() == "64" {
		info.SystemType = 64
	} else {
		info.SystemType = 32
	}

	info.DiskCap, err = getDiskCap()
	if err != nil {
		logger.Warning("Get disk capacity failed:", err)
	}

	lscpuRes, err := runLscpu()
	if err != nil {
		logger.Warning("run lscpu failed:", err)
		return
	} else {
		info.CPUMaxMHz, err = getCPUMaxMHzByLscpu(lscpuRes)
		if err != nil {
			logger.Warning(err)
		} else {
			if isFloatEqual(info.CPUMaxMHz, 0.0) {
				//关联信号,接收system的信号 : line139
				//此时若info.CurrentSpeed不为0, 则可以直接使用备用的currentspeed赋值
				if info.CurrentSpeed != 0 {
					info.CPUMaxMHz = float64(info.CurrentSpeed)
				}
			}
		}

		if info.Processor == "" {
			info.Processor, err = getProcessorByLscpu(lscpuRes)
			if err != nil {
				logger.Warning(err)
			}
		}
	}
}

func (info *SystemInfo) isValidity() bool {
	if info.Processor == "" || info.DiskCap == 0 || info.MemoryCap == 0 {
		return false
	}
	return true
}

func (*SystemInfo) GetInterfaceName() string {
	return dbusInterface
}
