// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package gesture1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDaemon(t *testing.T) {
	daemon := NewDaemon()

	assert.Equal(t, "gesture", daemon.Name())
}

func TestIsTouchRightButtonEvent(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		want      bool
	}{
		{name: "touch right button", direction: "down", want: true},
		{name: "touch right button", direction: "up", want: true},
		{name: "touch right button", direction: "left", want: false},
		{name: "swipe", direction: "up", want: false},
	}

	for _, test := range tests {
		t.Run(test.name+"/"+test.direction, func(t *testing.T) {
			assert.Equal(t, test.want, isTouchRightButtonEvent(test.name, test.direction))
		})
	}
}
