// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"runtime"
	"github.com/linuxdeepin/go-lib/strv"
	"github.com/linuxdeepin/go-lib/dbusutil"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

const (
	globalCpuDirPath                = "/sys/devices/system/cpu"
	globalCpuFreqDirName            = "cpufreq"
	globalGovernorFileName          = "scaling_governor"
	globalAvailableGovernorFileName = "scaling_available_governors"
	globalBoostFilePath             = "/sys/devices/system/cpu/cpufreq/boost"
	globalDefaultPath               = "/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor"
	_isHuaWei                       = `dmidecode -t 1 | awk "/Product Name:/{print $NF}" | cut -d ":" -f 2`
)

type CpuHandler struct {
	path     string
	governor string
}

type CpuHandlers []CpuHandler

//内核文档：https://docs.kernel.org/admin-guide/pm/cpufreq.html#generic-scaling-governors
var _scalingAvailableGovernors = []string {"performance", "powersave", "userspace", "ondemand", "conservative", "schedutil"}
var _scalingBalanceAvailableGovernors = []string {"ondemand", "conservative", "schedutil", "performance"}
var _supportGovernors []string
var _localAvailableGovernors []string
var _supportNormalBalanceArch = []string{"amd64", "x86_64", "amd", "x86", "386"}
var _useNormalBalance bool
var _supportHuaweiType = []string{"KLVU", "KLVV", "PGUV", "PGUW", "klvu", "klvv", "pguv", "pguw"}

func getScalingAvailableGovernors() []string {
	return _scalingAvailableGovernors
}

func getScalingBalanceAvailableGovernors() []string {
	return _scalingBalanceAvailableGovernors
}

func getSupportGovernors() []string {
	return _supportGovernors
}

func getLocalAvailableGovernors() []string {
	return _localAvailableGovernors
}

func getUseNormalBalance() bool {
	return _useNormalBalance
}

func setUseNormalBalance(value bool) {
	if _useNormalBalance != value {
		_useNormalBalance = value
	}
}

func setLocalAvailableGovernors(value []string) []string {
	_localAvailableGovernors = value
	return _localAvailableGovernors
}

func setSupportGovernors(value []string) []string {
	_supportGovernors = value
	return _supportGovernors
}

func getIsBalanceSupported() bool {
	for _, balance := range getScalingBalanceAvailableGovernors() {
		if strv.Strv(getSupportGovernors()).Contains(balance) {
			return true
		}
	}
	return false
}

func getIsHighPerformanceSupported() bool {
	cpus := CpuHandlers{}
	return  cpus.IsBoostFileExist() && strv.Strv(getSupportGovernors()).Contains("performance")
}

func getIsPowerSaveSupported() bool {
	return strv.Strv(getSupportGovernors()).Contains("powersave")
}

func isSystemSupportMode(mode string) bool {
	return strv.Strv(_scalingBalanceAvailableGovernors).Contains(mode) && strv.Strv(_supportGovernors).Contains(mode)
}

func trySetBalanceCpuGovernor(balanceScalingGovernor string) (error, string) {
	if "" == balanceScalingGovernor {
		return nil, ""
	}

	// dconfig设置的模式如果不是平衡模式类型，则不进行设置
	if !strv.Strv(getScalingBalanceAvailableGovernors()).Contains(balanceScalingGovernor) {
		logger.Warningf("[trySetBalanceCpuGovernor] The governor is invalid. %s ", balanceScalingGovernor)
		return dbusutil.ToError(fmt.Errorf(" The Governor is invalid. Please use performance, powersave, userspace, ondemand, conservative or schedutil. current value : %s", balanceScalingGovernor)), ""
	}

	// 系统支持的模式不包含设置的平衡模式，则按照产品优先级设置平衡模式 : ondemand > conservative > schedutil > performance
	// 系统支持的模式则可以直接进行设置
	if !strv.Strv(getSupportGovernors()).Contains(balanceScalingGovernor) {
		for _, available := range getScalingBalanceAvailableGovernors() {
			if strv.Strv(getSupportGovernors()).Contains(available) {
				logger.Infof(" [trySetBalanceCpuGovernor] use other support governor. %s", available)
				balanceScalingGovernor = available
				break
			}
		}
	}

	logger.Info("[trySetBalanceCpuGovernor] balanceScalingGovernor : ", balanceScalingGovernor)
	return nil, balanceScalingGovernor
}

// true : 平衡模式使用 ondemand > conservative > schedutil > performance
// false: 平衡模式使用 performance
func useNormalBalance() (ret bool) {
	goArch := runtime.GOARCH
	for _, value := range _supportNormalBalanceArch {
		if strings.Contains(goArch, value) {
			ret = true
			return ret
		}
	}

	if dutils.IsFileExist(_configHwSystem) && isHuaWei() {
		ret = true
	}

	return ret
}

func isHuaWei() bool {
	out, err := exec.Command("/bin/sh", "-c", _isHuaWei).CombinedOutput()
	if err != nil {
		logger.Warning("[isHuawei] err : ", err, out)
		return false
	}
	execRet := string(out)
	logger.Info("[isHuawei] out : ", execRet)
	for _, value := range _supportHuaweiType {
		if strings.Contains(execRet, value) {
			logger.Info("[isHuawei] The computer is HuaWei.")
			return true
		}
	}
	return false
}

func NewCpuHandlers() *CpuHandlers {
	cpus := make(CpuHandlers, 0)
	dirs, err := ioutil.ReadDir(globalCpuDirPath)
	if err != nil {
		logger.Warning(err)
		return &cpus
	}

	pattern, _ := regexp.Compile(`cpu[0-9]+`)
	for _, dir := range dirs {
		dirNane := dir.Name()
		cpuPath := filepath.Join(globalCpuDirPath, dirNane)
		isMatch := pattern.MatchString(dirNane)
		if isMatch {
			logger.Debugf("append %s", cpuPath)
			freqPath := filepath.Join(cpuPath, globalCpuFreqDirName)
			cpu := CpuHandler{
				path: freqPath,
			}
			_, err = cpu.GetGovernor(true)
			if err != nil {
				logger.Warning(err)
			}
			cpus = append(cpus, cpu)
		} else {
			logger.Debugf("skip %s", cpuPath)
		}
	}

	logger.Debugf("total %d cpus", len(cpus))
	return &cpus
}

func (cpu *CpuHandler) GetGovernor(force bool) (string, error) {
	if force {
		data, err := ioutil.ReadFile(filepath.Join(cpu.path, globalGovernorFileName))
		if err != nil {
			logger.Warning(err)
			return "", err
		}
		cpu.governor = strings.TrimSpace(string(data))
	}

	return cpu.governor, nil
}

func (cpu *CpuHandler) SetGovernor(governor string) error {
	err := ioutil.WriteFile(filepath.Join(cpu.path, globalGovernorFileName), []byte(governor), 0644)
	if err != nil {
		logger.Warning(err)

	}
	return err
}

func (cpus *CpuHandlers) GetGovernor() (string, error) {
	if len(*cpus) < 1 {
		return "", fmt.Errorf("cannot find cpu files")
	}

	// 理论上应该都是一样的，但是这里确认一遍
	governor, _ := (*cpus)[0].GetGovernor(true)
	for _, cpu := range *cpus {
		temp, _ := cpu.GetGovernor(true)
		if governor != temp {
			logger.Warning("governors are not same")
			return "", fmt.Errorf("governors are not same")
		}
	}
	return governor, nil
}

// 获取可用的Governor字符串
func (cpus *CpuHandlers) getAvailableGovernors() (ret string, err error) {
	for i, cpu := range *cpus {
		data, err := ioutil.ReadFile(filepath.Join(cpu.path, globalAvailableGovernorFileName))
		if err != nil {
			logger.Warning(err)
			return  ret, err
		}
		path := string(data)
		if ret != path {
			ret = path
			if i != 0 {
				logger.Warning(err)
				return path, dbusutil.ToError(fmt.Errorf("The cpu path not equal. path : %s ", path))
			}
		}
	}

	return ret, err
}

// 获取可用的Governor数组
func (cpus *CpuHandlers) getAvailableArrGovernors() []string {
	value, err := cpus.getAvailableGovernors()
	if err != nil {
		logger.Warning(err)
		return nil
	}
	return strings.Split(strings.TrimSpace(string(value)), " ")
}

func (cpus *CpuHandlers) getCpuGovernorPath() string {
	if len(*cpus) <= 0 {
		return ""
	}

	if !dutils.IsFileExist((*cpus)[0].path) {
		return ""
	}
	path := filepath.Join((*cpus)[0].path, globalGovernorFileName)
	if "" == path {
		path = globalDefaultPath
	}

	if !dutils.IsFileExist(path) {
		path = ""
	}

	return path
}

// 通过写文件的返回情况，获取非scaling_available_governors的值是否支持
func (cpus *CpuHandlers) tryWriteGovernor(lines []string) []string {
	path := cpus.getCpuGovernorPath()
	if "" == path {
		return nil
	}

	//获取当前系统的governor值
	oldGovernor, err := ioutil.ReadFile(path)
	if err != nil {
		return lines
	}
	logger.Infof(" [tryWriteGovernor] Test Path : %s.", path)
	// 遍历6个内核支持的数据，以及系统支持的数据
	// 通过是否能写入文件判断不包含在supportGovernors的值是否支持设置
	for _, value := range getScalingAvailableGovernors() {
		if !strv.Strv(lines).Contains(value) {
			err = ioutil.WriteFile(path, []byte(value), 0644)
			logger.Infof(" Not contain in supportGovernors. Governor : %s.", value)
			if err != nil {
				logger.Infof(" Can't use the governor : %s. err : %s", value, err)
			} else {
				lines = append(lines, value)
			}
		}
	}
	//将当前系统原始的governor值设置回去
	err = ioutil.WriteFile(path, oldGovernor, 0644)
	if err != nil {
		logger.Warning(err)
	}
	return lines
}

func (cpus *CpuHandlers) SetGovernor(governor string) error {
	logger.Info(" SetGovernor governor ： ", governor)
	for _, cpu := range *cpus {
		err := cpu.SetGovernor(governor)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cpus *CpuHandlers) IsBoostFileExist() bool {
	_, err := os.Lstat(globalBoostFilePath)
	return err == nil
}

func (cpus *CpuHandlers) SetBoostEnabled(enabled bool) error {
	var err error
	if enabled {
		err = ioutil.WriteFile(globalBoostFilePath, []byte("1"), 0644)
	} else {
		err = ioutil.WriteFile(globalBoostFilePath, []byte("0"), 0644)
	}
	return err
}

func (cpus *CpuHandlers) GetBoostEnabled() (bool, error) {
	data, err := ioutil.ReadFile(globalBoostFilePath)
	return strings.TrimSpace(string(data)) == "0", err
}
