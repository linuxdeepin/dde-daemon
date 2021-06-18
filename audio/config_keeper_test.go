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
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigKeeper_Save(t *testing.T) {
	type fields struct {
		file  string
		Cards map[string]*CardConfig
	}
	tests := []struct {
		name        string
		fields      fields
		wantErr     bool
		fileContent string
	}{
		{
			name: "ConfigKeeper_Save",
			fields: fields{
				file: "./testdata/ConfigKeeper_Save",
				Cards: map[string]*CardConfig{
					"one": {
						Name:  "xxx",
						Ports: map[string]*PortConfig{},
					},
				},
			},
			wantErr: false,
			fileContent: `{
  "one": {
    "Name": "xxx",
    "Ports": {}
  }
}`,
		},
		{
			name: "ConfigKeeper_Save empty",
			fields: fields{
				file:  "./testdata/ConfigKeeper_Save",
				Cards: map[string]*CardConfig{},
			},
			wantErr:     false,
			fileContent: "{}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ck := &ConfigKeeper{
				file:  tt.fields.file,
				Cards: tt.fields.Cards,
			}
			err := ck.Save()
			if tt.wantErr {
				assert.NotNil(t, err)
				return
			}

			assert.Nil(t, err)

			s, err := os.Stat(tt.fields.file)
			require.Nil(t, err)
			assert.Equal(t, 0644, int(s.Mode())&0777)

			content, err := ioutil.ReadFile(tt.fields.file)
			require.Nil(t, err)
			assert.Equal(t, tt.fileContent, string(content))

			os.Remove(tt.fields.file)
		})
	}
}
