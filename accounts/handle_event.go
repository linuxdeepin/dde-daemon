/*
 * Copyright (C) 2013 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package accounts

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"pkg.deepin.io/dde/daemon/accounts/users"
	"pkg.deepin.io/lib/strv"
	dutils "pkg.deepin.io/lib/utils"
)

const (
	userFilePasswd  = "/etc/passwd"
	userFileGroup   = "/etc/group"
	userFileShadow  = "/etc/shadow"
	userFileSudoers = "/etc/sudoers"

	lightdmConfig = "/etc/lightdm/lightdm.conf"
	kdmConfig     = "/usr/share/config/kdm/kdmrc"
	gdmConfig     = "/etc/gdm/custom.conf"
)

const (
	fileEventDelay = time.Millisecond * 500

	taskNamePasswd = "passwd"
	taskNameGroup  = "group"
	taskNameShadow = "shadow"
	taskNameDM     = "dm"
	taskNameSSSD   = "sssd"
)

func (m *Manager) getSSSDUserFile() string {
	fileSSSD := ""

	filepath := "/etc/sssd/sssd.conf"
	val, exist := dutils.ReadKeyFromKeyFile(filepath,
		"sssd", "domains", "")
	if !exist {
		return ""
	}

	domains, ok := val.(string)
	if !ok {
		return ""
	}
	fileSSSD = domains
	if val == "" || strings.Contains(domains, "DOMAIN_NAME") {
		fileSSSD = os.Getenv("DOMAIN_NAME")
	}
	if fileSSSD != "" {
		fileSSSD = "/var/lib/sss/db/" + "cache_" + fileSSSD + ".ldb"
	}

	return fileSSSD
}

func (m *Manager) getWatchFiles() []string {
	list := []string{"/etc"}
	dmConfig, err := users.GetDMConfig()
	if err == nil {
		list = append(list, filepath.Dir(dmConfig))
	}

	fileSSSD := m.getSSSDUserFile()
	logger.Debug("sssd file:", fileSSSD)
	if fileSSSD != "" {
		list = append(list, fileSSSD)
	}

	return list
}

func (m *Manager) handleFileChanged(ev fsnotify.Event) {
	var err error
	switch ev.Name {
	case userFilePasswd:
		logger.Debug("File changed:", ev)
		if task, _ := m.delayTaskManager.GetTask(taskNamePasswd); task != nil {
			err = task.Start()
		}
	case userFileGroup, userFileSudoers:
		logger.Debug("File changed:", ev)
		if task, _ := m.delayTaskManager.GetTask(taskNameGroup); task != nil {
			err = task.Start()
		}
	case userFileShadow:
		logger.Debug("File changed:", ev)
		if task, _ := m.delayTaskManager.GetTask(taskNameShadow); task != nil {
			err = task.Start()
		}
	case lightdmConfig, kdmConfig, gdmConfig:
		logger.Debug("File changed:", ev)
		if task, _ := m.delayTaskManager.GetTask(taskNameDM); task != nil {
			err = task.Start()
		}
	case m.fileSSSD:
		logger.Debug("sssd cache file changed:", ev)
		if m.fileSSSD != "" {
			if task, _ := m.delayTaskManager.GetTask(taskNameSSSD); task != nil {
				err = task.Start()
			}
		}
	default:
		return
	}
	if err != nil {
		logger.Warning("Failed to start task:", err, ev)
	}
}

func (m *Manager) handleFilePasswdChanged() {
	infos, err := users.GetHumanUserInfos()
	if err != nil {
		logger.Warning(err)
		return
	}

	infosMap := make(map[string]*users.UserInfo)
	for idx := range infos {
		info := &infos[idx]
		infosMap[info.Uid] = info
	}

	m.usersMapMu.Lock()

	// 之后需要删除的用户的uid列表
	var uidsDelete []string

	for _, u := range m.usersMap {
		uInfo, ok := infosMap[u.Uid]
		if ok {
			u.updatePropsPasswd(uInfo)
		} else {
			// 域账户没有保存在本地，无需删除
			if !IsDomainUserID(u.Uid) {
				uidsDelete = append(uidsDelete, u.Uid)
			}
		}
		delete(infosMap, u.Uid)
	}
	m.usersMapMu.Unlock()

	for _, uid := range uidsDelete {
		m.deleteUser(uid)
	}

	// infosMap 中还存留的用户，就是新增加的用户。
	for _, uInfo := range infosMap {
		m.addUser(uInfo)
	}

	m.updatePropUserList()
}

func (m *Manager) updatePropUserList() {
	logger.Debug("updatePropUserList")
	var userPaths []string

	m.usersMapMu.Lock()

	for _, u := range m.usersMap {
		userPath := userDBusPathPrefix + u.Uid
		userPaths = append(userPaths, userPath)
	}

	m.usersMapMu.Unlock()
	sort.Strings(userPaths)

	m.UserListMu.Lock()
	if !strv.Strv(userPaths).Equal(m.UserList) {
		m.UserList = userPaths
		err := m.service.EmitPropertyChanged(m, "UserList", userPaths)
		if err != nil {
			logger.Warning(err)
		}
	}
	m.UserListMu.Unlock()
}

func (m *Manager) handleFileGroupChanged() {
	m.usersMapMu.Lock()
	defer m.usersMapMu.Unlock()
	for _, u := range m.usersMap {
		u.updatePropAccountType()
		u.updatePropCanNoPasswdLogin()
		u.updatePropGroups()
	}
}

func (m *Manager) handleFileShadowChanged() {
	m.usersMapMu.Lock()
	defer m.usersMapMu.Unlock()

	for _, u := range m.usersMap {
		shadowInfo, err := users.GetShadowInfo(u.UserName)
		if err == nil {
			u.updatePropsShadow(shadowInfo)
		}
	}
}

func (m *Manager) handleDMConfigChanged() {
	for _, u := range m.usersMap {
		u.updatePropAutomaticLogin()
	}
}

func (m *Manager) addUser(uInfo *users.UserInfo) {
	logger.Debug("addUser", uInfo.Uid)
	err := m.exportUserByUid(uInfo.Uid)
	if err != nil {
		logger.Warningf("failed to export user %s: %v", uInfo.Uid, err)
		return
	}
	err = m.service.Emit(m, "UserAdded", userDBusPathPrefix+uInfo.Uid)
	if err != nil {
		logger.Warning(err)
	}
}

func (m *Manager) addDomainUser(uId uint32) error {
	logger.Debug("addDomainUser", uId)
	domainUserUId := strconv.FormatUint(uint64(uId), 10)
	err := m.exportUserByUid(domainUserUId)

	if err != nil {
		logger.Warningf("failed to export user %d: %v", uId, err)
		return err
	}

	m.updatePropUserList()
	err = m.service.Emit(m, "UserAdded", userDBusPathPrefix+domainUserUId)
	if err != nil {
		logger.Warning(err)
	}

	return err
}

func (m *Manager) handleSSSDChanged() {
	m.UpdateADDomainUserList()
}
