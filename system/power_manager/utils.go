package power_manager

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

const (
	fileMemSleep = "/sys/power/mem_sleep"
	// 0  　混合休眠功能禁止
	// 1　  混合休眠功能打开
	// 2    此设备不支持混合休眠
	fileS3ToS4 = "/sys/power/s3_to_s4_enable"
	// 混合休眠进入S3多长时间转s4　单位分钟
	fileS3TimeOut = "/sys/power/s3_timeout"
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

func canSuspendToHibernate() bool {
	// 先判断s3转s4的"/sys/power/s3_timeout"文件存在不，不存在就不支持
	_, err := ioutil.ReadFile(fileS3TimeOut)
	if err != nil {
		logger.Warningf("read %s failed: %v", fileS3TimeOut, err)
		return false
	}

	data, err := ioutil.ReadFile(fileS3ToS4)
	if err != nil {
		logger.Warningf("read %s failed: %v", fileS3ToS4, err)
		return false
	}

	s3ToS4EnableValue, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		logger.Warningf("read image size err: %v", err)
		return false
	}

	if s3ToS4EnableValue > 2 || s3ToS4EnableValue < 0 {
		logger.Warningf("s3ToS4EnableValue out of range: %v", s3ToS4EnableValue)
		return false
	}

	if s3ToS4EnableValue == 2 {
		return false
	}

	return true
}

func setSuspendToHibernateTime(timeMinute int32) error {

	fS3ToS4, err := os.OpenFile(fileS3ToS4, os.O_WRONLY, 0666)
	if err != nil {
		logger.Warningf("OpenFile %s failed: %v", fileS3ToS4, err)
		return err
	}
	defer fS3ToS4.Close()

	var s3ToS4EnableValue int32
	if timeMinute <= 0 {
		s3ToS4EnableValue = 0
	} else {
		s3ToS4EnableValue = 1
	}
	_, err = fS3ToS4.WriteString(strconv.Itoa(int(s3ToS4EnableValue)))
	if err != nil {
		logger.Warningf("WriteString to %s err: %v", fileS3ToS4, err)
		return err
	}
	// 满足内核要求，时间<=0不设置t3转t4时间
	if s3ToS4EnableValue == 0 {
		return nil
	}
	// 先判断s3转s4的"/sys/power/s3_timeout"文件存在不，不存在就不支持
	fS3TimeOut, err := os.OpenFile(fileS3TimeOut, os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		logger.Warningf("read %s failed: %v", fileS3TimeOut, err)
		return err
	}

	defer fS3TimeOut.Close()

	_, err = fS3TimeOut.WriteString(strconv.Itoa(int(timeMinute)))
	if err != nil {
		logger.Warningf("WriteString to %s err: %v", fileS3TimeOut, err)
		return err
	}

	return nil
}
