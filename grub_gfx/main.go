// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub_gfx

import (
	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/grub_common"
	ofd "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
)

func detectChange() {
	if !grub_common.HasDeepinGfxmodeMod() {
		logger.Debug("not found grub module deepin_gfxmode")
		return
	}

	params := grub_common.LoadGrubParams()
	logger.Debugf("grub params: %+v", params)
	if grub_common.ShouldFinishGfxmodeDetect(params) {
		logger.Debug("finish gfxmode detect")
		err := startSysGrubService()
		if err != nil {
			logger.Warning("failed to start sys-grub service:", err)
		}
		return
	}
	if grub_common.InGfxmodeDetectionMode(params) {
		logger.Debug("in gfxmode detection mode")
		return
	}

	currentGfxmode, allGrubGfxmodes, err := grub_common.GetBootArgDeepinGfxmode()
	if err != nil {
		logger.Warning("failed to get boot arg DEEPIN_GFXMODE:", err)
		if !grub_common.IsGfxmodeDetectFailed(params) {
			err = prepareGfxmodeDetect()
			if err != nil {
				logger.Warning(err)
			}
		}
		return
	}
	logger.Debug("currentGfxmode:", currentGfxmode)
	logger.Debug("allGrubGfxmodes:", allGrubGfxmodes)

	drmGfxmodes, err := grub_common.GetGfxmodesFromSysDrm()
	if err != nil {
		logger.Warning(err)
		return
	}
	logger.Debug("Gfxmodes from sys drm:", drmGfxmodes)

	maxGfxmode := drmGfxmodes.Intersection(allGrubGfxmodes).Max()

	cfgGfxmodeStr := grub_common.DecodeShellValue(params["GRUB_GFXMODE"])
	logger.Debug("cfgGfxmodeStr:", cfgGfxmodeStr)
	cfgGfxmode, cfgGfxmodeErr := grub_common.ParseGfxmode(cfgGfxmodeStr)
	if cfgGfxmodeErr != nil {
		logger.Warning("failed to parse cfgGfxmodeStr:", cfgGfxmodeErr)
	}

	edidsHash, err := grub_common.GetConnectedEdidsHash()
	if err != nil {
		logger.Warning("failed to get connected edids hash:", err)
	}
	logger.Debug("current edids hash:", edidsHash)

	// detectCacheStale is true when the cache is missing, unreadable, or no longer
	// matches the current EDID hash and maximum gfxmode, meaning a fresh detection
	// may be required.
	detectCacheStale := true
	detectCache, err := loadDetectCache()
	logger.Debugf("loaded detect cache: %+v", detectCache)
	if err == nil && detectCache.equal(edidsHash, maxGfxmode) {
		// cache is still valid — no detection needed due to cache staleness
		detectCacheStale = false
	}

	need := needDetect(cfgGfxmode, cfgGfxmodeErr, currentGfxmode, maxGfxmode, detectCacheStale)
	if !need {
		return
	}
	err = prepareGfxmodeDetect()
	if err != nil {
		logger.Warning("failed to prepare gfxmode detect:", err)
		return
	}

	// save detect cache
	if err := saveDetectCache(DetectCache{
		EdidsHash:  edidsHash,
		MaxGfxmode: maxGfxmode.String(),
	}); err != nil {
		logger.Warning("failed to save detect cache:", err)
	}
}

// needDetect reports whether a gfxmode detection run is required.
// It returns true in any of the following cases:
//   - the configured gfxmode could not be parsed (cfgGfxmodeErr != nil)
//   - the configured gfxmode differs from the currently active gfxmode
//   - the current gfxmode is not the maximum supported value and the detect cache is stale
func needDetect(cfgGfxmode grub_common.Gfxmode, cfgGfxmodeErr error,
	currentGfxmode, maxGfxmode grub_common.Gfxmode, detectCacheStale bool) bool {

	// condCfgGfxmodeErr: config gfxmode is invalid or missing
	condCfgGfxmodeErr := cfgGfxmodeErr != nil
	// condCfgNeqCurrent: config gfxmode is out of sync with the current gfxmode
	condCfgNeqCurrent := cfgGfxmode != currentGfxmode
	// condCurrentNeqMaxAndCacheStale: current gfxmode may be upgradeable and the cache is stale
	condCurrentNeqMaxAndCacheStale := currentGfxmode != maxGfxmode && detectCacheStale
	logger.Debugf("needDetect: cfgGfxmodeErr != nil: %v (cfgGfxmodeErr: %v)", condCfgGfxmodeErr, cfgGfxmodeErr)
	logger.Debugf("needDetect: cfgGfxmode != currentGfxmode: %v (cfgGfxmode: %v, currentGfxmode: %v)", condCfgNeqCurrent, cfgGfxmode, currentGfxmode)
	logger.Debugf("needDetect: currentGfxmode != maxGfxmode && detectCacheStale: %v (currentGfxmode: %v, maxGfxmode: %v, detectCacheStale: %v)", condCurrentNeqMaxAndCacheStale, currentGfxmode, maxGfxmode, detectCacheStale)
	result := condCfgGfxmodeErr || condCfgNeqCurrent || condCurrentNeqMaxAndCacheStale
	logger.Debug("needDetect result:", result)
	return result
}

func startSysGrubService() error {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	sysBusDaemon := ofd.NewDBus(sysBus)
	_, err = sysBusDaemon.StartServiceByName(dbus.FlagNoAutoStart,
		"org.deepin.dde.Grub2", 0)
	return err
}

func getSysGrubObj() (dbus.BusObject, error) {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	obj := sysBus.Object("org.deepin.dde.Grub2", "/org/deepin/dde/Grub2")
	return obj, nil
}

func prepareGfxmodeDetect() error {
	logger.Debug("prepare gfxmode detect")
	sysGrubObj, err := getSysGrubObj()
	if err != nil {
		return err
	}

	return sysGrubObj.Call("org.deepin.dde.Grub2.PrepareGfxmodeDetect", 0).Err
}
