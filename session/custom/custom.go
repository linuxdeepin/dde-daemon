// 无锡税务局定制
package custom

import (
	"os"
	"os/user"

	"github.com/godbus/dbus"
	accounts "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.accounts"
	"pkg.deepin.io/gir/gio-2.0"
)

const (
	dockSchema                = "com.deepin.dde.dock"
	dockMainWindowSchema      = "com.deepin.dde.dock.mainwindow"
	sessionShellSchema        = "com.deepin.dde.session-shell"
	settingKeyOnlyShowByWin   = "only-show-by-win"
	settingKeyHideMode        = "hide-mode"
	settingKeySystemHibernate = "system-hibernate"
	settingKeySystemShutdown  = "system-shutdown"
	settingKeySystemSuspend   = "system-suspend"
	settingKeySystemLock      = "system-lock"
	settingKeySystemReboot    = "system-reboot"
	settingKeySystemLogout    = "system-logout"
)

func IsStandardUser() (bool, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return false, err
	}
	acct := accounts.NewAccounts(conn)
	currentUser, err := user.Current()
	if err != nil {
		return false, err
	}

	objPath, err := acct.FindUserById(0, currentUser.Uid)
	if err != nil {
		return false, err
	}

	userObj, err := accounts.NewUser(conn, dbus.ObjectPath(objPath))
	if err != nil {
		return false, err
	}

	accountType, err := userObj.AccountType().Get(0)
	if err != nil {
		return false, err
	}

	return accountType == 0, nil
}

func IsNormalUser() bool {
	return os.Geteuid() != 0
}

func initDock() {
	gs := gio.NewSettings(dockSchema)
	defer gs.Unref()

	if gs.GetSchema().HasKey(settingKeyHideMode) {
		gs.SetString(settingKeyHideMode, "keep-hidden")
	}
}

func initDockMainWindow() {
	gs := gio.NewSettings(dockMainWindowSchema)
	defer gs.Unref()

	if gs.GetSchema().HasKey(settingKeyOnlyShowByWin) {
		gs.SetBoolean(settingKeyOnlyShowByWin, true)
	}
}

func initSessionShell() {
	gs := gio.NewSettings(sessionShellSchema)
	defer gs.Unref()

	if gs.GetSchema().HasKey(settingKeySystemLock) {
		gs.SetString(settingKeySystemLock, "Hiden")
	}
	if gs.GetSchema().HasKey(settingKeySystemSuspend) {
		gs.SetString(settingKeySystemSuspend, "Hiden")
	}
	if gs.GetSchema().HasKey(settingKeySystemShutdown) {
		gs.SetString(settingKeySystemShutdown, "Hiden")
	}
	if gs.GetSchema().HasKey(settingKeySystemHibernate) {
		gs.SetString(settingKeySystemHibernate, "Hiden")
	}
	if gs.GetSchema().HasKey(settingKeySystemReboot) {
		gs.SetString(settingKeySystemReboot, "Hiden")
	}
	if gs.GetSchema().HasKey(settingKeySystemLogout) {
		gs.SetString(settingKeySystemLogout, "Hiden")
	}
}

func InitGSettings() {
	// 普通用户屏蔽任务栏
	if IsNormalUser() {
		initDock()
		initDockMainWindow()
		initSessionShell()
	}
}
