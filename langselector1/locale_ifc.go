// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package langselector

import (
	"errors"
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-api/language_support"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	dbusPath      = "/org/deepin/dde/LangSelector1"
	dbusInterface = "org.deepin.dde.LangSelector1"

	localeIconStart    = "notification-change-language-start"
	localeIconFailed   = "notification-change-language-failed"
	localeIconFinished = "notification-change-language-finished"
)

// Set user desktop environment locale, the new locale will work after relogin.
// (Notice: this locale is only for the current user.)
//
// 设置用户会话的 locale，注销后生效，此改变只对当前用户生效。
//
// locale: see '/etc/locale.gen'
func (lang *LangSelector) SetLocale(locale string) *dbus.Error {
	lang.service.DelayAutoQuit()

	if !lang.isSupportedLocale(locale) {
		return dbusutil.ToError(fmt.Errorf("invalid locale: %v", locale))
	}

	lang.PropsMu.Lock()
	defer lang.PropsMu.Unlock()
	if lang.LocaleState == LocaleStateChanging || lang.CurrentLocale == locale {
		return nil
	}
	_ = lang.addLocale(locale)
	logger.Debugf("setLocale %q", locale)
	go lang.setLocale(locale)
	return nil
}

// Get locale info list that deepin supported
//
// 得到系统支持的 locale 信息列表
func (lang *LangSelector) GetLocaleList() (locales []LocaleInfo, busErr *dbus.Error) {
	lang.service.DelayAutoQuit()
	return lang.getCachedLocales(), nil
}

func (lang *LangSelector) GetLocaleDescription(locale string) (description string, busErr *dbus.Error) {
	lang.service.DelayAutoQuit()

	infos := lang.getCachedLocales()
	info, err := infos.Get(locale)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	return info.Desc, nil
}

// Reset set user desktop environment locale to system default locale
func (lang *LangSelector) Reset() *dbus.Error {
	lang.service.DelayAutoQuit()

	locale, err := getLocaleFromFile(systemLocaleFile)
	if err != nil {
		return dbusutil.ToError(err)
	}
	return lang.SetLocale(locale)
}

func (lang *LangSelector) GetLanguageSupportPackages(locale string) (packages []string, busErr *dbus.Error) {
	lang.service.DelayAutoQuit()

	ls, err := language_support.NewLanguageSupport()
	if err != nil {
		return nil, dbusutil.ToError(err)
	}

	packages = ls.ByLocale(locale, false)
	ls.Destroy()
	return packages, nil
}

func (lang *LangSelector) AddLocale(locale string) *dbus.Error {
	lang.service.DelayAutoQuit()

	if !lang.isSupportedLocale(locale) {
		return dbusutil.ToError(fmt.Errorf("invalid locale: %v", locale))
	}

	err := lang.addLocale(locale)
	return dbusutil.ToError(err)
}

func (lang *LangSelector) DeleteLocale(locale string) *dbus.Error {
	lang.service.DelayAutoQuit()

	lang.PropsMu.RLock()
	if locale == lang.CurrentLocale {
		lang.PropsMu.RUnlock()
		return dbusutil.ToError(errors.New("the locale is being used"))
	}
	lang.PropsMu.RUnlock()

	err := lang.deleteLocale(locale)
	return dbusutil.ToError(err)
}
