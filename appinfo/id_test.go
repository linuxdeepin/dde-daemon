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
package appinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NormalizeAppIDWithCaseSensitive(t *testing.T) {
	assert.Equal(t, "Ab-Cd-Ef-Gh", NormalizeAppIDWithCaseSensitive("Ab_Cd_Ef_Gh"))
}

func Test_NormalizeAppID(t *testing.T) {
	assert.Equal(t, "ab-cd-ef-gh", NormalizeAppID("Ab_Cd_Ef_Gh"))
}
