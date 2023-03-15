// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	"github.com/linuxdeepin/go-lib/appinfo/desktopappinfo"
	"github.com/linuxdeepin/go-lib/gettext"
)

const desktopHashPrefix = "d:"

type AppInfo struct {
	filename       string
	id             string
	icon           string
	identifyMethod string
	innerId        string
	name           string
	actions        []desktopAction
	isInstalled    bool
}

type desktopAction struct {
	Section string
	Name    string
}

func newAppInfo(dai *desktopappinfo.DesktopAppInfo) *AppInfo {
	if dai == nil {
		return nil
	}
	ai := &AppInfo{}
	xDeepinVendor, _ := dai.GetString(desktopappinfo.MainSection, "X-Deepin-Vendor")
	if xDeepinVendor == "deepin" {
		ai.name = dai.GetGenericName()
		if ai.name == "" {
			ai.name = dai.GetName()
		}
	} else {
		ai.name = dai.GetName()
	}

	enableLinglongSuffix := false
	getEnableLinglongSuffix := func() {
		dc, err := dconfig.NewDConfig("org.deepin.dde.daemon", "org.deepin.dde.daemon.application", "")
		if err != nil {
			logger.Warning("new dconfig error:", err)
			return
		}
		if dc == nil {
			logger.Warning("new dconfig error: dconfig is nil.")
			return
		}
		result, err := dc.GetValueBool("LinglongAppNameSuffixEnable")
		if err != nil {
			logger.Warning("failed to get dconfig LinglongAppNameSuffixEnable:", err)
			return
		}
		enableLinglongSuffix = result
	}
	getEnableLinglongSuffix()

	if strings.Contains(dai.GetId(), "Linglong") && enableLinglongSuffix {
		ai.name = ai.name + gettext.Tr("(Linglong)")
	}
	ai.innerId = genInnerIdWithDesktopAppInfo(dai)
	ai.filename = dai.GetFileName()
	ai.id = dai.GetId()
	ai.icon = dai.GetIcon()
	ai.isInstalled = dai.IsInstalled()
	actions := dai.GetActions()
	for _, act := range actions {
		ai.actions = append(ai.actions, desktopAction{
			Section: act.Section,
			Name:    act.Name,
		})
	}
	return ai
}

func getDockedDesktopAppInfo(app string) *desktopappinfo.DesktopAppInfo {
	if app[0] != '/' || len(app) <= 3 {
		return desktopappinfo.NewDesktopAppInfo(app)
	}

	absPath := unzipDesktopPath(app)
	ai, err := desktopappinfo.NewDesktopAppInfoFromFile(absPath)
	if err != nil {
		logger.Warning(err)
		return nil
	}
	return ai
}

func NewDockedAppInfo(app string) *AppInfo {
	if app == "" {
		return nil
	}
	return newAppInfo(getDockedDesktopAppInfo(app))
}

func NewAppInfo(id string) *AppInfo {
	if id == "" {
		return nil
	}
	return newAppInfo(desktopappinfo.NewDesktopAppInfo(id))
}

func NewAppInfoFromFile(file string) *AppInfo {
	if file == "" {
		return nil
	}
	dai, _ := desktopappinfo.NewDesktopAppInfoFromFile(file)
	if dai == nil {
		return nil
	}

	if !dai.IsInstalled() {
		appId, _ := dai.GetString(desktopappinfo.MainSection, "X-Deepin-AppID")
		if appId != "" {
			dai1 := desktopappinfo.NewDesktopAppInfo(appId)
			if dai1 != nil {
				dai = dai1
			}
		}
	}
	return newAppInfo(dai)
}

func (ai *AppInfo) GetFileName() string {
	return ai.filename
}

func (ai *AppInfo) GetIcon() string {
	return ai.icon
}

func (ai *AppInfo) GetId() string {
	return ai.id
}

func (ai *AppInfo) GetActions() []desktopAction {
	return ai.actions
}

func (ai *AppInfo) IsInstalled() bool {
	return ai.isInstalled
}

func genInnerIdWithDesktopAppInfo(dai *desktopappinfo.DesktopAppInfo) string {
	cmdline := dai.GetCommandline()
	hasher := md5.New()
	_, err := hasher.Write([]byte(cmdline))
	if err != nil {
		logger.Warning("Write error:", err)
	}
	return desktopHashPrefix + hex.EncodeToString(hasher.Sum(nil))
}

func (ai *AppInfo) String() string {
	if ai == nil {
		return "<nil>"
	}
	desktopFile := ai.GetFileName()
	icon := ai.GetIcon()
	id := ai.GetId()
	return fmt.Sprintf("<AppInfo id=%q hash=%q icon=%q desktop=%q>", id, ai.innerId, icon, desktopFile)
}
