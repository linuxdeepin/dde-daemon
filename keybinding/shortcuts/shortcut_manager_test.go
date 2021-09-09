/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     hubenchang <hubenchang@uniontech.com>
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
package shortcuts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_systemType(t *testing.T) {

	var versions = []string{
		"Community", "Professional", "Home",
		"Server", "Personal", "Desktop",
		"",
	}

	assert.Contains(t, versions, systemType())
}

func Test_arr2set(t *testing.T) {
	testData := []struct {
		input  []string
		output map[string]bool
	}{
		{
			input: []string{"key1", "key2", "key3"},
			output: map[string]bool{
				"key1": true,
				"key2": true,
				"key3": true,
			},
		},
	}

	for _, data := range testData {
		assert.Equal(t, data.output, arr2set(data.input))
	}

}

func Test_strvLower(t *testing.T) {
	testData := []struct {
		in  []string
		out []string
	}{
		{
			in:  []string{"What's", "Your", "Problem"},
			out: []string{"what's", "your", "problem"},
		},
	}

	for _, data := range testData {
		assert.Equal(t, data.out, strvLower(data.in))
	}
}
