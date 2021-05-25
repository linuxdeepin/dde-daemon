package lang

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/godbus/dbus"
	accounts "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.accounts"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"pkg.deepin.io/dde/daemon/loader"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/log"
)

const (
	langService = "com.deepin.system.Lang"

	userLocaleConfigFile    = ".config/locale.conf"
	userLocaleConfigFileTmp = ".config/.locale.conf"
)

type Module struct {
	*loader.ModuleBase
	lang *Lang
}

func (m *Module) GetDependencies() []string {
	return nil
}

func (m *Module) Start() error {
	if m.lang != nil {
		return nil
	}
	logger.Debug("start language service")
	m.lang = newLang()

	service := loader.GetService()
	m.lang.service = service

	m.lang.init()

	return nil
}

func (m *Module) Stop() error {
	// TODO:
	return nil
}

var logger = log.NewLogger("daemon/system/lang")

func newModule(logger *log.Logger) *Module {
	m := new(Module)
	m.ModuleBase = loader.NewModuleBase("lang", m, logger)
	return m
}

func init() {
	loader.Register(newModule(logger))
}

//go:generate dbusutil-gen -type Lang lang.go
//go:generate dbusutil-gen em -type Lang

type Lang struct {
	sessionMap    map[string]login1.SessionDetail
	sessionMapMux sync.Mutex
	service       *dbusutil.Service
	sigLoop       *dbusutil.SignalLoop
}

func newLang() *Lang {
	return &Lang{sessionMap: make(map[string]login1.SessionDetail)}
}

func (l *Lang) init() {
	l.sigLoop = dbusutil.NewSignalLoop(l.service.Conn(), 10)
	l.sigLoop.Start()

	l.loadSessionList()
	l.listenSystemSignals()
	l.updateAllUserLocale(true)
}

func (l *Lang) updateLocaleBySessionPath(sessionId string, sessionPath dbus.ObjectPath) {
	var sessionDetail login1.SessionDetail
	var ok bool

	l.sessionMapMux.Lock()
	if sessionDetail, ok = l.sessionMap[string(sessionPath)]; !ok {
		logger.Debugf("can not find sessionPath %s, return", sessionPath)
		l.sessionMapMux.Unlock()
		return
	}
	l.sessionMapMux.Unlock()

	account := accounts.NewAccounts(l.service.Conn())

	userPath, err := account.FindUserByName(0, sessionDetail.UserName)
	if err != nil {
		logger.Warning(err)
		return
	}

	l.updateLocaleByUserPath(dbus.ObjectPath(userPath))
	l.deleteSessionMap(sessionPath)
}

func (l *Lang) updateLocaleByUserPath(userPath dbus.ObjectPath) {
	user, err := accounts.NewUser(l.service.Conn(), userPath)
	if err != nil {
		logger.Warning(err)
		return
	}

	homeDir, err := user.HomeDir().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	localeConfigFile := filepath.Join(homeDir, userLocaleConfigFile)
	localeConfigFileTmp := filepath.Join(homeDir, userLocaleConfigFileTmp)

	_, err = os.Stat(localeConfigFileTmp)
	if err != nil {
		return
	}

	err = os.Rename(localeConfigFileTmp, localeConfigFile)
	if err != nil {
		logger.Warning(err)
	}
}

func (l *Lang) updateAllUserLocale(start bool) {
	if !start {
		return
	}

	manager := login1.NewManager(l.service.Conn())

	account := accounts.NewAccounts(l.service.Conn())
	userList, err := account.UserList().Get(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	inhibit, err := manager.Inhibit(0, "shutdown", langService, "to write language config file", "delay")
	if err != nil {
		logger.Warning(err)
	}

	for _, user := range userList {
		l.updateLocaleByUserPath(dbus.ObjectPath(user))
	}

	err = syscall.Close(int(inhibit))
	if err != nil {
		logger.Warning(err)
	}
}

func (l *Lang) deleteSessionMap(sessionPath dbus.ObjectPath) {
	l.sessionMapMux.Lock()
	defer l.sessionMapMux.Unlock()

	delete(l.sessionMap, string(sessionPath))
}

func (l *Lang) addSessionMap(sessionId string, sessionPath dbus.ObjectPath) {
	l.loadSessionList()
}

func (l *Lang) listenSystemSignals() {
	manager := login1.NewManager(l.service.Conn())
	manager.InitSignalExt(l.sigLoop, true)

	// 登录时,监听是哪个 session 登录,获取这个 session 的用户信息保存到 map 中
	_, err := manager.ConnectSessionNew(l.addSessionMap)
	if err != nil {
		logger.Warning(err)
	}

	// 注销时,监听是哪个 session 注销,从 map 中获取这个 session 的用户信息, 对这个用户的家目录下语言配置进行更新
	_, err = manager.ConnectSessionRemoved(l.updateLocaleBySessionPath)
	if err != nil {
		logger.Warning(err)
	}

	// 关机时,更新所有用户的家目录语言
	_, err = manager.ConnectPrepareForShutdown(l.updateAllUserLocale)
	if err != nil {
		logger.Warning(err)
	}
}

func (l *Lang) loadSessionList() {
	manager := login1.NewManager(l.service.Conn())
	sessions, err := manager.ListSessions(0)
	if err != nil {
		logger.Warning(err)
		return
	}

	l.sessionMapMux.Lock()
	defer l.sessionMapMux.Unlock()

	for _, sessionDetail := range sessions {
		l.sessionMap[string(sessionDetail.Path)] = sessionDetail
	}
}
