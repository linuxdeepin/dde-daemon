// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package launcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/dde-api/soundutils"
)

const (
	dbusServiceName    = "com.deepin.dde.daemon.Launcher"
	dbusObjPath        = "/com/deepin/dde/daemon/Launcher"
	dbusInterface      = dbusServiceName
	desktopMainSection = "Desktop Entry"
	launcherExecPath   = "/usr/bin/dde-launcher"
)

var errorInvalidID = errors.New("invalid ID")

func (m *Manager) GetInterfaceName() string {
	return dbusInterface
}

func (m *Manager) GetAllItemInfos() (list []ItemInfo, busErr *dbus.Error) {
	list = make([]ItemInfo, 0, len(m.items))
	for _, item := range m.items {
		list = append(list, item.newItemInfo())
	}
	logger.Debug("GetAllItemInfos list length:", len(list))
	return list, nil
}

func (m *Manager) GetItemInfo(id string) (itemInfo ItemInfo, busErr *dbus.Error) {
	item := m.getItemById(id)
	if item == nil {
		return ItemInfo{}, dbusutil.ToError(errorInvalidID)
	}
	return item.newItemInfo(), nil
}

func (m *Manager) GetAllNewInstalledApps() (apps []string, busErr *dbus.Error) {
	newApps, err := m.appsObj.LaunchedRecorder().GetNew(0)
	if err != nil {
		return nil, dbusutil.ToError(err)
	}
	// newApps type is map[string][]string
	for dir, names := range newApps {
		for _, name := range names {
			path := filepath.Join(dir, name) + desktopExt
			if item := m.getItemByPath(path); item != nil {
				apps = append(apps, item.ID)
			}
		}
	}
	return apps, nil
}

func (m *Manager) IsItemOnDesktop(id string) (result bool, busErr *dbus.Error) {
	item := m.getItemById(id)
	if item == nil {
		return false, dbusutil.ToError(errorInvalidID)
	}
	file := appInDesktop(m.getAppIdByFilePath(item.Path))
	if _, err := os.Stat(file); err != nil {
		if os.IsNotExist(err) {
			// not exist
			return false, nil
		} else {
			return false, dbusutil.ToError(err)
		}
	} else {
		// exist
		return true, nil
	}
}

func (m *Manager) RequestRemoveFromDesktop(id string) (ok bool, busErr *dbus.Error) {
	item := m.getItemById(id)
	if item == nil {
		return false, dbusutil.ToError(errorInvalidID)
	}
	file := appInDesktop(m.getAppIdByFilePath(item.Path))
	err := os.Remove(file)
	return err == nil, dbusutil.ToError(err)
}

func (m *Manager) RequestSendToDesktop(id string) (ok bool, busErr *dbus.Error) {
	item := m.getItemById(id)
	if item == nil {
		return false, dbusutil.ToError(errorInvalidID)
	}
	dest := appInDesktop(m.getAppIdByFilePath(item.Path))
	_, err := os.Stat(dest)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, dbusutil.ToError(err)
		}
		// dest file not exist
	} else {
		// dest file exist
		return false, dbusutil.ToError(os.ErrExist)
	}

	kf := keyfile.NewKeyFile()
	if err := kf.LoadFromFile(item.Path); err != nil {
		logger.Warning(err)
		return false, dbusutil.ToError(err)
	}
	kf.SetString(desktopMainSection, "X-Deepin-CreatedBy", dbusServiceName)
	kf.SetString(desktopMainSection, "X-Deepin-AppID", id)
	// Desktop files in user desktop directory do not require executable permission
	if err := kf.SaveToFile(dest); err != nil {
		logger.Warning("save new desktop file failed:", err)
		return false, dbusutil.ToError(err)
	}
	// success
	go func() {
		err := soundutils.PlaySystemSound(soundutils.EventIconToDesktop, "")
		if err != nil {
			logger.Warning("playSystemSound Failed", err)
		}
	}()
	return true, nil
}

// MarkLaunched 废弃
func (m *Manager) MarkLaunched(id string) *dbus.Error {
	return nil
}

// purge is useless
func (m *Manager) RequestUninstall(sender dbus.Sender, id string, purge bool) *dbus.Error {
	logger.Infof("RequestUninstall sender: %q id: %q", sender, id)

	execPath, err := m.getExecutablePath(sender)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	if execPath != launcherExecPath {
		err = fmt.Errorf("%q is not allowed to uninstall packages", execPath)
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	go func() {
		err = m.uninstall(id)
		if err != nil {
			logger.Warningf("uninstall %q failed: %v", id, err)
			err := m.service.Emit(m, "UninstallFailed", id, err.Error())
			if err != nil {
				logger.Warning("emit UninstallFailed Failed:", err)
			}
			return
		}

		m.removeAutostart(id)
		logger.Infof("uninstall %q success", id)
		err := m.service.Emit(m, "UninstallSuccess", id)
		if err != nil {
			logger.Warning("emit UninstallSuccess Failed:", err)
		}
	}()
	return nil
}

func (m *Manager) isItemsChanged() bool {
	old := atomic.SwapUint32(&m.itemsChangedHit, 0)
	return old > 0
}

func (m *Manager) Search(key string) *dbus.Error {
	key = strings.ToLower(key)
	logger.Debug("Search key:", key)

	keyRunes := []rune(key)

	m.searchMu.Lock()

	if m.isItemsChanged() {
		// clear search cache
		m.popPushOpChan <- &popPushOp{popCount: len(m.currentRunes)}
		m.currentRunes = nil
	}

	popCount, runesPush := runeSliceDiff(keyRunes, m.currentRunes)

	logger.Debugf("runeSliceDiff key %v, current %v", keyRunes, m.currentRunes)
	logger.Debugf("runeSliceDiff popCount %v, runesPush %v", popCount, runesPush)

	m.popPushOpChan <- &popPushOp{popCount, runesPush}
	m.currentRunes = keyRunes

	m.searchMu.Unlock()
	return nil
}

func (m *Manager) GetUseProxy(id string) (value bool, busErr *dbus.Error) {
	return m.getUseFeature(gsKeyAppsUseProxy, id)
}

func (m *Manager) SetUseProxy(id string, value bool) *dbus.Error {
	return m.setUseFeature(gsKeyAppsUseProxy, id, value)
}

func (m *Manager) GetDisableScaling(id string) (value bool, busErr *dbus.Error) {
	return m.getUseFeature(gsKeyAppsDisableScaling, id)
}

func (m *Manager) SetDisableScaling(id string, value bool) *dbus.Error {
	return m.setUseFeature(gsKeyAppsDisableScaling, id, value)
}

func (m *Manager) getExecutablePath(sender dbus.Sender) (string, error) {
	pid, err := m.service.GetConnPID(string(sender))
	if err != nil {
		return "", err
	}

	execPath, err := procfs.Process(pid).Exe()
	if err != nil {
		return "", err
	}

	return execPath, nil
}
