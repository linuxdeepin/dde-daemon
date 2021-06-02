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
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	//dbus "github.com/godbus/dbus"
	udcp "github.com/linuxdeepin/go-dbus-factory/com.deepin.udcp.iam"

	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"pkg.deepin.io/dde/daemon/accounts/users"
	dbus "pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/tasker"
	dutils "pkg.deepin.io/lib/utils"
)

const (
	actConfigDir       = "/var/lib/AccountsService"
	userConfigDir      = actConfigDir + "/deepin/users"
	userIconsDir       = actConfigDir + "/icons"
	userCustomIconsDir = actConfigDir + "/icons/local"

	userIconGuest       = actConfigDir + "/icons/guest.png"
	actConfigFile       = actConfigDir + "/accounts.ini"
	actConfigGroupGroup = "Accounts"
	actConfigKeyGuest   = "AllowGuest"

	interfacesFile = "/usr/share/dde-daemon/accounts/dbus-udcp.json"
)

type InterfaceConfig struct {
	Service   string `json:"service"`
	Path      string `json:"path"`
	Interface string `json:"interface"`
}

type domainUserConfig struct {
	Name      string
	Uid       string
	IsLogined bool
}

type DefaultDomainUserConfig map[string]*domainUserConfig

var configFile = filepath.Join(actConfigDir, "domainUser.json")

//go:generate dbusutil-gen -type Manager,User manager.go user.go

type Manager struct {
	service       *dbusutil.Service
	sysSigLoop    *dbusutil.SignalLoop
	login1Manager *login1.Manager
	PropsMu       sync.RWMutex

	UserList   []string
	UserListMu sync.RWMutex

	// dbusutil-gen: ignore
	GuestIcon  string
	AllowGuest bool

	watcher    *dutils.WatchProxy
	usersMap   map[string]*User
	usersMapMu sync.Mutex

	delayTaskManager *tasker.DelayTaskManager

	userAddedChanMap map[string]chan string

	userConfig      map[string]*domainUserConfig
	domainUserMapMu sync.Mutex
	//                    ^ username

	udcpCache                  *udcp.UdcpCache
	enablePasswdChangedHandler bool //add by glm

	//nolint
	signals *struct {
		UserAdded struct {
			objPath string
		}

		UserDeleted struct {
			objPath string
		}
	}

	methods *struct {
		CreateUser             func() `in:"name,fullName,accountType" out:"user"`
		DeleteUser             func() `in:"name,rmFiles"`
		FindUserById           func() `in:"uid" out:"user"`
		FindUserByName         func() `in:"name" out:"user"`
		RandUserIcon           func() `out:"iconFile"`
		IsUsernameValid        func() `in:"name" out:"ok,errReason,errCode"`
		IsPasswordValid        func() `in:"password" out:"ok,errReason,errCode"`
		AllowGuestAccount      func() `in:"allow"`
		CreateGuestAccount     func() `out:"user"`
		GetGroups              func() `out:"groups"`
		GetPresetGroups        func() `in:"accountType" out:"groups"`
		UpdateADDomainUserList func()
	}
}

func NewManager(service *dbusutil.Service) *Manager {
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return nil
	}
	login1Manager := login1.NewManager(systemBus)
	sysSigLoop := dbusutil.NewSignalLoop(systemBus, 10)
	sysSigLoop.Start()

	var m = &Manager{
		service:                    service,
		login1Manager:              login1Manager,
		sysSigLoop:                 sysSigLoop,
		enablePasswdChangedHandler: true,
	}

	m.usersMap = make(map[string]*User)
	m.userAddedChanMap = make(map[string]chan string)
	m.userConfig = make(map[string]*domainUserConfig)

	m.GuestIcon = userIconGuest
	m.AllowGuest = isGuestUserEnabled()
	m.initUsers(getUserPaths())

	m.initUdcpUsers()

	m.initDomainUsers()

	m.watcher = dutils.NewWatchProxy()
	if m.watcher != nil {
		m.delayTaskManager = tasker.NewDelayTaskManager()
		m.delayTaskManager.AddTask(taskNamePasswd, fileEventDelay, m.handleFilePasswdChanged)
		m.delayTaskManager.AddTask(taskNameGroup, fileEventDelay, m.handleFileGroupChanged)
		m.delayTaskManager.AddTask(taskNameShadow, fileEventDelay, m.handleFileShadowChanged)
		m.delayTaskManager.AddTask(taskNameDM, fileEventDelay, m.handleDMConfigChanged)

		m.watcher.SetFileList(m.getWatchFiles())
		m.watcher.SetEventHandler(m.handleFileChanged)
		go m.watcher.StartWatch()
	}

	m.login1Manager.InitSignalExt(m.sysSigLoop, true)
	_, _ = m.login1Manager.ConnectSessionNew(func(id string, sessionPath dbus.ObjectPath) {
		core, err := login1.NewSession(systemBus, sessionPath)
		if err != nil {
			logger.Warningf("new login1 session failed:%v", err)
			return
		}

		userInfo, err := core.User().Get(0)
		if err != nil {
			logger.Warningf("get user info failed:%v", err)
			return
		}

		if users.IsHumanUdcpUserUid(userInfo.UID) {
			if userInfo.UID > 10000 {
				logger.Warning("userInfo.UID < 10000", userInfo.UID)
				return
			}

			err = m.addUdcpUser(userInfo.UID)
		} else if IsDomainUserID(strconv.FormatUint(uint64(userInfo.UID), 10)) {
			err = m.addDomainUser(userInfo.UID)
		}
		if !users.IsHumanUdcpUserUid(userInfo.UID) {
			return
		}

		if err != nil {
			logger.Warningf("add login session failed:%v", err)
		}

		return

	})

	return m
}

func (m *Manager) destroy() {
	if m.watcher != nil {
		m.watcher.EndWatch()
		m.watcher = nil
	}

	m.sysSigLoop.Stop()
	m.stopExportUsers(m.UserList)
	m.service.StopExport(m)
}

func (m *Manager) initUsers(list []string) {
	var userList []string
	for _, p := range list {
		u, err := NewUser(p, m.service)
		if err != nil {
			logger.Errorf("New user '%s' failed: %v", p, err)
			continue
		}

		userList = append(userList, p)

		m.usersMapMu.Lock()
		m.usersMap[p] = u
		m.usersMapMu.Unlock()
	}
	sort.Strings(userList)
	m.UserList = userList
}

func (m *Manager) initUdcpCache() error {
	// 解析json文件 新建udcp-cache对象
	var ifcCfg InterfaceConfig
	content, err := ioutil.ReadFile(interfacesFile)
	if err != nil {
		return err
	}
	err = json.Unmarshal(content, &ifcCfg)
	if err != nil {
		logger.Warning(err)
		return err
	}
	if !dbus.ObjectPath(ifcCfg.Path).IsValid() {
		logger.Warningf("interface config file %q, path %q is invalid", interfacesFile, ifcCfg.Path)
		return err
	}

	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}

	udcpCache, err := udcp.NewUdcpCache(sysBus, ifcCfg.Service, dbus.ObjectPath(ifcCfg.Path))
	if err != nil {
		return err
	}

	m.udcpCache = udcpCache
	return nil

}

func (m *Manager) initUdcpUsers() {
	// 解析json文件 新建udcp-cache对象,获取所有加域账户ID
	err := m.initUdcpCache()
	if err != nil {
		logger.Errorf("New udcp cache object failed: %v", err)
		return
	}

	userIdList, err := m.udcpCache.GetUserIdList(0)
	if err != nil {
		logger.Errorf("Udcp cache getUserIdList failed: %v", err)
		return
	}

	isJoinUdcp, err := m.udcpCache.Enable().Get(0)
	if err != nil {
		logger.Errorf("Udcp cache get Enable failed: %v", err)
		return
	}

	if !isJoinUdcp {
		return
	}

	// 构造User服务对象
	var userList = m.UserList
	for _, uId := range userIdList {
		if users.ExistPwUid(uId) != 0 {
			continue
		}
		userGroups, err := m.udcpCache.GetUserGroups(0, users.GetPwName(uId))
		if err != nil {
			logger.Errorf("Udcp cache getUserGroups failed: %v", err)
			continue
		}

		u, err := NewUdcpUser(uId, m.service, userGroups, false)
		if err != nil {
			logger.Errorf("New udcp user '%d' failed: %v", uId, err)
			continue
		}
		userDBusPath := userDBusPathPrefix + strconv.FormatUint(uint64(uId), 10)
		userList = append(userList, userDBusPath)
		m.usersMapMu.Lock()
		m.usersMap[userDBusPath] = u
		m.usersMapMu.Unlock()
	}
	sort.Strings(userList)
	m.UserList = userList
}

func (m *Manager) exportUsers() {
	m.usersMapMu.Lock()

	for _, u := range m.usersMap {
		err := m.service.Export(dbus.ObjectPath(userDBusPathPrefix+u.Uid), u)
		if err != nil {
			logger.Errorf("failed to export user %q: %v",
				u.Uid, err)
			continue
		}

	}

	m.usersMapMu.Unlock()
}

func (m *Manager) exportUserByUid(uId string) error {
	var err error
	var u *User
	var userGroups []string
	var domainUserGroups []string
	userPath := userDBusPathPrefix + uId
	id, _ := strconv.Atoi(uId)

	domainUserGroups, err = GetUserGroupsByUID(uint32(id))
	if err != nil {
		logger.Warningf("failed to get domain user groups %s", uId)
	}

	if domainUserGroups != nil && IsDomainUserID(uId) {
		u, err = NewDomainUser(uint32(id), m.service)
		if err != nil {
			logger.Warningf("failed to new domain user: %s", u.UserName)
			return err
		}

		var config = &domainUserConfig{
			Name:      u.UserName,
			Uid:       u.Uid,
			IsLogined: true,
		}
		m.domainUserMapMu.Lock()
		m.userConfig[userPath] = config
		m.saveDomainUserConfig(m.userConfig)
		m.domainUserMapMu.Unlock()

	} else if id > 10000 && m.isUdcpUserID(uId) {
		if users.ExistPwUid(uint32(id)) != 0 {
			return errors.New("No such user id")
		}
		userGroups, err = m.udcpCache.GetUserGroups(0, users.GetPwName(uint32(id)))
		if err != nil {
			logger.Errorf("Udcp cache getUserGroups failed: %v", err)
			return err
		}

		u, err = NewUdcpUser(uint32(id), m.service, userGroups, true)
	} else {
		u, err = NewUser(userPath, m.service)
	}

	if err != nil {
		return err
	}

	m.usersMapMu.Lock()
	ch := m.userAddedChanMap[u.UserName]
	m.usersMapMu.Unlock()

	err = m.service.Export(dbus.ObjectPath(userPath), u)
	logger.Debugf("export user %q err: %v", userPath, err)
	if ch != nil {
		if err != nil {
			ch <- ""
		} else {
			ch <- userPath
		}
		logger.Debug("after ch <- userPath")
	}

	if err != nil {
		return err
	}

	m.usersMapMu.Lock()
	m.usersMap[userPath] = u
	m.usersMapMu.Unlock()

	return nil
}

func (m *Manager) initDomainUsers() {
	var domainUserList []string
	// 解析json文件,获取所有之前登录过的域账号
	config, err := m.loadDomainUserConfig()
	if config != nil {
		m.userConfig = config
	}

	if err != nil {
		logger.Errorf("init domain user config failed: %v", err)
		return
	}

	for _, v := range config {
		if IsDomainUserID(v.Uid) && (v.IsLogined == true) {
			domainUserList = append(domainUserList, v.Uid)
		}
	}

	// 构造User服务对象
	var userList = m.UserList
	for _, uId := range domainUserList {
		id, _ := strconv.Atoi(uId)
		u, err := NewDomainUser(uint32(id), m.service)
		if err != nil {
			logger.Errorf("New domain user '%s' failed: %v", uId, err)
			continue
		}

		userDBusPath := userDBusPathPrefix + uId
		userList = append(userList, userDBusPath)
		m.usersMapMu.Lock()
		m.usersMap[userDBusPath] = u
		m.usersMapMu.Unlock()
	}
	sort.Strings(userList)
	m.UserList = userList
}

func (m *Manager) loadDomainUserConfig() (DefaultDomainUserConfig, error) {
	logger.Debug("loadDomainUserConfig")
	var config DefaultDomainUserConfig
	if !dutils.IsFileExist(configFile) {
		err := dutils.CreateFile(configFile)
		if err != nil {
			return config, err
		}
	} else {
		data, err := ioutil.ReadFile(configFile)
		if err != nil {
			return config, err
		}

		if len(data) == 0 {
			logger.Warningf("domain user config file is empty")
			return config, nil
		}

		err = json.Unmarshal(data, &config)
		if err != nil {
			return config, err
		}

	}

	return config, nil
}

func (m *Manager) saveDomainUserConfig(config DefaultDomainUserConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	dir := filepath.Dir(configFile)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(configFile, data, 0644)
	return err
}

func (m *Manager) stopExportUsers(list []string) {
	for _, p := range list {
		m.stopExportUser(p)
	}
}

func (m *Manager) stopExportUser(userPath string) {
	m.usersMapMu.Lock()
	defer m.usersMapMu.Unlock()
	u, ok := m.usersMap[userPath]
	if !ok {
		logger.Debug("Invalid user path:", userPath)
		return
	}

	delete(m.usersMap, userPath)
	m.service.StopExport(u)
}

func (m *Manager) getUserByName(name string) *User {
	m.usersMapMu.Lock()
	defer m.usersMapMu.Unlock()

	for _, user := range m.usersMap {
		if user.UserName == name {
			return user
		}
	}
	return nil
}

func (m *Manager) getUserByUid(uid string) *User {
	m.usersMapMu.Lock()
	defer m.usersMapMu.Unlock()

	for _, user := range m.usersMap {
		if user.Uid == uid {
			return user
		}
	}
	return nil
}

func getUserPaths() []string {
	infos, err := users.GetHumanUserInfos()
	if err != nil {
		return nil
	}

	var paths []string
	for _, info := range infos {
		paths = append(paths, userDBusPathPrefix+info.Uid)
	}

	return paths
}

func isGuestUserEnabled() bool {
	v, exist := dutils.ReadKeyFromKeyFile(actConfigFile,
		actConfigGroupGroup, actConfigKeyGuest, true)
	if !exist {
		return false
	}

	ret, ok := v.(bool)
	if !ok {
		return false
	}

	return ret
}

func (m *Manager) checkAuth(sender dbus.Sender) error {
	return checkAuth(polkitActionUserAdministration, string(sender))
}
