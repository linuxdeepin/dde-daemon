// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package power

import (
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
)

func Test_getValidName(t *testing.T) {
	names := []string{"BAT0", "test.t", "test:t", "test-t", "test.1:2-3.4:5-6"}
	for _, name := range names {
		path := dbus.ObjectPath("/battery_" + getValidName(name))
		t.Log(path)
		assert.True(t, path.IsValid())
	}
}
func TestSetShortIdleStateDisabledIsNoOp(t *testing.T) {
	m := new(Manager)

	if err := m.setShortIdleState(true); err != nil {
		t.Fatalf("setShortIdleState returned error while disabled: %v", err)
	}
	if m.getShortIdleState() {
		t.Fatal("short idle state changed while disabled")
	}
}

func TestShortIdleEnableConcurrentAccess(t *testing.T) {
	m := new(Manager)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			m.setShortIdleEnable(i%2 == 0)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			_ = m.getShortIdleEnable()
		}
	}()
	wg.Wait()

	m.setShortIdleEnable(true)
	if !m.getShortIdleEnable() {
		t.Fatal("short idle enable state was not updated")
	}
}
