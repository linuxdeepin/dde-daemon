package power

import (
	"github.com/linuxdeepin/go-lib/dbusutil/gsprop"
)

const (
	dbusServiceNameV20 = "com.deepin.daemon.Power"
	dbusPathV20        = "/com/deepin/daemon/Power"
	dbusInterfaceV20   = dbusServiceNameV20
)

type ManagerV20 struct {
	manager *Manager

	// 睡眠前是否锁定
	SleepLock gsprop.Bool `prop:"access:rw"`
}

func NewManagerV20(manager *Manager) *ManagerV20 {
	managerV20 := &ManagerV20{
		manager: manager,
	}

	managerV20.SleepLock.Bind(managerV20.manager.settings, settingKeySleepLock)

	return managerV20
}

func (manager *ManagerV20) GetInterfaceName() string {
	return dbusInterfaceV20
}
