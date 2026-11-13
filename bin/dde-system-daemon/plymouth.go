// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"fmt"
	"github.com/linuxdeepin/dde-daemon/common/systemdunit"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	configManager "github.com/linuxdeepin/go-dbus-factory/org.desktopspec.ConfigManager"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

var plymouthLocker sync.Mutex

const (
	dsettingsAppID                       = "org.deepin.dde.daemon"
	dsettingsAppearanceName              = "org.deepin.dde.daemon.appearance"
	dsettingsScaleWithoutPlymouthEnabled = "scaleWithoutPlymouthEnabled"
	plymouthSetDefaultUnit               = "dde-set-default-plymouth-theme.service"
	updateInitramfsUnit                  = "dde-update-initramfs.service"

	polkitActionSetPlymouthTheme = "org.deepin.dde.daemon.set-plymouth-theme"
	polkitActionScalePlymouth    = "org.deepin.dde.daemon.scale-plymouth"

	// plymouthThemeMaxLength bounds the theme name taken from D-Bus. Real
	// theme names are far shorter.
	plymouthThemeMaxLength = 64
)

func (d *Daemon) getScaleWithoutPlymouthEnabled() bool {
	ds := configManager.NewConfigManager(d.systemSigLoop.Conn())

	appearancePath, err := ds.AcquireManager(0, dsettingsAppID, dsettingsAppearanceName, "")
	if err != nil {
		logger.Warning(err)
		return false
	}

	dsAppearance, err := configManager.NewManager(d.systemSigLoop.Conn(), appearancePath)
	if err != nil {
		logger.Warning(err)
		return false
	}

	v, err := dsAppearance.Value(0, dsettingsScaleWithoutPlymouthEnabled)
	if err != nil {
		logger.Warning(err)
		return false
	}

	if enabled, ok := v.Value().(bool); ok {
		return enabled
	}

	return false
}

func (d *Daemon) ScalePlymouth(sender dbus.Sender, scale uint32) *dbus.Error {
	if err := d.authorize(sender, polkitActionScalePlymouth); err != nil {
		logger.Warningf("ScalePlymouth authorization failed: %q", err.Error())
		return dbusutil.ToError(err)
	}
	return dbusutil.ToError(d.scalePlymouth(scale))
}

func (d *Daemon) scalePlymouth(scale uint32) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	if systemdunit.CheckUnitExist(conn, plymouthSetDefaultUnit) || systemdunit.CheckUnitExist(d.service.Conn(), updateInitramfsUnit) {
		return fmt.Errorf("another plymouth setting service is running")
	}
	if d.getScaleWithoutPlymouthEnabled() {
		logger.Info("skip scale plymouth")
		return nil
	}

	plymouthLocker.Lock()
	defer plymouthLocker.Unlock()
	defer logger.Debug("end ScalePlymouth", scale)

	edition, err := getEditionName()
	if err != nil {
		return err
	}
	var themeNames map[uint32]string
	if edition == "Community" {
		themeNames = map[uint32]string{
			1: "deepin-ssd-logo",
			2: "deepin-hidpi-ssd-logo",
		}
	} else {
		themeNames = map[uint32]string{
			1: "uos-ssd-logo",
			2: "uos-hidpi-ssd-logo",
		}
	}
	name, ok := themeNames[scale]
	if !ok {
		return fmt.Errorf("invalid scale value: %d", scale)
	}
	path, err := exec.LookPath("plymouth-set-default-theme")
	if err != nil {
		return fmt.Errorf("could not find plymouth-set-default-theme")
	}
	plymouthUnit := systemdunit.TransientUnit{
		Dbus:        conn,
		UnitName:    plymouthSetDefaultUnit,
		Type:        "oneshot",
		Description: "Transient Unit Set Default Plymouth Theme",
		Environment: []string{},
		Commands:    []string{path, name},
	}
	err = plymouthUnit.StartTransientUnit()
	if err != nil {
		return fmt.Errorf("failed create unit: %v, err: %v", plymouthSetDefaultUnit, err)
	}
	if !plymouthUnit.WaitforFinish(d.systemSigLoop) {
		return fmt.Errorf("%v run failed", plymouthUnit.UnitName)
	}

	kernel, err := exec.Command("uname", "-r").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get kernel, err: %v", err)
	}

	path, err = exec.LookPath("update-initramfs")
	if err != nil {
		return fmt.Errorf("could not find plymouth-set-default-theme")
	}
	updateInitramfsUnit := systemdunit.TransientUnit{
		Dbus:        conn,
		UnitName:    updateInitramfsUnit,
		Type:        "oneshot",
		Description: "Transient Unit Update Initramfs",
		Environment: []string{},
		Commands:    []string{path, "-u", "-k", string(bytes.TrimSpace(kernel))},
	}
	err = updateInitramfsUnit.StartTransientUnit()
	if err != nil {
		return fmt.Errorf("failed create unit: %v, err: %v", updateInitramfsUnit, err)
	}
	if !updateInitramfsUnit.WaitforFinish(d.systemSigLoop) {
		return fmt.Errorf("%v run failed", updateInitramfsUnit.UnitName)
	}

	return nil
}

// isValidPlymouthThemeName reports whether name can be a plymouth theme
// directory name. It rejects an empty or over-long name and anything that
// plymouth-set-default-theme could read as an option or a path.
func isValidPlymouthThemeName(name string) bool {
	if name == "" || len(name) > plymouthThemeMaxLength {
		return false
	}
	if name[0] == '-' || name[0] == '.' {
		return false
	}
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return true
}

// checkPlymouthThemeName requires themeName to be exactly one of the names
// printed by `plymouth-set-default-theme --list`. The previous substring test
// also accepted fragments of a real name and the empty string.
func checkPlymouthThemeName(themelist, themeName string) error {
	if !isValidPlymouthThemeName(themeName) {
		return fmt.Errorf("invalid plymouth theme name %q", themeName)
	}
	for _, name := range strings.Fields(themelist) {
		if name == themeName {
			return nil
		}
	}
	return fmt.Errorf("plymouth theme name %q does not exist in theme list", themeName)
}

// SetPlymouthTheme changes the boot theme and rebuilds the initramfs. Both
// commands run as root, so the caller is authorized first: callers registered
// with the security loader skip the prompt, everyone else needs the polkit
// action.
func (d *Daemon) SetPlymouthTheme(sender dbus.Sender, themeName string) *dbus.Error {
	if err := d.authorize(sender, polkitActionSetPlymouthTheme); err != nil {
		logger.Warningf("SetPlymouthTheme authorization failed: %q", err.Error())
		return dbusutil.ToError(err)
	}
	return dbusutil.ToError(d.setPlymouthTheme(themeName))
}

func (d *Daemon) setPlymouthTheme(themeName string) error {
	plymouthLocker.Lock()
	defer plymouthLocker.Unlock()
	defer logger.Debug("end SetPlymouthTheme", themeName)

	// Reject malformed names before launching the root theme-list command.
	if !isValidPlymouthThemeName(themeName) {
		return fmt.Errorf("invalid plymouth theme name %q", themeName)
	}

	themelistout, err := exec.Command("plymouth-set-default-theme", "--list").CombinedOutput()
	if err != nil {
		return fmt.Errorf("seems cannot find the plymouth-set-default-theme: %v", err)
	}
	themelist := string(themelistout)
	if err := checkPlymouthThemeName(themelist, themeName); err != nil {
		return err
	}

	out, err := exec.Command("plymouth-set-default-theme", themeName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set plymouth theme: %s, err: %v", string(out), err)
	}

	out, err = exec.Command("update-initramfs", "-u", "-k", "all").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update initramfs: %s, err: %v", string(out), err)
	}

	return nil
}

func getEditionName() (string, error) {
	conf, err := parseInfoFile("/etc/os-version", "=")
	if err != nil {
		return "", err
	}
	value, ok := conf["EditionName"]
	if !ok {
		return "", fmt.Errorf("Can not find the EditionName")
	}
	return value, nil
}

func parseInfoFile(file, delim string) (map[string]string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var ret = make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		array := strings.Split(line, delim)
		if len(array) != 2 {
			continue
		}
		ret[strings.TrimSpace(array[0])] = strings.TrimSpace(array[1])
	}
	return ret, nil
}
