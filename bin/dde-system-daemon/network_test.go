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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getConnectionFiles(t *testing.T) {
	type args struct {
		dir string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "getConnectionFiles",
			args: args{
				dir: "testdata/getConnectionFiles",
			},
			want:    []string{"testdata/getConnectionFiles/abc", "testdata/getConnectionFiles/ddd"},
			wantErr: false,
		},
		{
			name: "getConnectionFiles not found",
			args: args{
				dir: "testdata/ccc",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getConnectionFiles(tt.args.dir)
			if tt.wantErr {
				assert.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
