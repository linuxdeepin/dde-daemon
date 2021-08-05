/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
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

package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_loadConfig(t *testing.T) {
	cfg := &Config{}
	err := loadConfig("./testdata/config.json", cfg)
	assert.Nil(t, err)

	err = loadConfig("./testdata/config1.json", cfg)
	assert.NotNil(t, err)
}

func Test_loadConfigSafe(t *testing.T) {
	cfg := loadConfigSafe("./testdata/config.json")
	assert.True(t, cfg.Devices["enp2s0"].Enabled)
	assert.False(t, cfg.VpnEnabled)

	loadConfigSafe("./testdata/config1.json")
}

func Test_saveConfig(t *testing.T) {
	cfg := &Config{}
	err := saveConfig("./testdata/config2.json", cfg)
	assert.Nil(t, err)
}
