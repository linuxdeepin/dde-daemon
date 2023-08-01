// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/linuxdeepin/dde-daemon/appearance/background"
	"github.com/linuxdeepin/dde-daemon/appearance/subthemes"
	dutils "github.com/linuxdeepin/go-lib/utils"
)

var (
	gtkDirs  []string
	iconDirs []string
	bgDirs   []string
)

var prevTimestamp int64

func (m *Manager) handleThemeChanged() {
	if m.watcher == nil {
		return
	}

	m.watchGtkDirs()
	m.watchIconDirs()
	m.watchBgDirs()

	tmpFilePrefix := filepath.Join(background.CustomWallpapersConfigDir, "tmp-")

	for {
		select {
		case <-m.endWatcher:
			logger.Debug("[Fsnotify] quit watch")
			return
		case err := <-m.watcher.Errors:
			logger.Warning("Receive file watcher error:", err)
			return
		case ev, ok := <-m.watcher.Events:
			if !ok {
				logger.Error("Invalid event:", ev)
				return
			}

			if strings.HasPrefix(ev.Name, tmpFilePrefix) {
				continue
			}
			if (ev.Op == fsnotify.Create || ev.Op == fsnotify.Remove) && hasEventOccurred(ev.Name, iconDirs) && dutils.IsDir(ev.Name) {
				upDir := filepath.Join(ev.Name, "../")
				for _, v := range iconDirs {
					if upDir == v {
						if ev.Op == fsnotify.Create {
							m.watcher.Add(ev.Name)
						} else {
							m.watcher.Remove(ev.Name)
						}
						break
					}
				}
			}

			timestamp := time.Now().UnixNano()
			tmp := timestamp - prevTimestamp
			logger.Debug("[Fsnotify] timestamp:", prevTimestamp, timestamp, tmp, ev)
			prevTimestamp = timestamp
			// Filter time duration < 100ms's event
			if tmp > 100000000 {
				<-time.After(time.Millisecond * 100)
				file := ev.Name
				logger.Debug("[Fsnotify] changed file:", file)
				switch {
				case hasEventOccurred(file, bgDirs):
					logger.Debug("fs event in bgDirs")

					if ev.Op&fsnotify.Chmod != 0 {
						continue
					}

					background.NotifyChanged()
					for iloop := range m.wsLoopMap {
						m.wsLoopMap[iloop].NotifyFsChanged()
					}

				case hasEventOccurred(file, gtkDirs):
					logger.Debug("fs event in gtkDirs")
					// Wait for theme copy finished
					<-time.After(time.Millisecond * 700)
					subthemes.RefreshGtkThemes()
					m.emitSignalRefreshed(TypeGtkTheme)
				case hasEventOccurred(file, iconDirs):
					// Wait for theme copy finished
					logger.Debug("fs event in iconDirs")
					<-time.After(time.Millisecond * 700)
					subthemes.RefreshIconThemes()
					subthemes.RefreshCursorThemes()
					m.emitSignalRefreshed(TypeIconTheme)
					m.emitSignalRefreshed(TypeCursorTheme)
				}
			}
		}
	}
}

func (m *Manager) watchGtkDirs() {
	var home = os.Getenv("HOME")
	gtkDirs = []string{
		path.Join(home, ".local/share/themes"),
		path.Join(home, ".themes"),
		"/usr/local/share/themes",
		"/usr/share/themes",
	}

	m.watchDirs(gtkDirs)
}

func (m *Manager) watchIconDirs() {
	var home = os.Getenv("HOME")
	iconDirs = []string{
		path.Join(home, ".local/share/icons"),
		path.Join(home, ".icons"),
		"/usr/local/share/icons",
		"/usr/share/icons",
	}

	m.watchDirs(iconDirs)
}

func (m *Manager) watchBgDirs() {
	bgDirs = background.ListDirs()
	m.watchDirs(bgDirs)
}

func (m *Manager) watchDirs(dirs []string) {
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			logger.Debugf("Mkdir '%s' failed: %v", dir, err)
		}

		err = m.watcher.Add(dir)
		if err != nil {
			logger.Debugf("Watch dir '%s' failed: %v", dir, err)
		}
	}
}

func hasEventOccurred(ev string, list []string) bool {
	for _, v := range list {
		if strings.Contains(ev, v) {
			return true
		}
	}
	return false
}

func (m *Manager) emitSignalRefreshed(type0 string) {
	err := m.service.Emit(m, "Refreshed", type0)
	if err != nil {
		logger.Warning("emit signal Refreshed failed:", err)
	}
}
