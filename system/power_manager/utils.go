package power_manager

import (
	"io/ioutil"
	"strings"
)

const (
	fileMemSleep = "/sys/power/mem_sleep"
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
