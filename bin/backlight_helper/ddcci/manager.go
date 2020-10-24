package ddcci

import (
	"fmt"
	"runtime"
	"strings"
	"sync"

	x "github.com/linuxdeepin/go-x11-client"
	dbus "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
)

const (
	DbusPath      = "/com/deepin/daemon/helper/Backlight/DDCCI"
	dbusInterface = "com.deepin.daemon.helper.Backlight.DDCCI"
)

var logger = log.NewLogger("backlight_helper/ddcci")

type Manager struct {
	service *dbusutil.Service
	ddcci   *ddcci

	PropsMu         sync.RWMutex
	configTimestamp x.Timestamp

	methods *struct {
		CheckSupport    func() `in:"edidBase64" out:"support"`
		GetBrightness   func() `in:"edidBase64" out:"value"`
		SetBrightness   func() `in:"edidBase64,value"`
		RefreshDisplays func()
	}
}

func NewManager(service *dbusutil.Service) (*Manager, error) {
	m := &Manager{}
	m.service = service

	// 在 arm 和 mips 架构下，调用 i2c 的接口会导致待机后无法唤醒，
	// 所以不在 arm 和 mips 架构下使用 DDC/CI 功能。
	if !strings.HasPrefix(runtime.GOARCH, "mips") {
		var err error
		m.ddcci, err = newDDCCI()
		if err != nil {
			return nil, fmt.Errorf("brightness: failed to init ddc/ci: %s", err)
		}
	}

	return m, nil
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) CheckSupport(edidBase64 string) (bool, *dbus.Error) {
	if m.ddcci == nil {
		return false, nil
	}

	return m.ddcci.SupportBrightness(edidBase64), nil
}

func (m *Manager) GetBrightness(edidBase64 string) (int32, *dbus.Error) {
	if m.ddcci == nil {
		return 0, nil
	}

	if !m.ddcci.SupportBrightness(edidBase64) {
		err := fmt.Errorf("brightness: not support ddc/ci: %s", edidBase64)
		return 0, dbusutil.ToError(err)
	}

	brightness, err := m.ddcci.GetBrightness(edidBase64)
	return int32(brightness), dbusutil.ToError(err)
}

func (m *Manager) SetBrightness(edidBase64 string, value int32) *dbus.Error {
	if m.ddcci == nil {
		return nil
	}

	if !m.ddcci.SupportBrightness(edidBase64) {
		err := fmt.Errorf("brightness: not support ddc/ci: %s", edidBase64)
		return dbusutil.ToError(err)
	}

	err := m.ddcci.SetBrightness(edidBase64, int(value))
	return dbusutil.ToError(err)
}

func (m *Manager) RefreshDisplays() *dbus.Error {
	//ddcutil代码有问题，无法刷新，暂用退出的方式来刷新显示器列表
	m.service.Quit()
	return nil
	//if m.ddcci == nil {
	//	return nil
	//}
	//
	//err := m.ddcci.RefreshDisplays()
	//return dbusutil.ToError(err)
}
