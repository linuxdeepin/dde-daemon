package accounts

import "github.com/godbus/dbus"

const (
	dbusServiceNameV20 = "com.deepin.daemon.Accounts"
	dbusPathV20        = "/com/deepin/daemon/Accounts"
	dbusInterfaceV20   = "com.deepin.daemon.Accounts"
)

type ManagerV20 struct {
	m *Manager
	UserList      []string
	usersMap   map[string]*UserV20
}

func (manager *ManagerV20) GetInterfaceName() string {
	return dbusInterfaceV20
}

func NewManagerV20(m *Manager) *ManagerV20 {
	managerV20 := &ManagerV20{m: m}

	managerV20.usersMap = make(map[string] *UserV20)
	// init V20 User info
	for _, user := range m.usersMap {
		userV20 := NewUserV20(user)
		userV20Path := userDBusPathPrefixV20 + user.Uid
		managerV20.UserList = append(managerV20.UserList, userV20Path)
		managerV20.usersMap[userV20Path] = userV20
	}
	return managerV20
}

func (manager *ManagerV20) syncUserList(value []string) {
	manager.UserList = value
	manager.emitPropChangedUserList(value)
}

func (manager *ManagerV20) emitPropChangedUserList(value []string) error {
	return manager.m.service.EmitPropertyChanged(manager, "UserList", value)
}

func (manager *ManagerV20) exportUsers() {
	for _, u := range manager.usersMap {
		userPath := userDBusPathPrefixV20 + u.u.Uid
		err := manager.m.service.Export(dbus.ObjectPath(userPath), u)
		if err != nil {
			logger.Errorf("failed to export V20 user %q: %v", u.u.Uid, err)
		}
	}
}

func (manager *ManagerV20) addUser(user *User) {
	userV20 := NewUserV20(user)
	userV20Path := userDBusPathPrefixV20 + user.Uid
	manager.UserList = append(manager.UserList, userV20Path)
	manager.usersMap[userV20Path] = userV20

	err := manager.m.service.Export(dbus.ObjectPath(userV20Path), user)
	if err != nil {
		logger.Errorf("failed to export V20 user %q: %v", user.Uid, err)
	}
}

func (manager *ManagerV20) stopExportUsers() {
	for _, userPath := range manager.UserList {
		u, ok := manager.usersMap[userPath]
		if !ok {
			logger.Debug("Invalid V20 user path:", userPath)
			return
		}
		manager.m.service.StopExport(u)
	}
	manager.usersMap = nil
	manager.UserList = nil
}

func (manager *ManagerV20) stopExportUser(uid string) {
	userPath := userDBusPathPrefixV20 + uid
	user, ok := manager.usersMap[userPath]
	if !ok {
		logger.Debug("Invalid user path:", userPath)
		return
	}

	err := manager.m.service.StopExport(user)
	if err != nil {
		logger.Errorf("failed to StopExport V20 user %q: %v", uid, err)
	}

	delete(manager.usersMap,  userPath)
	for index, path := range manager.UserList {
		if userPath == path {
			manager.UserList = append(manager.UserList[0:index], manager.UserList[index+1:]...)
		}
	}
}
