// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package soundeffect

import (
	"errors"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/soundutils"
	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	soundthemeplayer "github.com/linuxdeepin/go-dbus-factory/system/org.deepin.dde.soundthemeplayer1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/strv"
)

//go:generate dbusutil-gen em -type Manager

const (
	dconfigAppearanceAppId  = "org.deepin.dde.appearance"
	dconfigAppearanceFileId = dconfigAppearanceAppId
	dconfigKeySoundName     = "Sound_Theme"

	dconfigappID         = "org.deepin.dde.daemon"
	dconfigSoundEffectId = "org.deepin.dde.daemon.soundeffect"
	dconfigKeyEnabled    = "enabled"

	DBusServiceName        = "org.deepin.dde.SoundEffect1"
	dbusPath               = "/org/deepin/dde/SoundEffect1"
	dbusInterface          = DBusServiceName
	allowPlaySoundMaxCount = 3
)

type Manager struct {
	service              *dbusutil.Service
	soundeffectConfigMgr *dconfig.DConfig
	appearanceConfigMgr  *dconfig.DConfig
	count                int
	countMu              sync.Mutex
	names                strv.Strv

	Enabled dconfig.Bool `prop:"access:rw"`
}

func NewManager(service *dbusutil.Service) *Manager {
	var m = new(Manager)

	m.service = service
	return m
}

func (m *Manager) init() error {
	var err error
	m.soundeffectConfigMgr, err = dconfig.NewDConfig(dconfigappID, dconfigSoundEffectId, "")
	if err != nil {
		return err
	}

	m.appearanceConfigMgr, err = dconfig.NewDConfig(dconfigAppearanceAppId, dconfigAppearanceFileId, "")
	if err != nil {
		return err
	}
	m.Enabled.Bind(m.soundeffectConfigMgr, dconfigKeyEnabled)

	m.names = m.getSoundNames()
	logger.Debug(m.names)
	return nil
}

func (m *Manager) getSoundNames() []string {
	keys, err := m.soundeffectConfigMgr.ListKeys()
	if err != nil {
		logger.Warning(err)
	}
	return keys
}

func (*Manager) GetInterfaceName() string {
	return dbusInterface
}

// deprecated
func (m *Manager) PlaySystemSound(name string) *dbus.Error {
	return m.PlaySound(name)
}

func (m *Manager) PlaySound(name string) *dbus.Error {
	m.service.DelayAutoQuit()

	if name == "" {
		return nil
	}

	go func() {
		m.countMu.Lock()
		m.count++
		logger.Debug("start", m.count)
		m.countMu.Unlock()

		if m.count <= allowPlaySoundMaxCount {
			err := soundutils.PlaySystemSound(name, "")
			if err != nil {
				logger.Error(err)
			}
		} else {
			logger.Warning("PlaySystemSound thread more than 3")
		}

		m.countMu.Lock()
		logger.Debug("end", m.count)
		m.count--
		m.countMu.Unlock()
	}()
	return nil
}

// deprecated
func (m *Manager) GetSystemSoundFile(name string) (file string, busErr *dbus.Error) {
	return m.GetSoundFile(name)
}

func (m *Manager) GetSoundFile(name string) (file string, busErr *dbus.Error) {
	m.service.DelayAutoQuit()

	file = soundutils.GetSystemSoundFile(name)
	if file == "" {
		return "", dbusutil.ToError(errors.New("sound file not found"))
	}

	return file, nil
}

func (m *Manager) canQuit() bool {
	m.countMu.Lock()
	count := m.count
	m.countMu.Unlock()
	return count == 0
}

func (m *Manager) syncConfigToSoundThemePlayer(name string, enabled bool) error {
	logger.Debug("sync config to sound-theme-player", name, enabled)
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	player := soundthemeplayer.NewSoundThemePlayer(sysBus)
	err = player.EnableSound(0, name, enabled)
	if err != nil {
		return err
	}
	theme, err := m.appearanceConfigMgr.GetValueString(dconfigKeySoundName)
	if err != nil {
		logger.Warning(err)
	}
	err = player.SetSoundTheme(0, theme)
	return err
}

func (m *Manager) EnableSound(name string, enabled bool) *dbus.Error {
	if !m.names.Contains(name) {
		return dbusutil.ToError(errors.New("invalid sound event"))
	}

	if name == soundutils.EventDesktopLogin ||
		name == soundutils.EventSystemShutdown {
		err := m.syncConfigToSoundThemePlayer(name, enabled)
		if err != nil {
			return dbusutil.ToError(err)
		}
	}

	err := m.soundeffectConfigMgr.SetValue(name, enabled)
	if err != nil {
		return dbusutil.ToError(err)
	}
	return nil
}

func (m *Manager) IsSoundEnabled(name string) (enabled bool, busErr *dbus.Error) {
	if !m.names.Contains(name) {
		return false, dbusutil.ToError(errors.New("invalid sound event"))
	}

	enabled, err := m.soundeffectConfigMgr.GetValueBool(name)
	if err != nil {
		return false, dbusutil.ToError(err)
	}

	return enabled, nil
}

func (m *Manager) GetSoundEnabledMap() (result map[string]bool, busErr *dbus.Error) {
	result = make(map[string]bool, len(m.names))
	for _, name := range m.names {
		enabled, err := m.soundeffectConfigMgr.GetValueBool(name)
		if err != nil {
			result[name] = false
			continue
		}

		result[name] = enabled
	}
	return result, nil
}

func (m *Manager) enabledWriteCb(write *dbusutil.PropertyWrite) *dbus.Error {
	enabled := write.Value.(bool)

	// NOTE: 已经约定 name 为空表示控制总开关
	err := m.syncConfigToSoundThemePlayer("", enabled)
	if err != nil {
		logger.Warning(err)
	}
	return nil
}
