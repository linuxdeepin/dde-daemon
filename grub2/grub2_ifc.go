// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub2

import (
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/grub_common"
	"github.com/linuxdeepin/dde-daemon/securityloader"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

const (
	dbusServiceName = "org.deepin.dde.Grub2"
	dbusPath        = "/org/deepin/dde/Grub2"
	dbusInterface   = dbusServiceName

	polikitActionIdCommon               = "org.deepin.dde.grub2"
	polikitActionIdPrepareGfxmodeDetect = "org.deepin.dde.grub2.prepare-gfxmode-detect"

	timeoutMax = 10
)

func (*Grub2) GetInterfaceName() string {
	return dbusInterface
}

// SetAllowCaller registers a security-loader launched caller, so that its
// calls on this interface skip the Polkit check. The D-Bus policy only lets
// the deepin-daemon group reach this method.
func (g *Grub2) SetAllowCaller(sender dbus.Sender, uniqueName string) *dbus.Error {
	return dbusutil.ToError(g.allowCallers.AddCaller(securityloader.Grub2Scope, sender, uniqueName))
}

// GetSimpleEntryTitles return entry titles only in level one and will
// filter out some useless entries such as sub-menus and "memtest86+".
func (grub *Grub2) GetSimpleEntryTitles(sender dbus.Sender) (titles []string, busErr *dbus.Error) {
	grub.service.DelayAutoQuit()

	grub.readEntries()

	for _, entry := range grub.entries {
		if entry.parentSubMenu == nil && entry.entryType == MENUENTRY {
			title := entry.getFullTitle()
			if !strings.Contains(title, "memtest86+") {
				titles = append(titles, title)
			}
		}
	}
	if len(titles) == 0 {
		logger.Warningf("there is no menu entry in %q", grubScriptFile)
	}
	return titles, nil
}

func (g *Grub2) GetAvailableGfxmodes() (gfxModes []string, busErr *dbus.Error) {
	// Read-only operation, no authentication required
	// Get gfxmodes from /sys/class/drm (works for all session types)
	g.service.DelayAutoQuit()
	modes, err := g.getAvailableGfxmodes()
	if err != nil {
		logger.Warning("failed to get available gfxmodes:", err)
		return nil, dbusutil.ToError(err)
	}
	modes.SortDesc()
	gfxModes = make([]string, len(modes))
	for idx, m := range modes {
		gfxModes[idx] = m.String()
	}

	return gfxModes, nil
}

func (g *Grub2) SetDefaultEntry(sender dbus.Sender, entry string) *dbus.Error {
	err := checkInvokePermission(g.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}
	g.service.DelayAutoQuit()

	err = g.checkAuth(sender, polikitActionIdCommon)
	if err != nil {
		return dbusutil.ToError(err)
	}

	idx := g.defaultEntryStr2Idx(entry)
	if idx == -1 {
		return dbusutil.ToError(errors.New("invalid entry"))
	}

	g.PropsMu.Lock()
	if g.setPropDefaultEntry(entry) {
		g.addModifyTask(getModifyTaskDefaultEntry(idx))
	}
	g.PropsMu.Unlock()
	return nil
}

var errInGfxmodeDetect = errors.New("in gfxmode detection mode")

func (g *Grub2) SetEnableTheme(sender dbus.Sender, enabled bool) *dbus.Error {
	err := checkInvokePermission(g.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}
	g.service.DelayAutoQuit()

	err = g.checkAuth(sender, polikitActionIdCommon)
	if err != nil {
		return dbusutil.ToError(err)
	}

	lang, err := g.getSenderLang(sender)
	if err != nil {
		logger.Warning("failed to get sender lang:", err)
	}

	g.PropsMu.Lock()

	if g.setPropEnableTheme(enabled) {
		var themeFile string
		if enabled {
			if g.gfxmodeDetectState == gfxmodeDetectStateNone {
				themeFile = defaultGrubTheme
			} else {
				themeFile = fallbackGrubTheme
			}
		}
		g.setPropThemeFile(themeFile)
		task := getModifyTaskEnableTheme(enabled, lang, g.gfxmodeDetectState)
		logger.Debugf("SetEnableTheme task: %#v", task)
		g.addModifyTask(task)
	}

	g.PropsMu.Unlock()
	return nil
}

func (g *Grub2) SetGfxmode(sender dbus.Sender, gfxmode string) *dbus.Error {
	err := checkInvokePermission(g.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}
	g.service.DelayAutoQuit()

	err = g.checkAuth(sender, polikitActionIdCommon)
	if err != nil {
		return dbusutil.ToError(err)
	}

	err = checkGfxmode(gfxmode)
	if err != nil {
		return dbusutil.ToError(err)
	}

	lang, err := g.getSenderLang(sender)
	if err != nil {
		logger.Warning("failed to get sender lang:", err)
	}

	// PrepareGfxmodeDetect holds this mutex while it checks state and
	// gathers modes. Without it, a setter could read None in the window
	// before PrepareGfxmodeDetect switches to Detecting, return success,
	// and then have its task overwritten by the detection task.
	g.gfxmodeDetectMu.Lock()
	defer g.gfxmodeDetectMu.Unlock()

	g.PropsMu.Lock()
	if g.gfxmodeDetectState == gfxmodeDetectStateDetecting || g.gfxmodeDetectStarting {
		g.PropsMu.Unlock()
		return dbusutil.ToError(errInGfxmodeDetect)
	}

	changed := g.setPropGfxmode(gfxmode)
	g.PropsMu.Unlock()

	if changed {
		g.addModifyTask(getModifyTaskGfxmode(gfxmode, lang))
	}
	return nil
}

func (g *Grub2) SetTimeout(sender dbus.Sender, timeout uint32) *dbus.Error {
	err := checkInvokePermission(g.service, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}
	g.service.DelayAutoQuit()

	/*
		err := g.checkAuth(sender, polikitActionIdCommon)
		if err != nil {
			return dbusutil.ToError(err)
		}
	*/

	if timeout > timeoutMax {
		return dbusutil.ToError(errors.New("exceeded the maximum value"))
	}

	g.PropsMu.Lock()
	if g.setPropTimeout(timeout) {
		g.addModifyTask(getModifyTaskTimeout(timeout))
	}
	g.PropsMu.Unlock()
	return nil
}

// Reset reset all configuration.
func (g *Grub2) Reset(sender dbus.Sender) *dbus.Error {
	g.service.DelayAutoQuit()

	const defaultEnableTheme = true

	err := g.checkAuth(sender, polikitActionIdCommon)
	if err != nil {
		return dbusutil.ToError(err)
	}

	lang, err := g.getSenderLang(sender)
	if err != nil {
		logger.Warning("failed to get sender lang:", err)
	}

	var modifyTasks []modifyTask

	g.PropsMu.Lock()
	if g.setPropTimeout(defaultGrubTimeoutInt) {
		modifyTasks = append(modifyTasks, getModifyTaskTimeout(defaultGrubTimeoutInt))
	}

	if g.setPropEnableTheme(defaultEnableTheme) {
		modifyTasks = append(modifyTasks,
			getModifyTaskEnableTheme(defaultEnableTheme, lang, g.gfxmodeDetectState))
	}

	g.setPropThemeFile(defaultGrubTheme)

	cfgDefaultEntry, _ := g.defaultEntryIdx2Str(defaultGrubDefaultInt)
	if g.setPropDefaultEntry(cfgDefaultEntry) {
		modifyTasks = append(modifyTasks, getModifyTaskDefaultEntry(defaultGrubDefaultInt))
	}
	g.PropsMu.Unlock()

	if len(modifyTasks) > 0 {
		compoundModifyFunc := func(params map[string]string) {
			for _, task := range modifyTasks {
				task.paramsModifyFunc(params)
			}
		}
		g.addModifyTask(modifyTask{
			paramsModifyFunc: compoundModifyFunc,
			adjustTheme:      true,
		})
	}

	return nil
}

func (g *Grub2) PrepareGfxmodeDetect(sender dbus.Sender) *dbus.Error {
	if !noCheckAuth {
		err := securityloader.AuthorizeWithPolkit(
			g.allowCallers,
			securityloader.Grub2Scope,
			sender,
			g.service.Conn(),
			polikitActionIdPrepareGfxmodeDetect,
		)
		if err != nil {
			logger.Warningf("PrepareGfxmodeDetect denied for %v: %v", sender, err)
			return dbusutil.ToError(err)
		}
	}

	// Detection rewrites the boot configuration, so two calls must never
	// schedule overlapping modify tasks. Serialise the whole method; the
	// state check below is only meaningful while no other call can slip in
	// between reading gfxmodeDetectState and setting it to Detecting.
	g.gfxmodeDetectMu.Lock()
	defer g.gfxmodeDetectMu.Unlock()

	// Reserve the startup transition before reading DRM/GRUB state. A
	// setter that only checked gfxmodeDetectState would otherwise see None
	// while this method is still gathering modes, enqueue its own task and
	// return success just before the Detecting state is committed.
	g.PropsMu.Lock()
	gfxmodeDetectState := g.gfxmodeDetectState
	if gfxmodeDetectState == gfxmodeDetectStateDetecting {
		g.PropsMu.Unlock()
		return dbusutil.ToError(errors.New("already in detection mode"))
	}
	g.gfxmodeDetectStarting = true
	g.PropsMu.Unlock()

	g.service.DelayAutoQuit()

	gfxmodes, err := grub_common.GetGfxmodesFromSysDrm()
	if err != nil {
		logger.Debug("failed to get gfxmodes from sys drm:", err)
	}
	logger.Debug("Gfxmodes from drm:", gfxmodes)

	gfxmodes.SortDesc()
	gfxmodesStr := joinGfxmodesForDetect(gfxmodes)
	logger.Debug("Gfxmodes str for detect:", gfxmodesStr)

	defaultParams := grub_common.LoadGrubParams()

	params := make(map[string]string)
	for _, key := range []string{grubBackground, grubGfxmode, grubTheme, grubTimeout} {
		if v, ok := defaultParams[key]; ok {
			params[key] = v
		}
	}

	if gfxmodeDetectState == gfxmodeDetectStateFailed {
		cur, _, err := grub_common.GetBootArgDeepinGfxmode()
		if err == nil {
			if params[grubGfxmode] == gfxmodesStr &&
				(len(gfxmodes) > 0 && gfxmodes[0] == cur) {
				g.finishGfxmodeDetect(params)
				return nil
			}
		}
	}

	availableGfxmodes, err := g.getAvailableGfxmodes()
	if err != nil {
		logger.Warning("failed to get available gfxmodes:", err)
	} else if len(availableGfxmodes) == 1 {
		// If available gfxmodes only has one, then check if it
		// is the same as current gfxmode
		currentGfxmode, _, err := grub_common.GetBootArgDeepinGfxmode()
		if err != nil {
			logger.Warning("failed to get current gfxmode:", err)
		} else if currentGfxmode == availableGfxmodes[0] {
			// Complete detection immediately
			g.finishGfxmodeDetect(params)
			return nil
		}
	}

	themeFile := getTheme(params)
	g.PropsMu.Lock()
	g.gfxmodeDetectStarting = false
	g.gfxmodeDetectState = gfxmodeDetectStateDetecting
	if themeFile != "" {
		g.setPropThemeFile(fallbackGrubTheme)
		g.setPropEnableTheme(true)
	} else {
		g.setPropThemeFile("")
		g.setPropEnableTheme(false)
	}
	g.setPropGfxmode(gfxmodesStr)
	g.PropsMu.Unlock()
	g.theme.emitSignalBackgroundChanged()

	g.addModifyTask(getModifyTaskPrepareGfxmodeDetect(gfxmodesStr))

	err = grub_common.CreateGfxmodeDetectReady()
	if err != nil {
		return dbusutil.ToError(err)
	}
	return nil
}
