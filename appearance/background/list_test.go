/*
 * Copyright (C) 2019 ~ 2021 Uniontech Software Technology Co.,Ltd
 *
 * Author:     zsien <i@zsien.cn>
 *
 * Maintainer: zsien <i@zsien.cn>
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
package background

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getCustomBgFilesInDir(t *testing.T) {
	type args struct {
		dir string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "getCustomBgFilesInDir",
			args: args{
				dir: "./testdata/fakeimages",
			},
			want: []string{"testdata/fakeimages/fakeimage1.jpg", "testdata/fakeimages/fakeimage2.jpg"},
		},
		{
			name: "getCustomBgFilesInDir empty",
			args: args{
				dir: "./testdata/fakeimages/empty",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCustomBgFilesInDir(tt.args.dir)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
