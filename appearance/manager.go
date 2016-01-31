package appearance

import (
	"encoding/json"
	"fmt"
	"path"

	"gir/gio-2.0"
	"gir/glib-2.0"
	"github.com/howeyc/fsnotify"
	"pkg.deepin.io/dde/daemon/appearance/background"
	"pkg.deepin.io/dde/daemon/appearance/dtheme"
	"pkg.deepin.io/dde/daemon/appearance/fonts"
	"pkg.deepin.io/dde/daemon/appearance/subthemes"
	dutils "pkg.deepin.io/lib/utils"
)

const (
	TypeDTheme        string = "dtheme"
	TypeGtkTheme             = "gtk"
	TypeIconTheme            = "icon"
	TypeCursorTheme          = "cursor"
	TypeBackground           = "background"
	TypeStandardFont         = "standardfont"
	TypeMonospaceFont        = "monospacefont"
	TypeFontSize             = "fontsize"
)

const (
	dthemeDefaultId = "deepin"
	dthemeCustomId  = "Custom"

	wrapBgSchema    = "com.deepin.wrap.gnome.desktop.background"
	gsKeyBackground = "picture-uri"

	appearanceSchema = "com.deepin.dde.appearance"
	gsKeyTheme       = "theme"
	gsKeyFontSize    = "font-size"
)

type Manager struct {
	// Current desktop theme
	Theme string
	// Current desktop font size
	FontSize int32

	// Theme changed signal
	// ty, name
	Changed func(string, string)

	setting *gio.Settings

	wrapBgSetting  *gio.Settings

	watcher    *fsnotify.Watcher
	endWatcher chan struct{}
}

func NewManager() *Manager {
	var m = new(Manager)
	m.setting = gio.NewSettings(appearanceSchema)
	m.setPropTheme(m.setting.GetString(gsKeyTheme))
	m.setPropFontSize(m.setting.GetInt(gsKeyFontSize))

	m.wrapBgSetting, _ = dutils.CheckAndNewGSettings(wrapBgSchema)

	var err error
	m.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		logger.Warning("New file watcher failed:", err)
	} else {
		m.endWatcher = make(chan struct{})
	}

	m.init()

	return m
}

func (m *Manager) destroy() {
	m.setting.Unref()

	if m.wrapBgSetting != nil {
		m.wrapBgSetting.Unref()
	}

	if m.watcher != nil {
		close(m.endWatcher)
		m.watcher.Close()
		m.watcher = nil
	}
}

func (m *Manager) init() {
	var file = path.Join(glib.GetUserConfigDir(), "fontconfig", "fonts.conf")
	if dutils.IsFileExist(file) {
		return
	}

	dt := m.getCurrentDTheme()
	if dt == nil {
		logger.Error("Not found valid dtheme")
		return
	}

	err := fonts.SetFamily(dt.StandardFont.Id, dt.MonospaceFont.Id, dt.FontSize)
	if err != nil {
		logger.Debug("[init]----------- font failed:", err)
		return
	}
}

func (m *Manager) doSetDTheme(id string) error {
	logger.Debug("[doSetDTheme] theme id:", id)
	err := dtheme.SetDTheme(id)
	if err != nil {
		logger.Debug("[doSetDTheme] set failed:", err)
		return err
	}

	if m.Theme == id {
		logger.Debug("[doSetDTheme] theme same:", id, m.Theme)
		return nil
	}

	m.setPropTheme(id)
	m.setting.SetString(gsKeyTheme, id)

	logger.Debug("[doSetDTheme] set font size")
	return m.doSetFontSize(dtheme.ListDTheme().Get(id).FontSize)
}

func (m *Manager) doSetGtkTheme(value string) error {
	dt := m.getCurrentDTheme()
	if dt == nil {
		return fmt.Errorf("Not found valid dtheme")
	}

	if dt.Gtk.Id == value {
		return nil
	}

	if !subthemes.IsGtkTheme(value) {
		return fmt.Errorf("Invalid gtk theme '%v'", value)
	}

	subthemes.SetGtkTheme(value)
	return m.setDThemeByComponent(&dtheme.ThemeComponent{
		Gtk:           value,
		Icon:          dt.Icon.Id,
		Cursor:        dt.Cursor.Id,
		Background:    dt.Background.Id,
		StandardFont:  dt.StandardFont.Id,
		MonospaceFont: dt.MonospaceFont.Id,
	})
}

func (m *Manager) doSetIconTheme(value string) error {
	dt := m.getCurrentDTheme()
	if dt == nil {
		return fmt.Errorf("Not found valid dtheme")
	}

	if dt.Icon.Id == value {
		return nil
	}

	if !subthemes.IsIconTheme(value) {
		return fmt.Errorf("Invalid icon theme '%v'", value)
	}

	subthemes.SetIconTheme(value)
	return m.setDThemeByComponent(&dtheme.ThemeComponent{
		Gtk:           dt.Gtk.Id,
		Icon:          value,
		Cursor:        dt.Cursor.Id,
		Background:    dt.Background.Id,
		StandardFont:  dt.StandardFont.Id,
		MonospaceFont: dt.MonospaceFont.Id,
	})
}

func (m *Manager) doSetCursorTheme(value string) error {
	dt := m.getCurrentDTheme()
	if dt == nil {
		return fmt.Errorf("Not found valid dtheme")
	}

	if dt.Cursor.Id == value {
		return nil
	}

	if !subthemes.IsCursorTheme(value) {
		return fmt.Errorf("Invalid cursor theme '%v'", value)
	}

	subthemes.SetCursorTheme(value)
	return m.setDThemeByComponent(&dtheme.ThemeComponent{
		Gtk:           dt.Gtk.Id,
		Icon:          dt.Icon.Id,
		Cursor:        value,
		Background:    dt.Background.Id,
		StandardFont:  dt.StandardFont.Id,
		MonospaceFont: dt.MonospaceFont.Id,
	})
}

func (m *Manager) doSetBackground(value string) error {
	logger.Debug("[doSetBackground] start set:", value)
	dt := m.getCurrentDTheme()
	if dt == nil {
		logger.Debug("[doSetBackground] not found validity dtheme")
		return fmt.Errorf("Not found valid dtheme")
	}

	if dt.Background.Id == value {
		return nil
	}

	if !background.IsBackgroundFile(value) {
		return fmt.Errorf("Invalid background file '%v'", value)
	}

	uri, err := background.ListBackground().Set(value)
	if err != nil {
		logger.Debugf("[doSetBackground] set '%s' failed: %v", value, uri, err)
		return err
	}

	logger.Debug("[doSetBackground] set over...")
	return m.setDThemeByComponent(&dtheme.ThemeComponent{
		Gtk:           dt.Gtk.Id,
		Icon:          dt.Icon.Id,
		Cursor:        dt.Cursor.Id,
		Background:    uri,
		StandardFont:  dt.StandardFont.Id,
		MonospaceFont: dt.MonospaceFont.Id,
	})
}

func (m *Manager) doSetStandardFont(value string) error {
	dt := m.getCurrentDTheme()
	if dt == nil {
		return fmt.Errorf("Not found valid dtheme")
	}

	if dt.StandardFont.Id == value {
		return nil
	}

	if !fonts.IsFontFamily(value) {
		return fmt.Errorf("Invalid font family '%v'", value)
	}

	//fonts.SetFamily(value, dt.MonospaceFont.Id, m.FontSize)
	return m.setDThemeByComponent(&dtheme.ThemeComponent{
		Gtk:           dt.Gtk.Id,
		Icon:          dt.Icon.Id,
		Cursor:        dt.Cursor.Id,
		Background:    dt.Background.Id,
		StandardFont:  value,
		MonospaceFont: dt.MonospaceFont.Id,
	})
}

func (m *Manager) doSetMonnospaceFont(value string) error {
	dt := m.getCurrentDTheme()
	if dt == nil {
		return fmt.Errorf("Not found valid dtheme")
	}

	if dt.MonospaceFont.Id == value {
		return nil
	}

	if !fonts.IsFontFamily(value) {
		return fmt.Errorf("Invalid font family '%v'", value)
	}

	//fonts.SetFamily(dt.StandardFont.Id, value, m.FontSize)
	return m.setDThemeByComponent(&dtheme.ThemeComponent{
		Gtk:           dt.Gtk.Id,
		Icon:          dt.Icon.Id,
		Cursor:        dt.Cursor.Id,
		Background:    dt.Background.Id,
		StandardFont:  dt.StandardFont.Id,
		MonospaceFont: value,
	})
}

func (m *Manager) doSetFontSize(size int32) error {
	if m.FontSize == size {
		logger.Debug("[doSetFontSize] equal:", m.FontSize, size)
		return nil
	}

	if !fonts.IsFontSizeValid(size) {
		logger.Debug("[doSetFontSize] invalid size:", size)
		return fmt.Errorf("Invalid font size '%v'", size)
	}

	m.setPropFontSize(size)
	m.setting.SetInt(gsKeyFontSize, size)
	logger.Debug("[doSetFontSize] gsetting changed over:", m.setting.GetInt(gsKeyFontSize))

	if size == fonts.GetFontSize() {
		logger.Debug("[doSetFontSize] equal with xsetting:", fonts.GetFontSize(), size)
		return nil
	}
	dt := m.getCurrentDTheme()
	if dt == nil {
		logger.Debug("[doSetFontSize] not found valid dtheme")
		return fmt.Errorf("Not found valid dtheme")
	}

	return fonts.SetFamily(dt.StandardFont.Id, dt.MonospaceFont.Id, size)
}

func (m *Manager) getCurrentDTheme() *dtheme.DTheme {
	id := m.setting.GetString(gsKeyTheme)
	dt := dtheme.ListDTheme().Get(id)
	if dt != nil {
		return dt
	}

	m.doSetDTheme(dthemeDefaultId)
	m.setPropTheme(dthemeDefaultId)
	return dtheme.ListDTheme().Get(dthemeDefaultId)
}

func (m *Manager) setDThemeByComponent(component *dtheme.ThemeComponent) error {
	id := dtheme.ListDTheme().FindDThemeId(component)
	if len(id) != 0 {
		logger.Debug("[setDThemeByComponent] found match theme:", id)
		return m.doSetDTheme(id)
	}

	err := dtheme.WriteCustomTheme(component)
	if err != nil {
		logger.Debug("[setDThemeByComponent] write custom theme failed:", err)
		return err
	}
	return m.doSetDTheme(dthemeCustomId)
}

func (*Manager) doShow(ifc interface{}) (string, error) {
	if ifc == nil {
		return "", fmt.Errorf("Not found target")
	}
	content, err := json.Marshal(ifc)
	return string(content), err
}
