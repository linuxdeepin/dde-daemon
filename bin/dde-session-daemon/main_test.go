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

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_allowRun(t *testing.T) {
	type env struct {
		name  string
		value string
	}
	tests := []struct {
		name string
		env  []env
		want bool
	}{
		{
			name: "allowRun",
			env: []env{
				{
					name:  "DDE_SESSION_PROCESS_COOKIE_ID",
					value: "1",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, e := range tt.env {
				os.Setenv(e.name, e.value)
			}

			got := allowRun()
			assert.Equal(t, tt.want, got)

			for _, e := range tt.env {
				os.Unsetenv(e.name)
			}
		})
	}
}
