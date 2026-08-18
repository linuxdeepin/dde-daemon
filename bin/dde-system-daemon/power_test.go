// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeShortIdleController struct {
	state    bool
	getErr   error
	setErr   error
	setCalls int
}

func (c *fakeShortIdleController) ShortIdleState() (bool, error) {
	return c.state, c.getErr
}

func (c *fakeShortIdleController) SetShortIdleState(state bool) error {
	c.setCalls++
	if c.setErr != nil {
		return c.setErr
	}
	c.state = state
	return nil
}

type blockingShortIdleController struct {
	mu              sync.Mutex
	state           bool
	getCalls        int
	firstSet        sync.Once
	firstSetStarted chan struct{}
	releaseFirstSet chan struct{}
	secondGetCalled chan struct{}
}

func (c *blockingShortIdleController) ShortIdleState() (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.getCalls++
	if c.getCalls == 2 {
		close(c.secondGetCalled)
	}
	return c.state, nil
}

func (c *blockingShortIdleController) SetShortIdleState(state bool) error {
	c.mu.Lock()
	c.state = state
	c.mu.Unlock()

	c.firstSet.Do(func() {
		close(c.firstSetStarted)
		<-c.releaseFirstSet
	})
	return nil
}

func (c *blockingShortIdleController) currentState() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

func TestSetStateWithControllerGetErrorDoesNotWrite(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "relax_state")
	if err := os.WriteFile(stateFile, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}

	controller := &fakeShortIdleController{getErr: errors.New("get failed")}
	d := &Daemon{idleStatePath: stateFile}
	if err := d.setStateWithController(controller, stateFile, true); err == nil {
		t.Fatal("expected get state error")
	}

	content, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "0" {
		t.Fatalf("state file changed after getter failure: %q", content)
	}
	if controller.setCalls != 0 {
		t.Fatalf("setter called after getter failure: %d", controller.setCalls)
	}
}

func TestSetStateWithControllerSetErrorDoesNotWrite(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "relax_state")
	if err := os.WriteFile(stateFile, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}

	controller := &fakeShortIdleController{setErr: errors.New("set failed")}
	d := &Daemon{idleStatePath: stateFile}
	if err := d.setStateWithController(controller, stateFile, true); err == nil {
		t.Fatal("expected set state error")
	}

	content, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "0" {
		t.Fatalf("state file changed after setter failure: %q", content)
	}
	if controller.setCalls != 1 {
		t.Fatalf("unexpected setter call count: %d", controller.setCalls)
	}
}

func TestSetScreenStateWritesWithoutController(t *testing.T) {
	screenStateFile := filepath.Join(t.TempDir(), "idle_state")
	if err := os.WriteFile(screenStateFile, []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}

	d := &Daemon{
		idleStatePath:       filepath.Join(t.TempDir(), "relax_state"),
		idleScreenStatePath: screenStateFile,
	}
	if err := d.setState(screenStateFile, false); err != nil {
		t.Fatalf("set screen state: %v", err)
	}

	content, err := os.ReadFile(screenStateFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "0" {
		t.Fatalf("screen state file = %q, want %q", content, "0")
	}
}

func TestSetStateWithControllerRetryRepairsFile(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "relax_state")
	controller := new(fakeShortIdleController)
	d := &Daemon{idleStatePath: stateFile}

	if err := d.setStateWithController(controller, stateFile, true); err == nil {
		t.Fatal("expected first write to fail")
	}
	if !controller.state {
		t.Fatal("controller state was not updated before the write failure")
	}

	if err := os.WriteFile(stateFile, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := d.setStateWithController(controller, stateFile, true); err != nil {
		t.Fatalf("retry state file write: %v", err)
	}

	content, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "1" {
		t.Fatalf("state file = %q, want %q", content, "1")
	}
	if controller.setCalls != 1 {
		t.Fatalf("unexpected setter call count: %d", controller.setCalls)
	}
}

func TestSetStateWithControllerSerializesTransitions(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "relax_state")
	if err := os.WriteFile(stateFile, []byte("0"), 0644); err != nil {
		t.Fatal(err)
	}

	controller := &blockingShortIdleController{
		firstSetStarted: make(chan struct{}),
		releaseFirstSet: make(chan struct{}),
		secondGetCalled: make(chan struct{}),
	}
	d := &Daemon{idleStatePath: stateFile}

	firstErr := make(chan error, 1)
	go func() {
		firstErr <- d.setStateWithController(controller, stateFile, true)
	}()

	select {
	case <-controller.firstSetStarted:
	case <-time.After(time.Second):
		t.Fatal("first transition did not reach the setter")
	}

	secondErr := make(chan error, 1)
	secondStarted := make(chan struct{})
	go func() {
		close(secondStarted)
		secondErr <- d.setStateWithController(controller, stateFile, false)
	}()
	<-secondStarted

	serialized := true
	select {
	case <-controller.secondGetCalled:
		serialized = false
	case <-time.After(100 * time.Millisecond):
	}

	close(controller.releaseFirstSet)
	if err := <-firstErr; err != nil {
		t.Fatalf("first transition: %v", err)
	}
	if err := <-secondErr; err != nil {
		t.Fatalf("second transition: %v", err)
	}
	if !serialized {
		t.Fatal("second transition entered before the first transition completed")
	}

	content, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "0" {
		t.Fatalf("state file = %q, want %q", content, "0")
	}
	if controller.currentState() {
		t.Fatal("controller state and state file are inconsistent")
	}
}