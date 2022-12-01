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

package langselector

import (
	"time"

	gio "github.com/linuxdeepin/go-gir/gio-2.0"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gsettings"
	"github.com/linuxdeepin/go-lib/log"
	"github.com/linuxdeepin/go-lib/strv"
)

const (
	dbusServiceName = "org.deepin.dde.LangSelector1"
)

var (
	logger = log.NewLogger("daemon/langselector")
)

func Run() {
	service, err := dbusutil.NewSessionService()
	if err != nil {
		logger.Fatal("failed to new session service:", err)
	}

	lang, err := newLangSelector(service)
	if err != nil {
		logger.Fatal("failed to new langSelector:", err)
	}

	err = service.Export(dbusPath, lang)
	if err != nil {
		logger.Fatal("failed to export:", err)
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		logger.Fatal("failed to request name:", err)
	}

	initNotifyTxt()
	lang.connectSettingsChanged()
	err = gsettings.StartMonitor()
	if err != nil {
		logger.Warning("failed to start monitor settings:", err)
	}
	service.SetAutoQuitHandler(time.Minute*5, func() bool {
		lang.PropsMu.RLock()
		canQuit := lang.LocaleState != LocaleStateChanging
		lang.PropsMu.RUnlock()
		return canQuit
	})
	service.Wait()
}

func GetLocales() []string {
	currentLocale := getCurrentUserLocale()
	settings := gio.NewSettings(gsSchemaLocale)
	locales := settings.GetStrv(gsKeyLocales)
	if !strv.Strv(locales).Contains(currentLocale) {
		locales = append(locales, currentLocale)
	}
	settings.Unref()
	return locales
}
