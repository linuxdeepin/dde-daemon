/*
 * Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:     quezhiyong <quezhiyong@uniontech.com>
 *
 * Maintainer: quezhiyong <quezhiyong@uniontech.com>
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

package trayicon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDaemon_GetDependencies(t *testing.T) {
	d := Daemon{}
	assert.ElementsMatch(t, []string{}, d.GetDependencies())
}

func TestDaemon_Name(t *testing.T) {
	d := Daemon{}
	assert.Equal(t, moduleName, d.Name())
}

func TestDaemon_Stop(t *testing.T) {
	d := Daemon{}
	assert.Nil(t, d.Stop())
}
