/*
 * Copyright (C) 2019 ~ 2022 Uniontech Software Technology Co.,Ltd
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

package eventlog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_writeLoginLog(t *testing.T) {
	t.Run("Test writeLoginLog", func(t *testing.T) {
		c := new(loginEventCollector)
		assert.NoError(t, c.writeLoginLog(&loginTimeInfo{Tid: SystemBootTid}))
	})
}

func Test_genLastRebootCmd(t *testing.T) {
	t.Run("Test genLastRebootCmd", func(t *testing.T) {
		cmd := genLastRebootCmd()
		assert.Equal(t, "/bin/bash -c last -x reboot --time-format iso | awk 'NR==2 {print}'", cmd.String())
	})
}

func Test_parseLastRebootCmdOutPut(t *testing.T) {
	t.Run("Test parseLastRebootCmdOutPut", func(t *testing.T) {
		_, err := parseLastRebootCmdOutPut([]byte("reboot   system boot  4.19.0-amd64-des 2022-08-15T14:13:35+08:00 - 2022-08-15T17:03:29+08:00  (02:49)"))
		assert.NoError(t, err)
	})
}
