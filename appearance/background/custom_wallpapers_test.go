// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package background

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_sumFileMd5(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "sumFileMd5",
			args: args{
				filename: "./testdata/Theme1/wallpapers/desktop.jpg",
			},
			want:    "fafa2baf6dba60d3e47b8c7fe4eea9e9",
			wantErr: false,
		},
		{
			name: "sumFileMd5 not found",
			args: args{
				filename: "./testdata/Theme1/wallpapers/desktop.jpxg",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sumFileMd5(tt.args.filename)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
