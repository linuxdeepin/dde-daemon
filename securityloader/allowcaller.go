// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package securityloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.dbus"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/log"
)

const (
	DaemonScope       = "org.deepin.dde.Daemon1:/org/deepin/dde/Daemon1"
	PowerScope        = "org.deepin.dde.Power1:/org/deepin/dde/Power1"
	InputDevicesScope = "org.deepin.dde.InputDevices1:/org/deepin/dde/InputDevices1"
	AirplaneModeScope = "org.deepin.dde.AirplaneMode1:/org/deepin/dde/AirplaneMode1"

	defaultRuntimeDir = "/run/dde-daemon"
	defaultStateFile  = defaultRuntimeDir + "/security_loader_allow_callers.json"
	privilegedGroup   = "deepin-daemon"
	invalidGroupID    = ^uint32(0)
)

// AuthResult describes the result of an Authorize call.
type AuthResult int

const (
	// AuthDenied means the caller is explicitly rejected. err is the reason.
	AuthDenied AuthResult = iota
	// AuthOK means the caller is authorized by the allow-caller registry.
	AuthOK
	// AuthNotEnabled means no caller has been registered for this scope.
	// The service was not started via deepin-security-loader.
	// Callers should fall back to their original authorization mechanism.
	AuthNotEnabled
)

var logger = log.NewLogger("daemon/security-loader")

type busService interface {
	NameHasOwner(name string) (bool, error)
	GetConnUID(name string) (uint32, error)
	GetConnPID(name string) (uint32, error)
	GetConnGroups(name string) ([]uint32, error)
	GetBusID() (string, error)
}

type serviceBus struct {
	*dbusutil.Service
}

func (s serviceBus) GetBusID() (string, error) {
	var id string
	err := s.Conn().BusObject().Call("org.freedesktop.DBus.GetId", 0).Store(&id)
	return id, err
}

func (s serviceBus) GetConnGroups(name string) ([]uint32, error) {
	pid, err := s.GetConnPID(name)
	if err != nil {
		return nil, err
	}
	return getProcessGroups(pid)
}

type persistedState struct {
	BusID   string              `json:"busId"`
	Callers map[string][]string `json:"callers"`
}

// AllowCallerRegistry stores the exact system-bus unique names authorized by
// deepin-security-loader for each exported D-Bus object.
type AllowCallerRegistry struct {
	service           busService
	stateFile         string
	busID             string
	privilegedGroupID uint32
	processParent     func(uint32) (uint32, error)

	mu        sync.RWMutex
	callers   map[string]map[string]struct{}
	persistMu sync.Mutex

	signalLoop   *dbusutil.SignalLoop
	persistState func(persistedState) error
}

var (
	defaultRegistryMu sync.RWMutex
	defaultRegistry   *AllowCallerRegistry
)

func NewAllowCallerRegistry(service *dbusutil.Service) *AllowCallerRegistry {
	groupID, err := lookupGroupID(privilegedGroup)
	if err != nil {
		logger.Warningf("failed to resolve privileged group %s: %v", privilegedGroup, err)
		groupID = invalidGroupID
	}
	registry := newAllowCallerRegistry(serviceBus{service}, defaultStateFile, groupID)
	if err := registry.load(); err != nil {
		logger.Warning("failed to load security-loader callers:", err)
	}

	registry.signalLoop = dbusutil.NewSignalLoop(service.Conn(), 10)
	registry.signalLoop.Start()
	dbusDaemon := ofdbus.NewDBus(service.Conn())
	dbusDaemon.InitSignalExt(registry.signalLoop, true)
	_, err = dbusDaemon.ConnectNameOwnerChanged(func(name, oldOwner, newOwner string) {
		if strings.HasPrefix(name, ":") && oldOwner != "" && newOwner == "" {
			registry.RemoveCaller(name)
		}
	})
	if err != nil {
		logger.Warning("failed to watch security-loader callers:", err)
	}

	return registry
}

func newAllowCallerRegistry(service busService, stateFile string, privilegedGroupID uint32) *AllowCallerRegistry {
	busID, err := service.GetBusID()
	if err != nil {
		logger.Warning("failed to get system bus ID:", err)
	}
	return &AllowCallerRegistry{
		service:           service,
		stateFile:         stateFile,
		busID:             busID,
		privilegedGroupID: privilegedGroupID,
		processParent:     getProcessParentPID,
		callers:           make(map[string]map[string]struct{}),
	}
}

func lookupGroupID(name string) (uint32, error) {
	group, err := user.LookupGroup(name)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(group.Gid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid gid %q for group %s: %w", group.Gid, name, err)
	}
	return uint32(value), nil
}

func getProcessGroups(pid uint32) ([]uint32, error) {
	content, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil, err
	}

	var groups []uint32
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "Gid:") && !strings.HasPrefix(line, "Groups:") {
			continue
		}
		for _, value := range strings.Fields(strings.SplitN(line, ":", 2)[1]) {
			gid, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid process gid %q: %w", value, err)
			}
			groups = append(groups, uint32(gid))
		}
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("no group credentials found for pid %d", pid)
	}
	return groups, nil
}

func getProcessParentPID(pid uint32) (uint32, error) {
	content, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}

	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "PPid:") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "PPid:"))
		if len(fields) != 1 {
			return 0, fmt.Errorf("invalid PPid entry for pid %d", pid)
		}
		parentPID, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid parent pid %q for pid %d: %w", fields[0], pid, err)
		}
		return uint32(parentPID), nil
	}
	return 0, fmt.Errorf("no PPid entry found for pid %d", pid)
}

func isProcessDescendant(pid, ancestorPID uint32, processParent func(uint32) (uint32, error)) (bool, error) {
	if pid == 0 || ancestorPID == 0 || pid == ancestorPID {
		return false, nil
	}

	visited := make(map[uint32]struct{})
	currentPID := pid
	for currentPID != 0 {
		if _, exists := visited[currentPID]; exists {
			return false, fmt.Errorf("cycle detected in process ancestry at pid %d", currentPID)
		}
		visited[currentPID] = struct{}{}

		parentPID, err := processParent(currentPID)
		if err != nil {
			return false, err
		}
		if parentPID == ancestorPID {
			return true, nil
		}
		currentPID = parentPID
	}
	return false, nil
}

func SetDefaultRegistry(registry *AllowCallerRegistry) {
	defaultRegistryMu.Lock()
	defaultRegistry = registry
	defaultRegistryMu.Unlock()
}

func DefaultRegistry() *AllowCallerRegistry {
	defaultRegistryMu.RLock()
	registry := defaultRegistry
	defaultRegistryMu.RUnlock()
	return registry
}

func (r *AllowCallerRegistry) Close() {
	if r != nil && r.signalLoop != nil {
		r.signalLoop.Stop()
	}
}

func (r *AllowCallerRegistry) AddCaller(scope string, sender dbus.Sender, uniqueName string) error {
	if r == nil {
		return errors.New("security-loader caller registry is nil")
	}
	if scope == "" {
		return errors.New("scope is empty")
	}
	if sender == "" {
		return errors.New("D-Bus sender is empty")
	}
	if !strings.HasPrefix(uniqueName, ":") {
		return fmt.Errorf("invalid D-Bus unique name %q", uniqueName)
	}

	hasOwner, err := r.service.NameHasOwner(uniqueName)
	if err != nil {
		return fmt.Errorf("check D-Bus owner %q failed: %w", uniqueName, err)
	}
	if !hasOwner {
		return fmt.Errorf("D-Bus caller %q has no owner", uniqueName)
	}
	if err := r.authorizeRegistrar(sender, uniqueName); err != nil {
		return err
	}

	r.persistMu.Lock()
	defer r.persistMu.Unlock()

	r.mu.Lock()
	callers := r.callers[scope]
	if callers == nil {
		callers = make(map[string]struct{})
		r.callers[scope] = callers
	}
	if _, exists := callers[uniqueName]; exists {
		r.mu.Unlock()
		return nil
	}
	callers[uniqueName] = struct{}{}
	if err := r.saveLocked(); err != nil {
		delete(r.callers[scope], uniqueName)
		if len(r.callers[scope]) == 0 {
			delete(r.callers, scope)
		}
		r.mu.Unlock()
		return err
	}
	r.mu.Unlock()

	logger.Infof("registered security-loader caller %s for %s", uniqueName, scope)
	return nil
}

func (r *AllowCallerRegistry) authorizeRegistrar(sender dbus.Sender, uniqueName string) error {
	senderUID, err := r.service.GetConnUID(string(sender))
	if err != nil {
		return fmt.Errorf("get SetAllowCaller sender %s UID failed: %w", sender, err)
	}
	// Root is trusted to register a process running under another account.
	if senderUID == 0 {
		return nil
	}
	if r.privilegedGroupID == invalidGroupID {
		return fmt.Errorf("privileged group %s is unavailable", privilegedGroup)
	}

	groups, err := r.service.GetConnGroups(string(sender))
	if err != nil {
		return fmt.Errorf("get SetAllowCaller sender %s groups failed: %w", sender, err)
	}
	if !containsGroup(groups, r.privilegedGroupID) {
		return fmt.Errorf("D-Bus caller %s is not in privileged group %s", sender, privilegedGroup)
	}

	targetUID, err := r.service.GetConnUID(uniqueName)
	if err != nil {
		return fmt.Errorf("get target caller %s UID failed: %w", uniqueName, err)
	}
	if targetUID != senderUID {
		return fmt.Errorf("SetAllowCaller sender UID %d does not own target %s with UID %d", senderUID, uniqueName, targetUID)
	}

	senderPID, err := r.service.GetConnPID(string(sender))
	if err != nil {
		return fmt.Errorf("get SetAllowCaller sender %s PID failed: %w", sender, err)
	}
	targetPID, err := r.service.GetConnPID(uniqueName)
	if err != nil {
		return fmt.Errorf("get target caller %s PID failed: %w", uniqueName, err)
	}
	if r.processParent == nil {
		return errors.New("process ancestry resolver is unavailable")
	}
	isDescendant, err := isProcessDescendant(targetPID, senderPID, r.processParent)
	if err != nil {
		return fmt.Errorf("verify target caller %s process ancestry failed: %w", uniqueName, err)
	}
	if !isDescendant {
		return fmt.Errorf(
			"target caller %s PID %d is not a descendant of SetAllowCaller sender %s PID %d",
			uniqueName, targetPID, sender, senderPID,
		)
	}
	return nil
}

func containsGroup(groups []uint32, target uint32) bool {
	for _, group := range groups {
		if group == target {
			return true
		}
	}
	return false
}

func (r *AllowCallerRegistry) Authorize(scope string, sender dbus.Sender) (AuthResult, error) {
	if r == nil {
		return AuthDenied, errors.New("security-loader caller registry is nil")
	}
	if sender == "" {
		return AuthDenied, errors.New("D-Bus sender is empty")
	}

	uid, err := r.service.GetConnUID(string(sender))
	if err != nil {
		return AuthDenied, fmt.Errorf("get caller %s UID failed: %w", sender, err)
	}
	if uid == 0 {
		return AuthOK, nil
	}

	r.mu.RLock()
	scopeCallers, scopeExists := r.callers[scope]
	_, callerExists := scopeCallers[string(sender)]
	r.mu.RUnlock()

	if !scopeExists || len(scopeCallers) == 0 {
		return AuthNotEnabled, nil
	}
	if !callerExists {
		return AuthDenied, fmt.Errorf("D-Bus caller %s is not authorized for %s", sender, scope)
	}
	return AuthOK, nil
}

func (r *AllowCallerRegistry) RemoveCaller(uniqueName string) {
	if r == nil || uniqueName == "" {
		return
	}

	r.persistMu.Lock()
	defer r.persistMu.Unlock()

	changed := false
	r.mu.Lock()
	for scope, callers := range r.callers {
		if _, exists := callers[uniqueName]; !exists {
			continue
		}
		delete(callers, uniqueName)
		changed = true
		if len(callers) == 0 {
			delete(r.callers, scope)
		}
	}
	if !changed {
		r.mu.Unlock()
		return
	}
	if err := r.saveLocked(); err != nil {
		logger.Warningf("failed to persist removal of security-loader caller %s: %v", uniqueName, err)
	}
	r.mu.Unlock()
}

func (r *AllowCallerRegistry) saveLocked() error {
	state := make(map[string][]string, len(r.callers))
	for scope, callers := range r.callers {
		uniqueNames := make([]string, 0, len(callers))
		for uniqueName := range callers {
			uniqueNames = append(uniqueNames, uniqueName)
		}
		sort.Strings(uniqueNames)
		state[scope] = uniqueNames
	}

	if r.busID == "" {
		return errors.New("system bus ID is empty")
	}
	persisted := persistedState{BusID: r.busID, Callers: state}
	if r.persistState != nil {
		return r.persistState(persisted)
	}
	return r.writeState(persisted)
}

func (r *AllowCallerRegistry) writeState(state persistedState) error {
	content, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal security-loader callers failed: %w", err)
	}

	dir := filepath.Dir(r.stateFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create security-loader runtime directory failed: %w", err)
	}
	tmp, err := ioutil.TempFile(dir, ".allow-callers-")
	if err != nil {
		return fmt.Errorf("create security-loader state file failed: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write security-loader state failed: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close security-loader state failed: %w", err)
	}
	if err := os.Rename(tmpName, r.stateFile); err != nil {
		return fmt.Errorf("replace security-loader state failed: %w", err)
	}
	return nil
}

func (r *AllowCallerRegistry) load() error {
	content, err := ioutil.ReadFile(r.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state persistedState
	if err := json.Unmarshal(content, &state); err != nil {
		return err
	}
	if state.BusID == "" || state.BusID != r.busID {
		return nil
	}

	for scope, callers := range state.Callers {
		if scope == "" {
			continue
		}
		for _, uniqueName := range callers {
			if !strings.HasPrefix(uniqueName, ":") {
				continue
			}
			hasOwner, err := r.service.NameHasOwner(uniqueName)
			if err != nil || !hasOwner {
				continue
			}
			if r.callers[scope] == nil {
				r.callers[scope] = make(map[string]struct{})
			}
			r.callers[scope][uniqueName] = struct{}{}
		}
	}
	return nil
}