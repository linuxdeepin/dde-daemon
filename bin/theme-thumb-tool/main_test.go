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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_moveThumbFiles(t *testing.T) {
	type args struct {
		files []string
	}
	tests := []struct {
		name string
		args args
		dest string
	}{
		{
			name: "moveThumbFiles",
			args: args{
				files: []string{
					"./testdata/moveThumbFiles/source1/f1",
				},
			},
			dest: "./testdata/moveThumbFiles/dest1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_destDir = tt.dest
			moveThumbFiles(tt.args.files)

			for _, f := range tt.args.files {
				assert.FileExists(t, filepath.Join(tt.dest, filepath.Base(f)))
			}
		})
	}
}
