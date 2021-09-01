/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     hubenchang <hubenchang@uniontech.com>
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

package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewBluezAudioManager(t *testing.T) {
	bam := NewBluezAudioManager("testdata/bluez_audio.json")
	assert.NotNil(t, bam)
}

func Test_SetMode(t *testing.T) {
	bam := NewBluezAudioManager("testdata/bluez_audio.json")
	assert.Equal(t, bluezModeA2dp, bluezModeDefault)

	bam.SetMode("test-card", bluezModeA2dp)
	assert.Equal(t, bluezModeA2dp, bam.GetMode("test-card"))
	bam.SetMode("test-card", bluezModeHeadset)
	assert.Equal(t, bluezModeHeadset, bam.GetMode("test-card"))
	bam.SetMode("test-card", bluezModeDefault)
	assert.Equal(t, bluezModeDefault, bam.GetMode("test-card"))
}
