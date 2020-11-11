package power_manager

import (
	"io/ioutil"
	"strconv"
	"strings"
)

const (
	fileMemSleep  = "/sys/power/mem_sleep"
	fileImageSize = "/sys/power/image_size"
	fileSwaps     = "/proc/swaps"
)

func canSuspend() bool {
	// TODO: 这种判断方式只能作为当前的临时方案使用，不是一个标准的判断是否支持待机的方法，
	// 等到内核对systemd的login中判断是否能待机的DBus接口(服务名 org.freedesktop.login1，
	// 对象 /org/freedesktop/login1，接口 org.freedesktop.login1.Manager 方法 CanSuspend)
	// 支持完善以后就要移除这部分逻辑。
	data, err := ioutil.ReadFile(fileMemSleep)
	if err != nil {
		logger.Warningf("read %s failed: %v", fileMemSleep, err)
		return false
	}
	if !strings.Contains(string(data), "deep") {
		logger.Debugf("can not find 'deep' in %s", fileMemSleep)
		return false
	}

	return true
}

func canHibernate() bool {
	// TODO: 需要考虑是否需要这些对于内存和swap的判断。
	data, err := ioutil.ReadFile(fileImageSize)
	if err != nil {
		logger.Warningf("read %s failed: %v", fileImageSize, err)
		return false
	}

	imageSize, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		logger.Warningf("read image size err: %v", err)
		return false
	}

	data, err = ioutil.ReadFile(fileSwaps)
	if err != nil {
		logger.Warningf("read %s failed: %v", fileSwaps, err)
		return false
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[1] != "partition" {
			continue
		}

		swapSize, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		if swapSize*1024 >= imageSize {
			return true
		}
		logger.Debugf("swap-partition(%s) smaller then image size", fields[0])
	}

	logger.Debug("do not support suspend")
	return false
}
