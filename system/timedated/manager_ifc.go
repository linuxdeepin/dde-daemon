/*
 * Copyright (C) 2017 ~ 2018 Deepin Technology Co., Ltd.
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

package timedated

import (
	"os"
	"os/exec"
	"time"

	"github.com/godbus/dbus"
	"pkg.deepin.io/dde/daemon/timedate/zoneinfo"
	"pkg.deepin.io/lib/dbusutil"
)

// SetTime set the current time and date,
// pass a value of microseconds since 1 Jan 1970 UTC
func (m *Manager) SetTime(sender dbus.Sender, usec int64, relative bool, message string) *dbus.Error {
	// TODO: check usec validity
	err := m.core.SetTime(0, usec, relative, false)
	return dbusutil.ToError(err)
}

// SetTimezone set the system time zone, the value from /usr/share/zoneinfo/zone.tab
func (m *Manager) SetTimezone(sender dbus.Sender, timezone, message string) *dbus.Error {
	ok, err := zoneinfo.IsZoneValid(timezone)
	if err != nil {
		return dbusutil.ToError(err)
	}
	if !ok {
		logger.Warning("invalid zone:", timezone)
		return dbusutil.ToError(zoneinfo.ErrZoneInvalid)
	}

	err = m.checkAuthorization("SetTimezone", message, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}

	currentTimezone, err := m.core.Timezone().Get(0)
	if err != nil {
		return dbusutil.ToError(err)
	}

	if currentTimezone == timezone {
		return nil
	}
	err = m.core.SetTimezone(0, timezone, false)
	return dbusutil.ToError(err)
}

// SetLocalRTC to control whether the RTC is the local time or UTC.
func (m *Manager) SetLocalRTC(sender dbus.Sender, enabled bool, fixSystem bool, message string) *dbus.Error {
	err := m.checkAuthorization("SetLocalRTC", message, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}

	currentLocalRTCEnabled, err := m.core.LocalRTC().Get(0)
	if err != nil {
		return dbusutil.ToError(err)
	}

	if currentLocalRTCEnabled == enabled {
		return nil
	}
	err = m.core.SetLocalRTC(0, enabled, fixSystem, false)
	return dbusutil.ToError(err)
}

// SetNTP to control whether the system clock is synchronized with the network
func (m *Manager) SetNTP(sender dbus.Sender, enabled bool, message string) *dbus.Error {
	currentNTPEnabled, err := m.core.NTP().Get(0)
	if err != nil {
		return dbusutil.ToError(err)
	}

	if currentNTPEnabled == enabled {
		return nil
	}

	err = m.core.SetNTP(0, enabled, false)
	if err != nil {
		return dbusutil.ToError(err)
	}

	// 关闭NTP时删除clock文件，使systemd以RTC时间为准
	if !enabled {
		err := os.Remove("/var/lib/systemd/timesync/clock")
		if err != nil {
			if !os.IsNotExist(err) {
				logger.Warning("delete [/var/lib/systemd/timesync/clock] err:", err)
			}
		}
	} else {
		go func() {
			ticker := time.NewTicker(time.Second)
			cnt := 0
			for range ticker.C {
				// > "/run/systemd/timesync/synchronized"
				// > A file that is touched on each successful synchronization, to
				// > assist systemd-time-wait-sync and other applications to
				// > detecting synchronization with accurate reference clocks.
				// from man systemd-timesyncd.service
				// 此处通过检查该文件确定 ntp 时间是否已经同步到 local time
				if _, err := os.Stat("/run/systemd/timesync/synchronized"); err == nil {
					logger.Info("write ntp synchronized time to rtc time")
					err = exec.Command("hwclock", "-w").Run()
					if err != nil {
						logger.Warningf("write ntp synchronized time to rtc time fail: %v", err)
					}
					break
				}
				cnt++
				if cnt >= 10 {
					logger.Warning("wait for ntp response ... timeup")
					break
				}
			}
			ticker.Stop()
			return
		}()
	}

	return dbusutil.ToError(err)
}

func (m *Manager) SetNTPServer(sender dbus.Sender, server, message string) *dbus.Error {
	err := m.checkAuthorization("SetNTPServer", message, sender)
	if err != nil {
		return dbusutil.ToError(err)
	}

	err = m.setNTPServer(server)
	if err != nil {
		logger.Warning(err)
	}

	ntp, err := m.core.NTP().Get(0)
	if err != nil {
		logger.Warning(err)
	} else if ntp {
		// ntp enabled
		go func() {
			_, err := m.systemd.RestartUnit(0, timesyncdService, "replace")
			if err != nil {
				logger.Warning("failed to restart systemd timesyncd service:", err)
			}
		}()
	}
	return nil
}
