// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-daemon/accounts/users"
	authenticate "github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.authenticate"
	glib "github.com/linuxdeepin/go-gir/glib-2.0"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gdkpixbuf"
	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/go-lib/strv"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

const (
	defaultUserIcon          = "file:///var/lib/AccountsService/icons/default.png"
	defaultUserBackgroundDir = "/usr/share/wallpapers/deepin/"

	controlCenterPath = "/usr/bin/dde-control-center"
	deepinDaemonDir   = "/usr/lib/deepin-daemon/"

	maxWidth  = 200
	maxHeight = 200
)

const (
	confGroupUser             = "User"
	confKeyXSession           = "XSession"
	confKeySystemAccount      = "SystemAccount"
	confKeyIcon               = "Icon"
	confKeyCustomIcon         = "CustomIcon"
	confKeyLocale             = "Locale"
	confKeyLayout             = "Layout"
	confKeyDesktopBackgrounds = "DesktopBackgrounds"
	confKeyGreeterBackground  = "GreeterBackground"
	confKeyHistoryLayout      = "HistoryLayout"
	confKeyUse24HourFormat    = "Use24HourFormat"
	confKeyUUID               = "UUID"
	confKeyWorkspace          = "Workspace"
	confKeyWeekdayFormat      = "WeekdayFormat"
	confKeyShortDateFormat    = "ShortDateFormat"
	confKeyLongDateFormat     = "LongDateFormat"
	confKeyShortTimeFormat    = "ShortTimeFormat"
	confKeyLongTimeFormat     = "LongTimeFormat"
	confKeyWeekBegins         = "WeekBegins"
	confKeyPasswordHint       = "PasswordHint"

	defaultUse24HourFormat = true
	defaultWeekdayFormat   = 0
	defaultShortDateFormat = 3
	defaultLongDateFormat  = 1
	defaultShortTimeFormat = 0
	defaultLongTimeFormat  = 0
	defaultWeekBegins      = 0
	defaultWorkspace       = 1
)

func getDefaultUserBackground() string {
	filename := filepath.Join(defaultUserBackgroundDir, "desktop.bmp")
	_, err := os.Stat(filename)
	if err == nil {
		return "file://" + filename
	}

	return "file://" + filepath.Join(defaultUserBackgroundDir, "desktop.jpg")
}

type User struct {
	service         *dbusutil.Service
	PropsMu         sync.RWMutex
	UserName        string
	UUID            string
	FullName        string
	Uid             string
	Gid             string
	HomeDir         string
	Shell           string
	Locale          string
	Layout          string
	IconFile        string
	PasswordHint    string
	Use24HourFormat bool
	WeekdayFormat   int32
	ShortDateFormat int32
	LongDateFormat  int32
	ShortTimeFormat int32
	LongTimeFormat  int32
	WeekBegins      int32

	customIcon string
	// dbusutil-gen: equal=nil
	DesktopBackgrounds []string
	// dbusutil-gen: equal=isStrvEqual
	Groups            []string
	GreeterBackground string
	XSession          string

	PasswordStatus     string
	MaxPasswordAge     int32
	PasswordLastChange int32
	// 用户是否被禁用
	Locked bool
	// 是否允许此用户自动登录
	AutomaticLogin bool
	// 当前工作区
	Workspace int32

	// deprecated property
	SystemAccount bool

	NoPasswdLogin bool

	AccountType int32
	LoginTime   uint64
	CreatedTime uint64

	// dbusutil-gen: equal=nil
	IconList []string
	// dbusutil-gen: equal=nil
	HistoryLayout []string

	configLocker sync.Mutex
}

func NewUser(userPath string, service *dbusutil.Service, ignoreErr bool) (*User, error) {
	userInfo, err := users.GetUserInfoByUid(getUidFromUserPath(userPath))
	if err != nil {
		return nil, err
	}

	shadowInfo, err := users.GetShadowInfo(userInfo.Name)
	if err != nil {
		if !ignoreErr {
			return nil, err
		} else {
			shadowInfo = &users.ShadowInfo{Name: userInfo.Name, Status: users.PasswordStatusLocked}
		}
	}

	var u = &User{
		service:            service,
		UserName:           userInfo.Name,
		FullName:           userInfo.Comment().FullName(),
		Uid:                userInfo.Uid,
		Gid:                userInfo.Gid,
		HomeDir:            userInfo.Home,
		Shell:              userInfo.Shell,
		AutomaticLogin:     users.IsAutoLoginUser(userInfo.Name),
		NoPasswdLogin:      users.CanNoPasswdLogin(userInfo.Name),
		Locked:             shadowInfo.Status == users.PasswordStatusLocked,
		PasswordStatus:     shadowInfo.Status,
		MaxPasswordAge:     int32(shadowInfo.MaxDays),
		PasswordLastChange: int32(shadowInfo.LastChange),
	}

	updateConfigPath(userInfo.Name)
	u.AccountType = u.getAccountType()
	u.Groups = u.getGroups()
	loadUserConfigInfo(u)

	return u, nil
}

func NewUdcpUser(usrId uint32, service *dbusutil.Service, groups []string) (*User, error) {
	var err error
	if users.ExistPwUid(usrId) != 0 {
		return nil, errors.New("no such user id")
	}

	userName := users.GetPwName(usrId)

	var u = &User{
		service:            service,
		UserName:           users.GetPwName(usrId),
		FullName:           users.GetPwGecos(usrId),
		Uid:                users.GetPwUid(usrId),
		Gid:                users.GetPwGid(usrId),
		HomeDir:            users.GetPwDir(usrId),
		Shell:              users.GetPwShell(usrId),
		AutomaticLogin:     users.IsAutoLoginUser(userName),
		NoPasswdLogin:      users.CanNoPasswdLogin(userName),
		Locked:             false,
		PasswordStatus:     users.PasswordStatusUsable,
		MaxPasswordAge:     30,
		PasswordLastChange: 18737,
	}

	u.Groups = groups

	// 解析对应域管用户是否有添加到sudo组里面，有为管理员用户，否则为标准用户
	if strv.Strv(groups).Contains("sudo") {
		u.AccountType = users.UserTypeAdmin
	} else {
		u.AccountType = users.UserTypeStandard
	}

	loadUserConfigInfo(u)
	return u, err
}

func getUserGreeterBackground(kf *glib.KeyFile) (string, bool) {
	greeterBg, _ := kf.GetString(confGroupUser, confKeyGreeterBackground)
	if greeterBg == "" {
		return "", false
	}
	_, err := os.Stat(dutils.DecodeURI(greeterBg))
	if err != nil {
		logger.Warning(err)
		return "", false
	}
	return greeterBg, true
}

func (u *User) updateIconList() {
	u.IconList = u.getAllIcons()
	_ = u.emitPropChangedIconList(u.IconList)
}

func (u *User) getAllIcons() []string {
	icons := _userStandardIcons
	if u.customIcon != "" {
		icons = append(icons, u.customIcon)
	}
	return icons
}

func (u *User) getGroups() []string {
	groups, err := users.GetUserGroups(u.UserName, u.Gid)
	if err != nil {
		logger.Warning("failed to get user groups:", err)
		return nil
	}
	return groups
}

// ret0: new user icon uri
// ret1: added
// ret2: error
func (u *User) setIconFile(iconURI string) (string, bool, error) {
	if isStrInArray(iconURI, u.IconList) {
		return iconURI, false, nil
	}

	iconFile := dutils.DecodeURI(iconURI)
	tmp, scaled, err := scaleUserIcon(iconFile)
	if err != nil {
		return "", false, err
	}

	if scaled {
		logger.Debug("icon scaled", tmp)
		defer func() {
			_ = os.Remove(tmp)
		}()
	}

	dest := getNewUserCustomIconDest(u.UserName)
	err = os.MkdirAll(path.Dir(dest), 0755)
	if err != nil {
		return "", false, err
	}
	err = dutils.CopyFile(tmp, dest)
	if err != nil {
		return "", false, err
	}
	return dutils.EncodeURI(dest, dutils.SCHEME_FILE), true, nil
}

type configChange struct {
	key   string
	value interface{} // allowed type are bool, string, []string , int32
}

func (u *User) writeUserConfigWithChanges(changes []configChange) error {
	u.configLocker.Lock()
	defer u.configLocker.Unlock()

	err := os.MkdirAll(userConfigDir, 0755)
	if err != nil {
		return err
	}

	config := path.Join(userConfigDir, u.UserName)
	if !dutils.IsFileExist(config) {
		err := dutils.CreateFile(config)
		if err != nil {
			return err
		}
	}

	kf, err := dutils.NewKeyFileFromFile(config)
	if err != nil {
		logger.Warningf("Load %s config file failed: %v", u.UserName, err)
		return err
	}
	defer kf.Free()

	kf.SetString(confGroupUser, confKeyXSession, u.XSession)
	kf.SetBoolean(confGroupUser, confKeySystemAccount, u.SystemAccount)
	kf.SetString(confGroupUser, confKeyLayout, u.Layout)
	kf.SetString(confGroupUser, confKeyLocale, u.Locale)
	kf.SetString(confGroupUser, confKeyIcon, u.IconFile)
	kf.SetString(confGroupUser, confKeyCustomIcon, u.customIcon)
	kf.SetStringList(confGroupUser, confKeyDesktopBackgrounds, u.DesktopBackgrounds)
	kf.SetString(confGroupUser, confKeyGreeterBackground, u.GreeterBackground)
	kf.SetStringList(confGroupUser, confKeyHistoryLayout, u.HistoryLayout)
	kf.SetString(confGroupUser, confKeyUUID, u.UUID)
	kf.SetInteger(confGroupUser, confKeyWorkspace, u.Workspace)

	for _, change := range changes {
		switch val := change.value.(type) {
		case bool:
			kf.SetBoolean(confGroupUser, change.key, val)
		case string:
			kf.SetString(confGroupUser, change.key, val)
		case []string:
			kf.SetStringList(confGroupUser, change.key, val)
		case int32:
			kf.SetInteger(confGroupUser, change.key, val)
		default:
			return errors.New("unsupported value type")
		}
	}

	_, err = kf.SaveToFile(config)
	if err != nil {
		logger.Warningf("Save %s config file failed: %v", u.UserName, err)
	}
	return err
}

func (u *User) writeUserConfigWithChange(confKey string, value interface{}) error {
	return u.writeUserConfigWithChanges([]configChange{
		{confKey, value},
	})
}

func (u *User) writeUserConfig() error {
	return u.writeUserConfigWithChanges(nil)
}

func (u *User) updatePropAccountType(accountType int32) {
	u.PropsMu.Lock()
	u.setPropAccountType(accountType)
	u.PropsMu.Unlock()
}

func (u *User) updatePropCanNoPasswdLogin() {
	newVal := users.CanNoPasswdLogin(u.UserName)
	u.PropsMu.Lock()
	u.setPropNoPasswdLogin(newVal)
	u.PropsMu.Unlock()
}

func (u *User) updatePropGroups(groups []string) {
	u.PropsMu.Lock()
	u.setPropGroups(groups)
	u.PropsMu.Unlock()
}

func (u *User) updatePropGroupsNoLock() {
	newVal := u.getGroups()
	u.setPropGroups(newVal)
}

func (u *User) updatePropAutomaticLogin() {
	newVal := users.IsAutoLoginUser(u.UserName)
	u.PropsMu.Lock()
	u.setPropAutomaticLogin(newVal)
	u.PropsMu.Unlock()
}

func (u *User) updatePropsPasswd(uInfo *users.UserInfo) {
	var userNameChanged bool
	var oldUserName string

	u.PropsMu.Lock()
	if u.Gid != uInfo.Gid {
		// gid 被修改
		u.setPropGid(uInfo.Gid)
		u.updatePropGroupsNoLock()
	}

	if u.UserName != uInfo.Name {
		oldUserName = u.UserName
		userNameChanged = true
	}
	u.setPropUserName(uInfo.Name)

	u.setPropHomeDir(uInfo.Home)
	u.setPropShell(uInfo.Shell)
	fullName := uInfo.Comment().FullName()
	u.setPropFullName(fullName)
	u.PropsMu.Unlock()

	if userNameChanged {
		logger.Debugf("user name changed old: %q, new: %q", oldUserName, uInfo.Name)
		err := os.Rename(filepath.Join(userConfigDir, oldUserName),
			filepath.Join(userConfigDir, uInfo.Name))
		if err != nil {
			logger.Warning(err)
		}
	}
}

func (u *User) updatePropsShadow(shadowInfo *users.ShadowInfo) {
	u.PropsMu.Lock()

	u.setPropPasswordStatus(shadowInfo.Status)
	u.setPropLocked(shadowInfo.Status == users.PasswordStatusLocked)
	u.setPropMaxPasswordAge(int32(shadowInfo.MaxDays))
	u.setPropPasswordLastChange(int32(shadowInfo.LastChange))

	u.PropsMu.Unlock()
}

func (u *User) getAccountType() int32 {
	if users.IsAdminUser(u.UserName) {
		return users.UserTypeAdmin
	}
	return users.UserTypeStandard
}

func (u *User) checkIsControlCenter(sender dbus.Sender) bool {
	pid, err := u.service.GetConnPID(string(sender))
	if err != nil {
		logger.Warning(err)
		return false
	}

	p := procfs.Process(pid)
	exe, err := p.Exe()
	if err != nil {
		logger.Warning(err)
		return false
	}

	if exe == controlCenterPath {
		return true
	}

	return false
}

func (u *User) checkIsDeepinDaemon(sender dbus.Sender) bool {
	pid, err := u.service.GetConnPID(string(sender))
	if err != nil {
		logger.Warning(err)
		return false
	}

	p := procfs.Process(pid)
	exe, err := p.Exe()
	if err != nil {
		logger.Warning(err)
		return false
	}

	abs, err := filepath.Abs(exe)
	if err != nil {
		logger.Warning(err)
		return false
	}

	if strings.HasPrefix(abs, deepinDaemonDir) {
		return true
	}

	return false
}

func (u *User) checkAuth(sender dbus.Sender, selfPass bool, actionId string) error {
	uid, err := u.service.GetConnUID(string(sender))
	if err != nil {
		return err
	}

	isSelf := u.isSelf(uid)
	if selfPass && isSelf {
		return nil
	}

	if actionId == "" {
		if isSelf {
			actionId = polkitActionChangeOwnData
		} else {
			actionId = polkitActionUserAdministration
		}
	}

	return checkAuth(actionId, string(sender))
}

func (u *User) checkAuthAutoLogin(sender dbus.Sender, enabled bool) error {
	var actionId string
	if enabled {
		actionId = polkitActionEnableAutoLogin
	} else {
		actionId = polkitActionDisableAutoLogin
	}

	return u.checkAuth(sender, false, actionId)
}

func (u *User) checkAuthNoPasswdLogin(sender dbus.Sender, enabled bool) error {
	var actionId string
	if enabled {
		actionId = polkitActionEnableNoPasswordLogin
	} else {
		actionId = polkitActionDisableNoPasswordLogin
	}
	return u.checkAuth(sender, false, actionId)
}

func (u *User) isSelf(uid uint32) bool {
	return u.Uid == strconv.FormatInt(int64(uid), 10)
}

func (u *User) clearData() {
	// delete user config file
	configFile := path.Join(userConfigDir, u.UserName)
	err := os.Remove(configFile)
	if err != nil {
		logger.Warning("remove user config failed:", err)
	}

	// delete user custom icon
	if u.customIcon != "" {
		customIconFile := dutils.DecodeURI(u.customIcon)
		err := os.Remove(customIconFile)
		if err != nil {
			logger.Warning("remove user custom icon failed:", err)
		}
	}

}

// 删除生物认证信息,TODO (人脸，虹膜)
func (u *User) clearBiometricChara() {
	u.clearFingers()

}
func (u *User) clearFingers() {
	logger.Debug("clearFingers")

	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warning("connect to system bus failed:", err)
		return
	}

	fpObj := authenticate.NewFingerprint(sysBus)
	err = fpObj.DeleteAllFingers(0, u.UserName)
	if err != nil {
		logger.Warning("failed to delete enrolled fingers:", err)
		return
	}

	logger.Debug("clear fingers succesed")
}

func (u *User) clearSecretQuestions() {
	path := filepath.Join(secretQuestionDirectory, u.UserName)

	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		logger.Warning(err)
	}
}

// userPath must be composed with 'userDBusPath + uid'
func getUidFromUserPath(userPath string) string {
	items := strings.Split(userPath, userDBusPathPrefix)

	return items[1]
}

// ret0: output file
// ret1: scaled
// ret2: error
func scaleUserIcon(file string) (string, bool, error) {
	w, h, err := gdkpixbuf.GetImageSize(file)
	if err != nil {
		return "", false, err
	}

	if w <= maxWidth && h <= maxHeight {
		return file, false, nil
	}

	dest, err := getTempFile()
	if err != nil {
		return "", false, err
	}

	err = gdkpixbuf.ScaleImagePrefer(file, dest, maxWidth, maxHeight, gdkpixbuf.GDK_INTERP_BILINEAR, gdkpixbuf.FormatPng)
	if err != nil {
		return "", false, err
	}

	return dest, true, nil
}

// return temp file path and error
func getTempFile() (string, error) {
	tmpfile, err := ioutil.TempFile("", "dde-daemon-accounts")
	if err != nil {
		return "", err
	}
	name := tmpfile.Name()
	_ = tmpfile.Close()
	return name, nil
}

func getUserSession(homeDir string) string {
	session, ok := dutils.ReadKeyFromKeyFile(homeDir+"/.dmrc", "Desktop", "Session", "")
	if !ok {
		v := ""
		list := getSessionList()
		switch len(list) {
		case 0:
			v = ""
		case 1:
			v = list[0]
		default:
			if strv.Strv(list).Contains("deepin.desktop") {
				v = "deepin.desktop"
			} else {
				v = list[0]
			}
		}
		return v
	}
	return session.(string)
}

func getSessionList() []string {
	fileInfoList, err := ioutil.ReadDir("/usr/share/xsessions")
	if err != nil {
		return nil
	}

	var sessions []string
	for _, fileInfo := range fileInfoList {
		if fileInfo.IsDir() || !strings.Contains(fileInfo.Name(), ".desktop") {
			continue
		}
		sessions = append(sessions, fileInfo.Name())
	}
	return sessions
}

// 迁移配置文件，复制文件从 $actConfigDir/users/$username 到 $userConfigDir/$username
func updateConfigPath(username string) {
	config := path.Join(userConfigDir, username)
	if dutils.IsFileExist(config) {
		return
	}

	err := os.MkdirAll(userConfigDir, 0755)
	if err != nil {
		logger.Warning("Failed to mkdir for user config:", err)
		return
	}

	oldConfig := path.Join(actConfigDir, "users", username)
	err = dutils.CopyFile(oldConfig, config)
	if err != nil {
		logger.Warning("Failed to update config.")
	}
}

// 从配置文件中加载用户的配置信息
func loadUserConfigInfo(u *User) {
	var err error

	u.IconList = u.getAllIcons()

	// NOTICE(jouyouyun): Got created time,  not accurate, can only be used as a reference
	u.CreatedTime, err = u.getCreatedTime()
	if err != nil {
		logger.Warning("Failed to get created time:", err)
	}

	kf, err := dutils.NewKeyFileFromFile(
		path.Join(userConfigDir, u.UserName))
	if err != nil {
		xSession, _ := users.GetDefaultXSession()
		u.XSession = xSession
		u.SystemAccount = false
		u.Layout = getDefaultLayout()
		u.setLocale(getDefaultLocale())
		u.IconFile = defaultUserIcon
		defaultUserBackground := getDefaultUserBackground()
		u.DesktopBackgrounds = []string{defaultUserBackground}
		u.GreeterBackground = defaultUserBackground
		u.Use24HourFormat = defaultUse24HourFormat
		u.UUID = dutils.GenUuid()
		u.WeekdayFormat = defaultWeekdayFormat
		u.ShortDateFormat = defaultShortDateFormat
		u.LongDateFormat = defaultLongDateFormat
		u.ShortTimeFormat = defaultShortTimeFormat
		u.LongTimeFormat = defaultLongTimeFormat
		u.WeekBegins = defaultWeekBegins
		u.Workspace = defaultWorkspace

		err = u.writeUserConfig()
		if err != nil {
			logger.Warning(err)
		}

		return
	}
	defer kf.Free()

	var isSave = false
	xSession, _ := kf.GetString(confGroupUser, confKeyXSession)
	u.XSession = xSession
	if u.XSession == "" {
		xSession, _ = users.GetDefaultXSession()
		u.XSession = xSession
		isSave = true
	}
	_, err = kf.GetBoolean(confGroupUser, confKeySystemAccount)
	// only show non system account
	u.SystemAccount = false
	if err != nil {
		isSave = true
	}
	locale, _ := kf.GetString(confGroupUser, confKeyLocale)
	u.setLocale(locale)
	if locale == "" {
		u.setLocale(getDefaultLocale())
		isSave = true
	}
	layout, _ := kf.GetString(confGroupUser, confKeyLayout)
	u.Layout = layout
	if layout == "" {
		u.Layout = getDefaultLayout()
		isSave = true
	}
	icon, _ := kf.GetString(confGroupUser, confKeyIcon)
	u.IconFile = icon
	if u.IconFile == "" {
		u.IconFile = defaultUserIcon
		isSave = true
	}

	u.customIcon, _ = kf.GetString(confGroupUser, confKeyCustomIcon)

	// CustomIcon is the newly added field in the configuration file
	if u.customIcon == "" {
		if u.IconFile != defaultUserIcon && !isStrInArray(u.IconFile, u.IconList) {
			// u.IconFile is a custom icon, not a standard icon
			u.customIcon = u.IconFile
			isSave = true
		}
	}

	u.IconList = u.getAllIcons()

	_, desktopBgs, _ := kf.GetStringList(confGroupUser, confKeyDesktopBackgrounds)
	u.DesktopBackgrounds = desktopBgs
	if len(desktopBgs) == 0 {
		u.DesktopBackgrounds = []string{getDefaultUserBackground()}
		isSave = true
	}

	greeterBg, ok := getUserGreeterBackground(kf)
	if ok {
		u.GreeterBackground = greeterBg
	} else {
		u.GreeterBackground = getDefaultUserBackground()
		isSave = true
	}

	_, u.HistoryLayout, _ = kf.GetStringList(confGroupUser, confKeyHistoryLayout)
	if !strv.Strv(u.HistoryLayout).Contains(u.Layout) {
		u.HistoryLayout = append(u.HistoryLayout, u.Layout)
		isSave = true
	}

	u.Use24HourFormat, err = kf.GetBoolean(confGroupUser, confKeyUse24HourFormat)
	if err != nil {
		u.Use24HourFormat = defaultUse24HourFormat
		isSave = true
	}

	u.WeekdayFormat, err = kf.GetInteger(confGroupUser, confKeyWeekdayFormat)
	if err != nil {
		u.WeekdayFormat = defaultWeekdayFormat
		isSave = true
	}

	u.ShortDateFormat, err = kf.GetInteger(confGroupUser, confKeyShortDateFormat)
	if err != nil {
		u.ShortDateFormat = defaultShortDateFormat
		isSave = true
	}

	u.LongDateFormat, err = kf.GetInteger(confGroupUser, confKeyLongDateFormat)
	if err != nil {
		u.LongDateFormat = defaultLongDateFormat
		isSave = true
	}

	u.ShortTimeFormat, err = kf.GetInteger(confGroupUser, confKeyShortTimeFormat)
	if err != nil {
		u.ShortTimeFormat = defaultShortTimeFormat
		isSave = true
	}

	u.LongTimeFormat, err = kf.GetInteger(confGroupUser, confKeyLongTimeFormat)
	if err != nil {
		u.LongTimeFormat = defaultLongTimeFormat
		isSave = true
	}

	u.WeekBegins, err = kf.GetInteger(confGroupUser, confKeyWeekBegins)
	if err != nil {
		u.WeekBegins = defaultWeekBegins
		isSave = true
	}

	u.UUID, err = kf.GetString(confGroupUser, confKeyUUID)
	if err != nil || u.UUID == "" {
		u.UUID = dutils.GenUuid()
		isSave = true
	}

	u.Workspace, err = kf.GetInteger(confGroupUser, confKeyWorkspace)
	if err != nil || u.Workspace == 0 {
		u.Workspace = defaultWorkspace
		isSave = true
	}

	passwordHint, err := kf.GetString(confGroupUser, confKeyPasswordHint)
	if err != nil {
		u.PasswordHint = ""
	} else {
		decodeHint, err := base64.StdEncoding.DecodeString(passwordHint)
		if err != nil {
			logger.Warning(err)
			u.PasswordHint = ""
		} else {
			u.PasswordHint = string(decodeHint)
		}
	}

	if isSave {
		err := u.writeUserConfig()
		if err != nil {
			logger.Warning(err)
		}
	}

	u.checkLeftSpace()
}
