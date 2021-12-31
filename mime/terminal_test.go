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
package mime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getPresetTerminalPath(t *testing.T) {
	type env struct {
		name  string
		value string
	}
	type test struct {
		name string
		env  []env
		want string
	}

	tests := []test{
		func() test {
			p, _ := filepath.Abs("./testdata/getTerminalPath/notfound")

			return test{
				name: "getTerminalPath",
				env: []env{
					{
						name:  "PATH",
						value: p,
					},
				},
				want: "",
			}
		}(),
		func() test {
			p, _ := filepath.Abs("./testdata/getTerminalPath/path1")

			return test{
				name: "getTerminalPath",
				env: []env{
					{
						name:  "PATH",
						value: p,
					},
				},
				want: filepath.Join(p, "terminator"),
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, e := range tt.env {
				os.Setenv(e.name, e.value)
			}

			got := GetPresetTerminalPath()
			assert.Equal(t, tt.want, got)

			for _, e := range tt.env {
				os.Unsetenv(e.name)
			}
		})
	}
}
