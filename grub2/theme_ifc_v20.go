package grub2

import (
	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	DbusServiceNameV20 = "com.deepin.daemon.Grub2"
	themeDbusPathV20        = "/com/deepin/daemon/Grub2/Theme"
	themeDbusInterfaceV20   = "com.deepin.daemon.Grub2.Theme"
)

// Theme is a dbus object which provide properties and methods to
// setup deepin grub2 theme.
type ThemeV20 struct {
	t *Theme
	service *dbusutil.Service
}

// NewThemeV20 create Theme object.
func NewThemeV20(g *Grub2) *ThemeV20 {
	themeV20 := &ThemeV20{}
	themeV20.service = g.service
	themeV20.t = g.theme
	return themeV20
}

func (*ThemeV20) GetInterfaceName() string {
	return themeDbusInterfaceV20
}

func (theme *ThemeV20) SetBackgroundSourceFile(sender dbus.Sender, filename string) *dbus.Error {
	theme.service.DelayAutoQuit()
	return theme.t.SetBackgroundSourceFile(sender, filename)
}