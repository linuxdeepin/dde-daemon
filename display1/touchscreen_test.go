// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"testing"

	"github.com/linuxdeepin/go-x11-client/ext/randr"
	"github.com/stretchr/testify/assert"
)

func Test_genTransformationMatrixAux(t *testing.T) {
	type args struct {
		offsetX            int16
		offsetY            int16
		screenWidth        uint16
		screenHeight       uint16
		totalDisplayWidth  uint16
		totalDisplayHeight uint16
		rotation           uint16
	}
	tests := []struct {
		name string
		args args
		want TransformationMatrix
	}{
		{
			name: "genTransformationMatrixAux single screen",
			args: args{
				offsetX:            0,
				offsetY:            0,
				screenWidth:        1920,
				screenHeight:       1080,
				totalDisplayWidth:  1920,
				totalDisplayHeight: 1080,
				rotation:           randr.RotationRotate0,
			},
			want: TransformationMatrix{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			},
		},
		{
			name: "genTransformationMatrixAux single screen rotate 90",
			args: args{
				offsetX:            0,
				offsetY:            0,
				screenWidth:        1920,
				screenHeight:       1080,
				totalDisplayWidth:  1920,
				totalDisplayHeight: 1080,
				rotation:           randr.RotationRotate90,
			},
			want: TransformationMatrix{
				0, -1, 1,
				1, 0, 0,
				0, 0, 1,
			},
		},
		{
			name: "genTransformationMatrixAux single screen rotate 90 reflect X",
			args: args{
				offsetX:            0,
				offsetY:            0,
				screenWidth:        1920,
				screenHeight:       1080,
				totalDisplayWidth:  1920,
				totalDisplayHeight: 1080,
				rotation:           randr.RotationRotate90 | randr.RotationReflectX,
			},
			want: TransformationMatrix{
				0, 1, 0,
				1, 0, 0,
				0, 0, 1,
			},
		},
		{
			name: "genTransformationMatrixAux single screen rotate 90 reflect All",
			args: args{
				offsetX:            0,
				offsetY:            0,
				screenWidth:        1920,
				screenHeight:       1080,
				totalDisplayWidth:  1920,
				totalDisplayHeight: 1080,
				rotation:           randr.RotationRotate90 | randr.RotationReflectX | randr.RotationReflectY,
			},
			want: TransformationMatrix{
				0, 1, 0,
				-1, 0, 1,
				0, 0, 1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := genTransformationMatrixAux(tt.args.offsetX, tt.args.offsetY,
				tt.args.screenWidth, tt.args.screenHeight,
				tt.args.totalDisplayWidth, tt.args.totalDisplayHeight,
				tt.args.rotation)
			assert.Equal(t, tt.want, got)
		})
	}
}
