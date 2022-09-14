// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

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
