// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package securityloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type fakeBusService struct {
	owners    map[string]bool
	uids      map[string]uint32
	uidErrors map[string]error
	groups    map[string][]uint32
	pids      map[string]uint32
	parents   map[uint32]uint32
	busID     string
}

const testPrivilegedGroupID = 996

func (s *fakeBusService) NameHasOwner(name string) (bool, error) {
	return s.owners[name], nil
}

func (s *fakeBusService) GetConnUID(name string) (uint32, error) {
	if err := s.uidErrors[name]; err != nil {
		return 0, err
	}
	return s.uids[name], nil
}

func (s *fakeBusService) GetConnPID(name string) (uint32, error) {
	pid, ok := s.pids[name]
	if !ok {
		return 0, fmt.Errorf("PID for %s is unavailable", name)
	}
	return pid, nil
}

func (s *fakeBusService) GetConnGroups(name string) ([]uint32, error) {
	return s.groups[name], nil
}

func (s *fakeBusService) GetBusID() (string, error) {
	return s.busID, nil
}

func newTestAllowCallerRegistry(service *fakeBusService, stateFile string, privilegedGroupID uint32) *AllowCallerRegistry {
	registry := newAllowCallerRegistry(service, stateFile, privilegedGroupID)
	registry.processParent = func(pid uint32) (uint32, error) {
		parentPID, ok := service.parents[pid]
		if !ok {
			return 0, fmt.Errorf("parent PID for %d is unavailable", pid)
		}
		return parentPID, nil
	}
	return registry
}

func TestAllowCallerRegistryAuthorize(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "allow-callers.json")
	service := &fakeBusService{
		owners:  map[string]bool{":1.1": true, ":1.10": true, ":1.11": true, ":1.12": true},
		uids:    map[string]uint32{":1.1": 1000, ":1.10": 1000, ":1.11": 1000, ":1.12": 0},
		groups:  map[string][]uint32{":1.1": {testPrivilegedGroupID}},
		pids:    map[string]uint32{":1.1": 101, ":1.10": 110},
		parents: map[uint32]uint32{110: 101},
		busID:   "bus-a",
	}
	registry := newTestAllowCallerRegistry(service, stateFile, testPrivilegedGroupID)

	if err := registry.AddCaller(DaemonScope, dbus.Sender(":1.1"), ":1.10"); err != nil {
		t.Fatal(err)
	}
	result, err := registry.Authorize(DaemonScope, dbus.Sender(":1.10"))
	if result != AuthOK || err != nil {
		t.Fatalf("registered caller authorization = (%v, %v), want (AuthOK, nil)", result, err)
	}
	result, err = registry.Authorize(DaemonScope, dbus.Sender(":1.11"))
	if result != AuthPolkit || err != nil {
		t.Fatalf("unregistered caller authorization = (%v, %v), want (AuthPolkit, nil)", result, err)
	}
	result, err = registry.Authorize(PowerScope, dbus.Sender(":1.10"))
	if result != AuthPolkit || err != nil {
		t.Fatalf("caller in unregistered scope authorization = (%v, %v), want (AuthPolkit, nil)", result, err)
	}
	result, err = registry.Authorize(PowerScope, dbus.Sender(":1.12"))
	if result != AuthOK || err != nil {
		t.Fatalf("root caller authorization = (%v, %v), want (AuthOK, nil)", result, err)
	}
}

func TestAllowCallerRegistryAuthorizationErrorsDoNotFallBack(t *testing.T) {
	uidErr := errors.New("UID lookup failed")
	registry := newTestAllowCallerRegistry(&fakeBusService{
		uids:      make(map[string]uint32),
		uidErrors: map[string]error{":1.20": uidErr},
	}, filepath.Join(t.TempDir(), "allow-callers.json"), testPrivilegedGroupID)

	tests := []struct {
		name     string
		registry *AllowCallerRegistry
		sender   dbus.Sender
	}{
		{name: "nil registry", sender: ":1.20"},
		{name: "empty sender", registry: registry},
		{name: "UID lookup failure", registry: registry, sender: ":1.20"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.registry.Authorize(DaemonScope, tt.sender)
			if result != AuthError || err == nil {
				t.Fatalf("authorization = (%v, %v), want (AuthError, non-nil error)", result, err)
			}
		})
	}
}

func TestAllowCallerRegistryPersistenceAndRemoval(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "allow-callers.json")
	service := &fakeBusService{
		owners: map[string]bool{":1.20": true, ":1.21": false},
		uids:   map[string]uint32{":1.20": 1000},
		pids:   map[string]uint32{":1.20": 120},
		groups: make(map[string][]uint32),
		busID:  "bus-a",
	}
	content, err := json.Marshal(persistedState{
		BusID: "bus-a",
		Callers: map[string]map[string]callerInfo{
			DaemonScope: {":1.20": {UID: 1000, PID: 120}, ":1.21": {}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateFile, content, 0600); err != nil {
		t.Fatal(err)
	}

	registry := newTestAllowCallerRegistry(service, stateFile, testPrivilegedGroupID)
	if err := registry.load(); err != nil {
		t.Fatal(err)
	}
	result, _ := registry.Authorize(DaemonScope, dbus.Sender(":1.20"))
	if result != AuthOK {
		t.Fatalf("live persisted caller was denied")
	}
	registry.RemoveCaller(":1.20")
	result, _ = registry.Authorize(DaemonScope, dbus.Sender(":1.20"))
	if result == AuthOK {
		t.Fatal("removed caller was allowed")
	}
}

func TestWriteStateUsesPrivatePermissions(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "allow-callers.json")
	registry := &AllowCallerRegistry{stateFile: stateFile, busID: "bus-a"}
	if err := registry.writeState(persistedState{BusID: "bus-a"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("state file mode = %o, want 600", got)
	}
	if leftovers, err := filepath.Glob(filepath.Join(dir, ".allow-callers-*")); err != nil {
		t.Fatal(err)
	} else if len(leftovers) != 0 {
		t.Fatalf("temporary state files remain: %v", leftovers)
	}
}

func TestAllowCallerRegistryRejectsStateFromAnotherBus(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "allow-callers.json")
	content, err := json.Marshal(persistedState{
		BusID: "old-bus",
		Callers: map[string]map[string]callerInfo{
			DaemonScope: {":1.30": {UID: 1000, PID: 130}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateFile, content, 0600); err != nil {
		t.Fatal(err)
	}

	service := &fakeBusService{
		owners: map[string]bool{":1.30": true},
		uids:   map[string]uint32{":1.30": 1000},
		groups: make(map[string][]uint32),
		busID:  "new-bus",
	}
	registry := newTestAllowCallerRegistry(service, stateFile, testPrivilegedGroupID)
	if err := registry.load(); err != nil {
		t.Fatal(err)
	}
	result, _ := registry.Authorize(DaemonScope, dbus.Sender(":1.30"))
	if result == AuthOK {
		t.Fatal("caller restored from a different system-bus lifetime")
	}
}

func TestAllowCallerRegistryConcurrentAddsPersistAllCallers(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "allow-callers.json")
	service := &fakeBusService{
		owners:  map[string]bool{":1.1": true},
		uids:    map[string]uint32{":1.1": 1000},
		groups:  map[string][]uint32{":1.1": {testPrivilegedGroupID}},
		pids:    map[string]uint32{":1.1": 101},
		parents: make(map[uint32]uint32),
		busID:   "bus-a",
	}
	const callerCount = 20
	for i := 0; i < callerCount; i++ {
		name := ":1." + strconv.Itoa(100+i)
		service.owners[name] = true
		service.uids[name] = 1000
		service.pids[name] = uint32(200 + i)
		service.parents[uint32(200+i)] = 101
	}

	registry := newTestAllowCallerRegistry(service, stateFile, testPrivilegedGroupID)
	errCh := make(chan error, callerCount)
	var wg sync.WaitGroup
	for i := 0; i < callerCount; i++ {
		name := ":1." + strconv.Itoa(100+i)
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			errCh <- registry.AddCaller(DaemonScope, dbus.Sender(":1.1"), name)
		}(name)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	content, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	var state persistedState
	if err := json.Unmarshal(content, &state); err != nil {
		t.Fatal(err)
	}
	if got := len(state.Callers[DaemonScope]); got != callerCount {
		t.Fatalf("persisted caller count = %d, want %d", got, callerCount)
	}
}

func TestAllowCallerRegistryValidatesRegistrar(t *testing.T) {
	tests := []struct {
		name       string
		sender     dbus.Sender
		target     string
		uids       map[string]uint32
		groups     map[string][]uint32
		descendant bool
		wantError  bool
	}{
		{
			name:       "privileged same uid",
			sender:     dbus.Sender(":1.1"),
			target:     ":1.10",
			uids:       map[string]uint32{":1.1": 1000, ":1.10": 1000},
			groups:     map[string][]uint32{":1.1": {testPrivilegedGroupID}},
			descendant: true,
		},
		{
			name:      "same uid unrelated process",
			sender:    dbus.Sender(":1.5"),
			target:    ":1.50",
			uids:      map[string]uint32{":1.5": 1000, ":1.50": 1000},
			groups:    map[string][]uint32{":1.5": {testPrivilegedGroupID}},
			wantError: true,
		},
		{
			name:      "missing privileged group",
			sender:    dbus.Sender(":1.2"),
			target:    ":1.20",
			uids:      map[string]uint32{":1.2": 1000, ":1.20": 1000},
			groups:    map[string][]uint32{":1.2": {1000}},
			wantError: true,
		},
		{
			name:      "cross uid target",
			sender:    dbus.Sender(":1.3"),
			target:    ":1.30",
			uids:      map[string]uint32{":1.3": 1000, ":1.30": 1001},
			groups:    map[string][]uint32{":1.3": {testPrivilegedGroupID}},
			wantError: true,
		},
		{
			name:   "root cross uid target",
			sender: dbus.Sender(":1.4"),
			target: ":1.40",
			uids:   map[string]uint32{":1.4": 0, ":1.40": 1001},
			groups: make(map[string][]uint32),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeBusService{
				owners:  map[string]bool{tt.target: true},
				uids:    tt.uids,
				groups:  tt.groups,
				pids:    map[string]uint32{string(tt.sender): 101, tt.target: 110},
				parents: map[uint32]uint32{110: 1, 1: 0},
				busID:   "bus-a",
			}
			if tt.descendant {
				service.parents[110] = 101
			}
			registry := newTestAllowCallerRegistry(service, filepath.Join(t.TempDir(), "allow-callers.json"), testPrivilegedGroupID)
			err := registry.AddCaller(DaemonScope, tt.sender, tt.target)
			if tt.wantError && err == nil {
				t.Fatal("expected registration to be denied")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("registration was denied: %v", err)
			}
		})
	}
}

func TestAddCallerIsNotVisibleBeforePersistenceSucceeds(t *testing.T) {
	service := &fakeBusService{
		owners:  map[string]bool{":1.1": true, ":1.50": true},
		uids:    map[string]uint32{":1.1": 1000, ":1.50": 1000},
		groups:  map[string][]uint32{":1.1": {testPrivilegedGroupID}},
		pids:    map[string]uint32{":1.1": 101, ":1.50": 150},
		parents: map[uint32]uint32{150: 101},
		busID:   "bus-a",
	}
	registry := newTestAllowCallerRegistry(service, filepath.Join(t.TempDir(), "allow-callers.json"), testPrivilegedGroupID)
	persistStarted := make(chan struct{})
	releasePersist := make(chan struct{})
	registry.persistState = func(persistedState) error {
		close(persistStarted)
		<-releasePersist
		return errors.New("persist failed")
	}

	addResult := make(chan error, 1)
	go func() {
		addResult <- registry.AddCaller(DaemonScope, dbus.Sender(":1.1"), ":1.50")
	}()
	<-persistStarted

	authorizeResult := make(chan AuthResult, 1)
	go func() {
		result, _ := registry.Authorize(DaemonScope, dbus.Sender(":1.50"))
		authorizeResult <- result
	}()
	select {
	case <-authorizeResult:
		t.Fatalf("authorization completed before registration commit")
	case <-time.After(50 * time.Millisecond):
	}

	close(releasePersist)
	if err := <-addResult; err == nil {
		t.Fatal("expected persistence failure")
	}
	if result := <-authorizeResult; result == AuthOK {
		t.Fatal("failed registration became authorized")
	}
}

func TestAllowCallerRegistryTOCTOUProtection(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fakeBusService)
	}{
		{
			name: "different UID",
			mutate: func(service *fakeBusService) {
				service.uids[":1.10"] = 1001
			},
		},
		{
			name: "same UID different process",
			mutate: func(service *fakeBusService) {
				service.pids[":1.10"] = 111
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeBusService{
				owners:  map[string]bool{":1.1": true, ":1.10": true},
				uids:    map[string]uint32{":1.1": 1000, ":1.10": 1000},
				groups:  map[string][]uint32{":1.1": {testPrivilegedGroupID}},
				pids:    map[string]uint32{":1.1": 101, ":1.10": 110},
				parents: map[uint32]uint32{110: 101},
				busID:   "bus-a",
			}
			registry := newTestAllowCallerRegistry(
				service,
				filepath.Join(t.TempDir(), "allow-callers.json"),
				testPrivilegedGroupID,
			)

			if err := registry.AddCaller(DaemonScope, dbus.Sender(":1.1"), ":1.10"); err != nil {
				t.Fatal(err)
			}
			result, err := registry.Authorize(DaemonScope, dbus.Sender(":1.10"))
			if result != AuthOK || err != nil {
				t.Fatalf("authorization before recycling = (%v, %v), want (AuthOK, nil)", result, err)
			}

			tt.mutate(service)
			result, err = registry.Authorize(DaemonScope, dbus.Sender(":1.10"))
			if result != AuthPolkit || err != nil {
				t.Fatalf("authorization after recycling = (%v, %v), want (AuthPolkit, nil)", result, err)
			}
		})
	}
}

func TestRemoveCallerIfMatchesPreservesReregistration(t *testing.T) {
	service := &fakeBusService{
		owners:  map[string]bool{":1.1": true, ":1.10": true},
		uids:    map[string]uint32{":1.1": 1000, ":1.10": 1000},
		groups:  map[string][]uint32{":1.1": {testPrivilegedGroupID}},
		pids:    map[string]uint32{":1.1": 101, ":1.10": 110},
		parents: map[uint32]uint32{110: 101},
		busID:   "bus-a",
	}
	registry := newTestAllowCallerRegistry(
		service,
		filepath.Join(t.TempDir(), "allow-callers.json"),
		testPrivilegedGroupID,
	)
	if err := registry.AddCaller(DaemonScope, dbus.Sender(":1.1"), ":1.10"); err != nil {
		t.Fatal(err)
	}

	registry.mu.RLock()
	stale := registry.callers[DaemonScope][":1.10"]
	registry.mu.RUnlock()

	service.pids[":1.10"] = 111
	service.parents[111] = 101
	if err := registry.AddCaller(DaemonScope, dbus.Sender(":1.1"), ":1.10"); err != nil {
		t.Fatal(err)
	}
	if registry.removeCallerIfMatches(DaemonScope, ":1.10", stale) {
		t.Fatal("stale cleanup removed a newer registration")
	}

	result, err := registry.Authorize(DaemonScope, dbus.Sender(":1.10"))
	if result != AuthOK || err != nil {
		t.Fatalf("new registration authorization = (%v, %v), want (AuthOK, nil)", result, err)
	}
}

func TestIsProcessDescendant(t *testing.T) {
	parents := map[uint32]uint32{400: 300, 300: 200, 200: 1, 1: 0}
	processParent := func(pid uint32) (uint32, error) {
		parentPID, ok := parents[pid]
		if !ok {
			return 0, fmt.Errorf("parent PID for %d is unavailable", pid)
		}
		return parentPID, nil
	}

	isDescendant, err := isProcessDescendant(400, 200, processParent)
	if err != nil {
		t.Fatal(err)
	}
	if !isDescendant {
		t.Fatal("multi-level descendant was rejected")
	}

	isDescendant, err = isProcessDescendant(400, 999, processParent)
	if err != nil {
		t.Fatal(err)
	}
	if isDescendant {
		t.Fatal("unrelated process was accepted as a descendant")
	}

	cyclicParent := func(pid uint32) (uint32, error) {
		if pid == 10 {
			return 11, nil
		}
		return 10, nil
	}
	if _, err := isProcessDescendant(10, 99, cyclicParent); err == nil {
		t.Fatal("cyclic process ancestry was accepted")
	}
}

func TestGetProcessParentPID(t *testing.T) {
	parentPID, err := getProcessParentPID(uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	if want := uint32(os.Getppid()); parentPID != want {
		t.Fatalf("parent PID = %d, want %d", parentPID, want)
	}
}

func TestGetProcessGroupsIncludesEffectiveGroup(t *testing.T) {
	groups, err := getProcessGroups(uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	if !containsGroup(groups, uint32(os.Getegid())) {
		t.Fatalf("effective gid %d was not found in process credentials %v", os.Getegid(), groups)
	}
}
