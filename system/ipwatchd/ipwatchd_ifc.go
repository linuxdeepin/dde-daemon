package ipwatchd

import (
	"fmt"
	"net"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	dbusServiceName = "com.deepin.system.IPWatchD"
	dbusPath        = "/com/deepin/system/IPWatchD"
	dbusInterface   = dbusServiceName
)

func (i *IPWatchD) GetInterfaceName() string {
	return dbusInterface
}

func (i *IPWatchD) RequestIPConflictCheck(ip, ifc string) (mac string, busErr *dbus.Error) {
	ip4 := net.ParseIP(ip).To4()
	if ip4 == nil {
		return "", dbusutil.ToError(fmt.Errorf("invalid ip"))
	}

	var dev *device
	if len(ifc) == 0 {
		ifc = "unknown"
		dev = i.findDeviceByIP(ip4)
	} else {
		dev = i.findDevice(ifc)
	}
	if dev == nil {
		return "", dbusutil.ToError(fmt.Errorf("can't find device with %s", ifc))
	}

	err := i.addTask(ip4, dev)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	// 直接返回空，通过信号通知检测结果
	return "", nil
}
