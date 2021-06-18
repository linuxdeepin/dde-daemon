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
package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pkg.deepin.io/lib/pulse"
)

func Test_getCardName(t *testing.T) {
	type args struct {
		card *pulse.Card
	}
	tests := []struct {
		name     string
		args     args
		wantName string
	}{
		{
			name: "getCardName",
			args: args{
				card: &pulse.Card{
					Name: "abcd",
					PropList: map[string]string{
						"alsa.card_name":     "",
						"device.api":         "fdf",
						"device.description": "",
					},
				},
			},
			wantName: "abcd",
		},
		{
			name: "getCardName with alsa card name",
			args: args{
				card: &pulse.Card{
					Name: "abcd",
					PropList: map[string]string{
						"alsa.card_name":     "cdcd",
						"device.api":         "fdf",
						"device.description": "",
					},
				},
			},
			wantName: "cdcd",
		},
		{
			name: "getCardName bluez",
			args: args{
				card: &pulse.Card{
					Name: "abcd",
					PropList: map[string]string{
						"alsa.card_name":     "",
						"device.api":         "bluez",
						"device.description": "xxxxxx",
					},
				},
			},
			wantName: "xxxxxx",
		},
		{
			name: "getCardName bluez no description",
			args: args{
				card: &pulse.Card{
					Name: "abcd",
					PropList: map[string]string{
						"alsa.card_name":     "vvvvvv",
						"device.api":         "bluez",
						"device.description": "",
					},
				},
			},
			wantName: "vvvvvv",
		},
		{
			name: "getCardName bluez no description and alsa card name",
			args: args{
				card: &pulse.Card{
					Name: "abcd",
					PropList: map[string]string{
						"alsa.card_name":     "",
						"device.api":         "bluez",
						"device.description": "",
					},
				},
			},
			wantName: "abcd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName := getCardName(tt.args.card)
			assert.Equal(t, tt.wantName, gotName)
		})
	}
}
