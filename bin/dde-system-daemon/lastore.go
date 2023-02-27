// SPDX-FileCopyrightText: 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/godbus/dbus"
	login1 "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/procfs"
)

const (
	logindConfDir                = "/etc/systemd/logind.conf.d"
	lastoreLogindConf            = "deepin-lastore.conf"
	systemdLogindConfigSection   = "Login"
	systemdLogindConfigNAutoVTs  = "NAutoVTs"
	systemdLogindConfigReserveVT = "ReserveVT"
)

func (d *Daemon) EnableDistUpgradeMode(sender dbus.Sender, enable bool) *dbus.Error {
	// 鉴权 dde-lock 和 root 可以调用
	var uid uint32
	var err error
	authErr := func(sender dbus.Sender) *dbus.Error {
		uid, err = d.service.GetConnUID(string(sender))
		if err != nil {
			return dbusutil.ToError(err)
		}
		if uid != 0 {
			pid, err := d.service.GetConnPID(string(sender))
			if err != nil {
				return dbusutil.ToError(err)
			}
			p := procfs.Process(pid)
			cmd, err := p.Exe()
			if err != nil {
				return dbusutil.ToError(err)
			}
			if cmd == "/usr/bin/dde-lock" {
				return nil
			} else {
				return dbusutil.ToError(fmt.Errorf("not allow %v call this method", sender))
			}
		} else {
			return nil
		}
	}(sender)
	if authErr != nil {
		logger.Warning(authErr)
		return authErr
	}

	pid, err := d.service.GetConnPID(string(sender))
	if err != nil {
		return dbusutil.ToError(err)
	}
	// 重启logind服务
	restartLogindService := func() error {
		_, err := d.systemd.RestartUnit(0, "systemd-logind.service", "replace")
		if err != nil {
			logger.Warning(err)
			return err
		}
		return nil
	}
	lastoreLogindConfPath := filepath.Join(logindConfDir, lastoreLogindConf)
	var selfSessionPath dbus.ObjectPath
	if uid == 0 && !enable {
		// root 时,只是用来特殊场景恢复系统状态
		err := os.RemoveAll(lastoreLogindConfPath)
		if err != nil {
			logger.Warning(err)
			return dbusutil.ToError(err)
		} else {
			return dbusutil.ToError(restartLogindService())
		}
	} else {
		selfSessionPath, err = d.loginManager.GetSessionByPID(0, pid)
		if err != nil {
			return dbusutil.ToError(err)
		}
		// 退出所有非当前session
		terminateOtherSession := func() *dbus.Error {
			sessionDetails, err := d.loginManager.ListSessions(0)
			if err != nil {
				return dbusutil.ToError(err)
			}
			for _, sessionDetail := range sessionDetails {
				if sessionDetail.Path != selfSessionPath {
					session, err := login1.NewSession(d.service.Conn(), sessionDetail.Path)
					if err != nil {
						return dbusutil.ToError(err)
					}
					err = session.Terminate(0)
					if err != nil {
						return dbusutil.ToError(err)
					}
				}
			}
			return nil
		}
		// 开关所有非当前tty
		enableOtherTTY := func(enable bool) *dbus.Error {
			session, err := login1.NewSession(d.service.Conn(), selfSessionPath)
			if err != nil {
				return dbusutil.ToError(err)
			}
			currentTTY, err := session.VTNr().Get(0)
			if err != nil {
				return dbusutil.ToError(err)
			}
			for i := uint32(1); i <= 6; i++ {
				if i == currentTTY {
					continue
				}
				ttyService := fmt.Sprintf("getty@tty%v.service", i)
				if enable {
					_, err := d.systemd.StartUnit(0, ttyService, "replace")
					if err != nil {
						return dbusutil.ToError(err)
					}
				} else {
					_, err := d.systemd.StopUnit(0, ttyService, "replace")
					if err != nil {
						return dbusutil.ToError(err)
					}
				}
			}
			return nil
		}
		inhibit := func(enable bool) {
			gettext.SetLocale(gettext.LcAll, getLocaleWithSender(d.service, sender))
			// 阻塞关机
			if enable {
				if d.inhibitFd == -1 {
					fd, err := d.loginManager.Inhibit(0, "shutdown:sleep", dbusServiceName,
						gettext.Tr("Updating the system, please shut down or reboot later."), "block")
					logger.Infof("prevent shutdown: fd:%v\n", fd)
					if err != nil {
						logger.Infof("prevent shutdown failed: fd:%v, err:%v\n", fd, err)
					}
					d.inhibitFd = fd
				}
			} else {
				// 解除阻塞
				if d.inhibitFd != -1 {
					err := syscall.Close(int(d.inhibitFd))
					if err != nil {
						logger.Infof("enable shutdown: fd:%d, err:%s\n", d.inhibitFd, err)
					} else {
						logger.Info("enable shutdown")
					}
					d.inhibitFd = -1
				}
			}
		}
		if enable {
			// 增加tty配置,用于禁用tty启动
			lastoreConf := keyfile.NewKeyFile()
			lastoreConf.SetInt(systemdLogindConfigSection, systemdLogindConfigNAutoVTs, 1)
			lastoreConf.SetInt(systemdLogindConfigSection, systemdLogindConfigReserveVT, 0)
			err := lastoreConf.SaveToFile(lastoreLogindConfPath)
			if err != nil {
				logger.Warning(err)
				return dbusutil.ToError(err)
			} else {
				err := terminateOtherSession()
				if err != nil {
					logger.Warning(err)
					return err
				}
				err = enableOtherTTY(false)
				if err != nil {
					logger.Warning(err)
					return err
				}
				inhibit(true)
				return dbusutil.ToError(restartLogindService())
			}
		} else {
			inhibit(false)
			err := os.RemoveAll(lastoreLogindConfPath)
			if err != nil {
				logger.Warning(err)
				return dbusutil.ToError(err)
			} else {
				err := enableOtherTTY(true)
				if err != nil {
					logger.Warning(err)
				}
				return dbusutil.ToError(restartLogindService())
			}
		}
	}
}

func getLocaleWithSender(service *dbusutil.Service, sender dbus.Sender) (result string) {
	result = "en_US.UTF-8"
	pid, err := service.GetConnPID(string(sender))
	if err != nil {
		logger.Warning(err)
		return
	}

	p := procfs.Process(pid)
	environ, err := p.Environ()
	if err != nil {
		return
	} else {
		result = environ.Get("LANG")
	}
	return
}
