// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fprintd

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	huawei_fprint "github.com/linuxdeepin/go-dbus-factory/system/com.huawei.fingerprint"
	fprint "github.com/linuxdeepin/go-dbus-factory/system/net.reactivated.fprint"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
	polkit "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.policykit1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
	"golang.org/x/xerrors"
)

const (
	dbusServiceName     = "org.deepin.dde.Fprintd1"
	dbusPath            = "/org/deepin/dde/Fprintd1"
	dbusInterface       = dbusServiceName
	dbusDeviceInterface = dbusServiceName + ".Device"

	systemdDBusServiceName = "org.freedesktop.systemd1"
	systemdDBusPath        = "/org/freedesktop/systemd1"
	systemdDBusInterface   = systemdDBusServiceName + ".Manager"
)

//go:generate dbusutil-gen -type Manager -import github.com/godbus/dbus/v5 manager.go
//go:generate dbusutil-gen em -type Manager,HuaweiDevice,Device

type Manager struct {
	service       *dbusutil.Service
	sysSigLoop    *dbusutil.SignalLoop
	fprintManager fprint.Manager
	huaweiFprint  huawei_fprint.Fingerprint
	huaweiDevice  *HuaweiDevice
	dbusDaemon    ofdbus.DBus
	devices       Devices
	devicesMu     sync.Mutex
	fprintCh      chan struct{}

	PropsMu sync.RWMutex
	// dbusutil-gen: equal=nil
	Devices []dbus.ObjectPath
}

func newManager(service *dbusutil.Service) (*Manager, error) {
	systemConn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	return &Manager{
		service:       service,
		fprintManager: fprint.NewManager(systemConn),
		huaweiFprint:  huawei_fprint.NewFingerprint(systemConn),
		dbusDaemon:    ofdbus.NewDBus(systemConn),
		sysSigLoop:    dbusutil.NewSignalLoop(systemConn, 10),
	}, nil
}

func (m *Manager) getDefaultDeviceInter() (IDevice, error) {
	if m.huaweiDevice != nil {
		return m.huaweiDevice, nil
	}

	objPath, err := m.fprintManager.GetDefaultDevice(0)
	if err != nil {
		return nil, err
	}
	return m.devices.Get(objPath), nil
}

func (m *Manager) GetDefaultDevice() (device dbus.ObjectPath, busErr *dbus.Error) {
	err := m.refreshDeviceHuawei()
	if err != nil {
		logger.Warning(err)
	}

	if m.huaweiDevice != nil {
		return huaweiDevicePath, nil
	}

	objPath, err := m.fprintManager.GetDefaultDevice(0)
	if err != nil {
		logger.Debug("failed to get default device:", err)
		return "/", dbusutil.ToError(err)
	}
	m.addDevice(objPath)
	return convertFPrintPath(objPath), nil
}

func (m *Manager) GetDevices() (devices []dbus.ObjectPath, busErr *dbus.Error) {
	m.refreshDevices()
	m.PropsMu.Lock()
	devices = m.Devices
	m.PropsMu.Unlock()
	return devices, nil
}

func (m *Manager) refreshDevicesFprintd() error {
	devicePaths, err := m.fprintManager.GetDevices(0)
	if err != nil {
		return err
	}

	if m.huaweiDevice != nil {
		devicePaths = append(devicePaths, huaweiDevicePath)
	}

	var needDelete []dbus.ObjectPath
	var needAdd []dbus.ObjectPath

	m.devicesMu.Lock()

	// 在 m.devList 但不在 devicePaths 中的记录在 needDelete
	for _, d := range m.devices {
		found := false
		for _, devPath := range devicePaths {
			if d.getCorePath() == devPath {
				found = true
				break
			}
		}
		if !found {
			needDelete = append(needDelete, d.getCorePath())
		}
	}

	// 在 devicePaths 但不在 m.devList 中的记录在 needAdd
	for _, devPath := range devicePaths {
		found := false
		for _, d := range m.devices {
			if d.getCorePath() == devPath {
				found = true
				break
			}
		}
		if !found {
			needAdd = append(needAdd, devPath)
		}
	}

	for _, devPath := range needDelete {
		m.devices = m.devices.Delete(devPath)
	}
	for _, devPath := range needAdd {
		m.devices = m.devices.Add(devPath, m.service, m.sysSigLoop)
	}
	m.devicesMu.Unlock()

	m.updatePropDevices()
	return nil
}

func (m *Manager) refreshDeviceHuawei() error {
	if m.huaweiDevice != nil {
		return nil
	}

	has, err := m.hasHuaweiDevice()
	if err != nil {
		return err
	}

	if has {
		m.addHuaweiDevice()
		m.updatePropDevices()
	}
	return nil
}

func (m *Manager) refreshDevices() {
	err := m.refreshDeviceHuawei()
	if err != nil {
		logger.Warning(err)
	}
	err = m.refreshDevicesFprintd()
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) updatePropDevices() {
	m.devicesMu.Lock()
	paths := make([]dbus.ObjectPath, len(m.devices))
	for idx, d := range m.devices {
		paths[idx] = d.getPath()
	}
	m.devicesMu.Unlock()

	m.PropsMu.Lock()
	m.setPropDevices(paths)
	m.PropsMu.Unlock()
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) hasHuaweiDevice() (has bool, err error) {
	activatableNames, err := m.dbusDaemon.ListActivatableNames(0)
	if err != nil {
		return false, err
	}
	if !strv.Strv(activatableNames).Contains(m.huaweiFprint.ServiceName_()) {
		return false, nil
	}

	has, err = m.huaweiFprint.SearchDevice(0)
	return
}

func (m *Manager) init() {
	m.sysSigLoop.Start()
	m.fprintCh = make(chan struct{}, 1)
	m.listenDBusSignals()

	paths, err := m.fprintManager.GetDevices(0)
	if err != nil {
		logger.Warning("Failed to get fprint devices:", err)
		return
	}
	for _, devPath := range paths {
		m.addDevice(devPath)
	}
	m.updatePropDevices()
}

func (m *Manager) addHuaweiDevice() {
	logger.Debug("add huawei device")
	d := &HuaweiDevice{
		service:  m.service,
		core:     m.huaweiFprint,
		ScanType: "press",
	}

	// listen dbus signals
	m.huaweiFprint.InitSignalExt(m.sysSigLoop, true)
	_, err := m.huaweiFprint.ConnectEnrollStatus(func(progress int32, result int32) {
		d.handleSignalEnrollStatus(progress, result)
	})
	if err != nil {
		logger.Warning(err)
	}

	_, err = m.huaweiFprint.ConnectIdentifyStatus(func(result int32) {
		d.handleSignalIdentifyStatus(result)
	})
	if err != nil {
		logger.Warning(err)
	}

	err = m.service.Export(huaweiDevicePath, d)
	if err != nil {
		logger.Warning(err)
		return
	}

	m.huaweiDevice = d

	m.devicesMu.Lock()
	m.devices = append(m.devices, d)
	m.devicesMu.Unlock()
}

func (m *Manager) listenDBusSignals() {
	m.dbusDaemon.InitSignalExt(m.sysSigLoop, true)
	_, err := m.dbusDaemon.ConnectNameOwnerChanged(func(name string, oldOwner string, newOwner string) {
		fprintDBusServiceName := m.fprintManager.ServiceName_()
		if name == fprintDBusServiceName && newOwner != "" {
			select {
			case m.fprintCh <- struct{}{}:
			default:
			}
		}
		if newOwner == "" &&
			oldOwner != "" &&
			name == oldOwner &&
			strings.HasPrefix(name, ":") {
			// uniq name lost

			if m.huaweiDevice != nil {
				m.huaweiDevice.handleNameLost(name)
			}
		}
	})
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) addDevice(objPath dbus.ObjectPath) {
	logger.Debug("add device:", objPath)
	m.devicesMu.Lock()
	defer m.devicesMu.Unlock()

	d := m.devices.Get(objPath)
	if d != nil {
		return
	}
	m.devices = m.devices.Add(objPath, m.service, m.sysSigLoop)
}

func (m *Manager) destroy() {
	destroyDevices(m.devices)
	m.sysSigLoop.Stop()
}

var errAuthFailed = errors.New("authentication failed")

func checkAuth(actionId string, busName string) error {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	authority := polkit.NewAuthority(systemBus)
	subject := polkit.MakeSubject(polkit.SubjectKindSystemBusName)
	subject.SetDetail("name", busName)

	ret, err := authority.CheckAuthorization(0, subject,
		actionId, nil,
		polkit.CheckAuthorizationFlagsAllowUserInteraction, "")
	if err != nil {
		return err
	}

	if ret.IsAuthorized {
		return nil
	}
	return errAuthFailed
}

func (m *Manager) TriggerUDevEvent(sender dbus.Sender) *dbus.Error {
	uid, err := m.service.GetConnUID(string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	if uid != 0 {
		err = errors.New("not root user")
		return dbusutil.ToError(err)
	}

	logger.Debug("udev event")

	select {
	case <-m.fprintCh:
	default:
	}

	err = restartSystemdService("fprintd.service", "replace")
	if err != nil {
		return dbusutil.ToError(err)
	}

	select {
	case <-m.fprintCh:
		logger.Debug("fprintd started")
	case <-time.After(5 * time.Second):
		logger.Warning("wait fprintd restart timed out!")
	}

	err = m.refreshDevicesFprintd()
	if err != nil {
		return dbusutil.ToError(err)
	}
	return nil
}

func restartSystemdService(name, mode string) error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	obj := sysBus.Object(systemdDBusServiceName, systemdDBusPath)
	var jobPath dbus.ObjectPath
	err = obj.Call(systemdDBusInterface+".RestartUnit", dbus.FlagNoAutoStart, name, mode).Store(&jobPath)
	return err
}

func (m *Manager) PreAuthEnroll(sender dbus.Sender) *dbus.Error {
	logger.Debug("PreAuthEnroll sender:", sender)
	err := checkAuth(actionIdEnroll, string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}

	var dev IDevice
	dev, err = m.getDefaultDeviceInter()
	if err != nil {
		return dbusutil.ToError(xerrors.Errorf("failed to get default device: %w", err))
	}
	if dev == nil {
		logger.Warning("PreAuthEnroll dev is nil")
		return nil
	}

	// wait device free, timeout 4s
	for i := 0; i < 20; i++ {
		free, err := dev.isFree()
		if err != nil {
			logger.Warning("dev isFree err:", err)
			return dbusutil.ToError(err)
		}
		if free {
			logger.Debug("PreAuthEnroll default device is free now")
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	logger.Warning("wait for the default device to become free timed out")

	return nil
}
