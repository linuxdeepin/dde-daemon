// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub2

import (
	"fmt"
	"github.com/godbus/dbus/v5"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"

	"github.com/linuxdeepin/dde-daemon/grub_common"
	"github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
)

const (
	grubMkconfigCmd = "grub-mkconfig"
	updateGrubCmd   = "update-grub"
	adjustThemeCmd  = "/usr/lib/deepin-api/adjust-grub-theme"
)

func init() {
	_ = os.Setenv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
}

type modifyManager struct {
	g           *Grub2
	ch          chan modifyTask
	modifyTasks []modifyTask

	running       bool
	stateChangeCb func(running bool)

	mu sync.Mutex
}

func newModifyManager() *modifyManager {
	m := &modifyManager{
		ch: make(chan modifyTask),
	}
	return m
}

func (m *modifyManager) notifyStateChange() {
	if m.stateChangeCb != nil {
		m.stateChangeCb(m.running)
	}
}

func (m *modifyManager) loop() {
	for {
		t, ok := <-m.ch
		if !ok {
			return
		}
		m.mu.Lock()

		if m.running {
			m.modifyTasks = append(m.modifyTasks, t)
		} else {
			m.start(t)
		}

		m.mu.Unlock()
	}
}

func (m *modifyManager) start(tasks ...modifyTask) {
	logger.Infof("modifyManager start")
	defer logger.Infof("modifyManager start return")

	params, _ := grub_common.LoadDDEGrubParams()

	logger.Debug("modifyManager.start len(tasks):", len(tasks))
	var adjustTheme bool
	var adjustThemeLang string
	for _, task := range tasks {
		f := task.paramsModifyFunc
		if f != nil {
			f(params)
		}
		if task.adjustTheme {
			adjustTheme = true
			adjustThemeLang = task.adjustThemeLang
		}
	}
	err := writeGrubParams(params)
	if err != nil {
		logger.Warning("failed to write grub params:", err)
		return
	}

	logStart()
	m.running = true
	m.notifyStateChange()
	go m.update(adjustTheme, adjustThemeLang)
}

func (m *modifyManager) update(adjustTheme bool, adjustThemeLang string) {
	if adjustTheme {
		logJobStart(logJobAdjustTheme)
		err := copyBgSource(defaultThemeDir, defaultThemeTmpDir)
		if err != nil && !os.IsNotExist(err) {
			logger.Warning("failed to copy background source:", err)
		} else {
			var args []string
			if adjustThemeLang != "" {
				args = append(args, "-lang", adjustThemeLang)
			}
			args = append(args, "-theme-output", themesTmpDir)
			cmd := exec.Command(adjustThemeCmd, args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			logger.Debugf("$ %s %s", adjustThemeCmd, strings.Join(args, " "))
			err = cmd.Run()
			if err != nil {
				logger.Warning("failed to adjust theme:", err)
			} else {
				syscall.Sync()
				err = replaceAndBackupDir(themesDir, themesTmpDir)
				if err != nil {
					logger.Warning("failed to replace and backup dir:", err)
				}
			}
		}
		logJobEnd(logJobAdjustTheme, err)
		m.g.theme.emitSignalBackgroundChanged()
	}

	logJobStart(logJobMkConfig)
	err := m.runUpdateGrubWithUnit()
	if err != nil {
		logger.Warning("failed to make config:", err)
	}
	logJobEnd(logJobMkConfig, err)
	m.updateEnd()
}

type execStart struct {
	Path             string   // the binary path to execute
	Args             []string // an array with all arguments to pass to the executed command, starting with argument 0
	UncleanIsFailure bool     // a boolean whether it should be considered a failure if the process exits uncleanly
}

func (m *modifyManager) runUpdateGrubWithUnit() error {
	var command []string
	serviceName := "dde-update-grub.service"
	path, err := exec.LookPath(updateGrubCmd)
	if err != nil {
		path = grubMkconfigCmd
		command = append(command, updateGrubCmd, "-o", grubScriptFile)
	} else {
		command = append(command, path)
	}

	locale := getSystemLocale()
	var language string
	if locale != "" {
		logger.Info("system locale:", locale)
		language = strings.Split(locale, ".")[0]
	} else {
		err = fmt.Errorf("failed to get system locale")
		return err
	}
	conn, err := dbus.SystemBus()
	if err != nil {
		logger.Warning("failed to connect to system bus:", err)
		return err
	}

	systemd := systemd1.NewManager(conn)
	var properties []systemd1.Property
	var aux []systemd1.PropertyCollection
	properties = append(properties, systemd1.Property{"Type", dbus.MakeVariant("oneshot")})
	properties = append(properties, systemd1.Property{"Description", dbus.MakeVariant("Transient Unit Update Grub")})
	properties = append(properties, systemd1.Property{"Environment", dbus.MakeVariant([]string{"LANG=" + locale, "LANGUAGE=" + language})})
	properties = append(properties, systemd1.Property{"ExecStart", dbus.MakeVariant([]execStart{{
		Path:             command[0],
		Args:             command,
		UncleanIsFailure: false,
	},
	})})
	jobPath, err := systemd.StartTransientUnit(0, serviceName, "replace", properties, aux)
	if err != nil {
		logger.Warning("failed to start transient unit:", err)
		return err
	}
	systemd.InitSignalExt(m.g.sysLoop, true)
	logger.Infof("%s started successfully: %v", serviceName, jobPath)
	var wait = make(chan bool)
	systemd.ConnectJobRemoved(func(id uint32, job dbus.ObjectPath, unit string, result string) {
		if job == jobPath {
			wait <- true
		}
	})
	<-wait
	logger.Infof("%s unit removed: %v", serviceName, path)
	return nil
}

func (m *modifyManager) updateEnd() {
	m.mu.Lock()

	logEnd()
	logger.Info("modifyManager update end")

	if len(m.modifyTasks) > 0 {
		m.start(m.modifyTasks...)
		m.modifyTasks = nil
	} else {
		// loop end
		syscall.Sync()
		m.running = false
		m.notifyStateChange()
	}

	m.mu.Unlock()
}

func (m *modifyManager) IsRunning() bool {
	m.mu.Lock()
	running := m.running
	m.mu.Unlock()
	return running
}
