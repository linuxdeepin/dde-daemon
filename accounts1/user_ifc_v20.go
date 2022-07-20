package accounts

import (
	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	userDBusPathPrefixV20 = "/com/deepin/daemon/Accounts/User"
	userDBusInterfaceV20  = "com.deepin.daemon.Accounts.User"
)

type UserV20 struct {
	service         *dbusutil.Service
	// 用户是否被禁用
	Locked bool
	// 是否允许此用户自动登录
	AutomaticLogin bool
	// 是否自动登录
	NoPasswdLogin bool

	// User
	u *User
}

func (user *UserV20) GetInterfaceName() string {
	return userDBusInterfaceV20
}

func NewUserV20(u *User) *UserV20 {
	userV20 := &UserV20{
		Locked: u.Locked,
		AutomaticLogin: u.AutomaticLogin,
		NoPasswdLogin: u.NoPasswdLogin,
		u: u,
	}
	return userV20
}

func (user *UserV20) EnableNoPasswdLogin(sender dbus.Sender, enabled bool) *dbus.Error {
	return user.u.EnableNoPasswdLogin(sender, enabled)
}

func (user *UserV20) SetAutomaticLogin(sender dbus.Sender, enabled bool) *dbus.Error {
	return user.u.SetAutomaticLogin(sender, enabled)
}

func (user *UserV20) SetLocked(sender dbus.Sender, locked bool) *dbus.Error {
	return user.u.SetLocked(sender, locked)
}

func (user *UserV20) syncLocked(value bool) {
	user.Locked = value
	user.emitPropChangedLocked(value)
}

func (user *UserV20) emitPropChangedLocked(value bool) error {
	return user.service.EmitPropertyChanged(user, "Locked", value)
}

	func (user *UserV20) syncAutomaticLogin(value bool) {
		user.Locked = value
	user.emitPropChangedAutomaticLogin(value)
}

func (user *UserV20) emitPropChangedAutomaticLogin(value bool) error {
	return user.service.EmitPropertyChanged(user, "AutomaticLogin", value)
}

func (user *UserV20) syncNoPasswdLogin(value bool) {
	user.Locked = value
	user.emitPropChangedNoPasswdLogin(value)
}

func (user *UserV20) emitPropChangedNoPasswdLogin(value bool) error {
	return user.service.EmitPropertyChanged(user, "NoPasswdLogin", value)
}
