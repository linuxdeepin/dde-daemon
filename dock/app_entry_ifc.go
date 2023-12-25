// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dock

import (
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/procfs"
	"github.com/linuxdeepin/go-x11-client/util/wm/ewmh"
)

func (e *AppEntry) GetInterfaceName() string {
	return entryDBusInterface
}

func (entry *AppEntry) Activate(timestamp uint32) *dbus.Error {
	logger.Debug("Activate timestamp:", timestamp)
	var err error

	m := entry.manager
	if HideModeType(m.HideMode.Get()) == HideModeSmartHide {
		m.setPropHideState(HideStateShow)
		m.updateHideState(true)
	}

	entry.PropsMu.RLock()
	hasWindow := entry.hasWindow()
	entry.PropsMu.RUnlock()

	if !hasWindow {
		entry.launchApp(timestamp)
		return nil
	}

	if entry.current == nil {
		err = errors.New("entry.current is nil")
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	winInfo := entry.current
	if winInfo == nil {
		err = errors.New("winInfo is nil")
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	if m.isWaylandSession {
		if m.isActiveWindow(winInfo) {
			showing, _ := m.waylandWM.IsShowingDesktop(0)
			if winInfo.isMinimized() || showing {
				err = winInfo.activate()
			} else {
				if len(entry.windows) == 1 {
					err = winInfo.minimize()
				} else {
					nextWinInfo := entry.findNextLeader()
					if nextWinInfo != nil {
						err = nextWinInfo.activate()
					} else {
						err = errors.New("nextWinInfo is nil")
					}
				}
			}
		} else {
			err = winInfo.activate()
		}
	} else {
		win := winInfo.getXid()
		state, err0 := ewmh.GetWMState(globalXConn, win).Reply(globalXConn)
		if err0 != nil {
			logger.Warningf("failed to get ewmh WMState for win %d: %v", win, err0)
			return dbusutil.ToError(err0)
		}

		activeWin := entry.manager.getActiveWindow()
		if activeWin == nil {
			err = errors.New("activeWin is nil")
			logger.Warning(err)
			return dbusutil.ToError(err)
		}
		if win == activeWin.getXid() {
			if atomsContains(state, atomNetWmStateHidden) {
				err = activateWindow(win)
			} else {
				if len(entry.windows) == 1 {
					err = minimizeWindow(win)
				} else if entry.manager.getActiveWindow().getXid() == win {
					nextWin := entry.findNextLeader()
					if nextWin == nil {
						err = errors.New("nextWin is nil")
						logger.Warning(err)
						return dbusutil.ToError(err)
					}
					err = nextWin.activate()
				}
			}
		} else {
			err = activateWindow(win)
		}
	}

	if err != nil {
		logger.Warning(err)
	}
	return dbusutil.ToError(err)
}

func (e *AppEntry) HandleMenuItem(timestamp uint32, id string) *dbus.Error {
	logger.Debugf("HandleMenuItem id: %q timestamp: %v", id, timestamp)
	menu := e.Menu.getMenu()
	if menu != nil {
		err := menu.HandleAction(id, timestamp)
		return dbusutil.ToError(err)
	}
	logger.Warning("HandleMenuItem failed: entry.coreMenu is nil")
	return nil
}

func (e *AppEntry) HandleDragDrop(timestamp uint32, files []string) *dbus.Error {
	logger.Debugf("handle drag drop files: %v, timestamp: %v", files, timestamp)

	ai := e.appInfo
	if ai != nil {
		e.manager.launch(ai.GetFileName(), timestamp, files)
	} else {
		logger.Warning("not supported")
	}
	return nil
}

// RequestDock 驻留
func (entry *AppEntry) RequestDock() *dbus.Error {
	docked, err := entry.manager.dockEntry(entry)
	if err != nil {
		return dbusutil.ToError(err)
	}
	if docked {
		entry.manager.saveDockedApps()
	}
	return nil
}

// RequestUndock 取消驻留
func (entry *AppEntry) RequestUndock() *dbus.Error {
	entry.manager.undockEntry(entry)
	return nil
}

func (entry *AppEntry) PresentWindows() *dbus.Error {
	entry.PropsMu.RLock()
	windowIds := entry.getWindowIds()
	entry.PropsMu.RUnlock()
	if len(windowIds) > 0 {
		err := entry.manager.wm.PresentWindows(dbus.FlagNoAutoStart, windowIds)
		if err != nil {
			logger.Warning("PresentWindows error:", err)
		}
	}
	return nil
}

func (entry *AppEntry) showWorkspace() error {
	err := entry.manager.wm.ShowWorkspace(0)
	if err != nil {
		return dbusutil.ToError(err)
	}
	return nil
}

func (entry *AppEntry) NewInstance(timestamp uint32) *dbus.Error {
	entry.launchApp(timestamp)
	return nil
}

func (entry *AppEntry) Check() *dbus.Error {
	entry.PropsMu.RLock()
	winInfoSlice := entry.getWindowInfoSlice()
	entry.PropsMu.RUnlock()

	for _, winInfo := range winInfoSlice {
		entry.manager.attachOrDetachWindow(winInfo)
	}
	return nil
}

func (entry *AppEntry) ForceQuit() *dbus.Error {
	entry.PropsMu.RLock()
	winInfoSlice := entry.getWindowInfoSlice()
	entry.PropsMu.RUnlock()

	pidWinInfosMap := make(map[uint][]WindowInfoImp)
	for _, winInfo := range winInfoSlice {
		pid := winInfo.getPid()
		if pid != 0 && isProcessAlive(pid) {
			pidWinInfosMap[pid] = append(pidWinInfosMap[pid], winInfo)
		} else {
			err := winInfo.killClient()
			if err != nil {
				logger.Warning(err)
			}
		}
	}

	for pid, winInfoSlice := range pidWinInfosMap {
		err := killProcess(pid)
		if err != nil {
			logger.Warning(err)
			if os.IsPermission(err) {
				for _, winInfo := range winInfoSlice {
					err = winInfo.killClient()
					if err != nil {
						logger.Warning(err)
					}
				}
			}
		}
	}
	return nil
}

func killProcess(pid uint) error {
	p := procfs.Process(pid)
	if p.Exist() {
		logger.Debug("kill process", pid)
		osP, err := os.FindProcess(int(pid))
		if err != nil {
			return err
		}
		err = osP.Signal(syscall.SIGTERM)
		if err != nil {
			logger.Warningf("failed to send signal TERM to process %d: %v",
				osP.Pid, err)
			return err
		}
		time.AfterFunc(5*time.Second, func() {
			if p.Exist() {
				err := osP.Kill()
				if err != nil {
					logger.Warningf("failed to send signal KILL to process %d: %v",
						osP.Pid, err)
				}
			}
		})
	}
	return nil
}

func (entry *AppEntry) GetAllowedCloseWindows() (windows []uint32, busErr *dbus.Error) {
	entry.PropsMu.RLock()
	ret := make([]uint32, len(entry.windows))
	winInfos := entry.getAllowedCloseWindows()
	for idx, winInfo := range winInfos {
		ret[idx] = uint32(winInfo.getXid())
	}
	entry.PropsMu.RUnlock()
	return ret, nil
}

func isProcessAlive(pid uint) bool {
	p, err := os.FindProcess(int(pid))
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}
	return true
}
