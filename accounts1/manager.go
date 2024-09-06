// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"sync"
	"syscall"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/accounts1/users"
	"github.com/linuxdeepin/dde-daemon/common/sessionmsg"
	udcp "github.com/linuxdeepin/go-dbus-factory/system/com.deepin.udcp.iam"
	login1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/tasker"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

const (
	actConfigDir       = "/var/lib/AccountsService"
	userConfigDir      = actConfigDir + "/deepin/users"
	userIconsDir       = actConfigDir + "/icons"
	userCustomIconsDir = actConfigDir + "/icons/local"

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

//go:generate dbusutil-gen -type Manager,User manager.go user.go
//go:generate dbusutil-gen em -type Manager,User,ImageBlur

type Manager struct {
	service       *dbusutil.Service
	sysSigLoop    *dbusutil.SignalLoop
	login1Manager login1.Manager
	PropsMu       sync.RWMutex

	UserList   []string
	UserListMu sync.RWMutex

	// dbusutil-gen: ignore
	GuestIcon  string
	AllowGuest bool

	watcher    *dutils.WatchProxy
	usersMap   map[string]*User
	usersMapMu sync.Mutex

	enablePasswdChangedHandler   bool
	enablePasswdChangedHandlerMu sync.Mutex

	delayTaskManager *tasker.DelayTaskManager
	userAddedChanMap map[string]chan string
	udcpCache        udcp.UdcpCache

	//nolint
	signals *struct {
		UserAdded struct {
			objPath string
		}

		UserDeleted struct {
			objPath string
		}
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

	m.GuestIcon = getRandomIcon()
	m.AllowGuest = isGuestUserEnabled()
	m.initUsers(getUserPaths())
	m.initUdcpUsers()

	m.watcher = dutils.NewWatchProxy()
	if m.watcher != nil {
		m.delayTaskManager = tasker.NewDelayTaskManager()
		_ = m.delayTaskManager.AddTask(taskNamePasswd, fileEventDelay, m.handleFilePasswdChanged)
		_ = m.delayTaskManager.AddTask(taskNameGroup, fileEventDelay, m.handleFileGroupChanged)
		_ = m.delayTaskManager.AddTask(taskNameShadow, fileEventDelay, m.handleFileShadowChanged)
		_ = m.delayTaskManager.AddTask(taskNameDM, fileEventDelay, m.handleDMConfigChanged)

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

		if userInfo.UID < 10000 {
			return
		}

		err = m.addUdcpUser(userInfo.UID)
		if err != nil {
			logger.Warningf("add login session failed:%v", err)
		}
	})

	_, _ = m.login1Manager.ConnectPrepareForSleep(func(before bool) {
		if before {
			return
		}

		pwdChangerLock.Lock()
		defer pwdChangerLock.Unlock()

		if pwdChangerProcess != nil {
			err := syscall.Kill(-pwdChangerProcess.Pid, syscall.SIGTERM)
			if err != nil {
				logger.Warning(err)
			}
		}
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
	_ = m.service.StopExport(m)
}

func (m *Manager) initUsers(list []string) {
	var userList []string
	for _, p := range list {
		u, err := NewUser(p, m.service, false)
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
	content, err := os.ReadFile(interfacesFile)
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

	udcpCache.SetInterfaceName_(ifcCfg.Interface)

	m.udcpCache = udcpCache
	return nil

}

func (m *Manager) initUdcpUsers() {
	// 解析json文件 新建udcp-cache对象,获取所有加域账户ID
	err := m.initUdcpCache()
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
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

		u, err := NewUdcpUser(uId, m.service, userGroups)
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

func (m *Manager) stopExportUsers(list []string) {
	for _, p := range list {
		m.stopExportUser(p)
	}
}

func (m *Manager) exportUserByUid(uId string) error {
	var err error
	var u *User
	var userGroups []string
	userPath := userDBusPathPrefix + uId
	id, _ := strconv.Atoi(uId)

	if /*m.isUserJoinUdcp()*/ id > 10000 {
		if users.ExistPwUid(uint32(id)) != 0 {
			return errors.New("no such user id")
		}
		userGroups, err = m.udcpCache.GetUserGroups(0, users.GetPwName(uint32(id)))
		if err != nil {
			logger.Errorf("Udcp cache getUserGroups failed: %v", err)
			return err
		}

		u, err = NewUdcpUser(uint32(id), m.service, userGroups)
	} else {
		u, err = NewUser(userPath, m.service, true)
	}
	if err != nil {
		return err
	}

	var stat = &syscall.Stat_t{}
	if err := syscall.Stat(u.HomeDir, stat); err != nil {
		logger.Warning(err)
	} else if strconv.Itoa(int(stat.Uid)) != u.Uid {
		logger.Debug("incorrect ownership")
		err = recoverOwnership(u)
		if err != nil {
			logger.Warning(err)
		}
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

func (m *Manager) stopExportUser(userPath string) {
	m.usersMapMu.Lock()
	defer m.usersMapMu.Unlock()
	u, ok := m.usersMap[userPath]
	if !ok {
		logger.Debug("Invalid user path:", userPath)
		return
	}

	delete(m.usersMap, userPath)
	_ = m.service.StopExport(u)
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

func chownHomeDir(homeDir string, username string) {
	logger.Debug("change owner for dir:", homeDir)
	err := exec.Command("chown", "-hR", username+":"+username, homeDir).Run()
	if err != nil {
		logger.Warningf("change owner for dir %v failed: %v", homeDir, err)
		return
	}
	err = sessionmsg.SendMessage(sessionmsg.NewMessage(true, &sessionmsg.BodyNotify{
		Icon: "preferences-system",
		Body: &sessionmsg.LocalizeStr{
			Format: Tr("User \"%s\" existed before and its data is synced"),
			Args:   []string{username},
		},
		ExpireTimeout: -1,
	}))
	if err != nil {
		logger.Warning("send session msg failed:", err)
	}
}

func Tr(text string) string {
	return text
}
