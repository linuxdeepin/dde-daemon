// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package background

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getSysBgFiles(t *testing.T) {
	type args struct {
		dir []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "getSysBgFiles",
			args: args{
				dir: []string{"./testdata/fakeimages"},
			},
			want: []string{"testdata/fakeimages/fakeimage1.jpg", "testdata/fakeimages/fakeimage2.jpg"},
		},
		{
			name: "getCustomBgFilesInDir empty",
			args: args{
				dir: []string{"./testdata/fakeimages/empty"},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSysBgFiles(tt.args.dir)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

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
