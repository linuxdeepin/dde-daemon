// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package securityloader

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"os"
	"os/user"
	"path/filepath"
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

	privilegedGroup = "deepin-daemon"
	invalidGroupID  = ^uint32(0)

	defaultRuntimeDir = "/run/dde-daemon"
	defaultStateFile  = defaultRuntimeDir + "/security_loader_allow_callers.json"
)

// AuthResult describes how a caller should be authorized.
type AuthResult int

const (
	// AuthError means the registry could not determine whether the caller is
	// authorized. err describes the internal authorization failure.
	AuthError AuthResult = iota
	// AuthOK means the caller is authorized by the allow-caller registry and
	// does not need the service's original authorization mechanism.
	AuthOK
	// AuthPolkit means the caller is not registered for the requested scope.
	// Callers should fall back to their original Polkit authorization.
	AuthPolkit
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

// callerInfo stores the credentials recorded for an authorized D-Bus
// connection. Checking both fields prevents a recycled name owned by another
// process, including one under the same UID, from inheriting authorization.
type callerInfo struct {
	UID uint32 `json:"uid"`
	PID uint32 `json:"pid"`
}

type persistedState struct {
	BusID   string                           `json:"busId"`
	Callers map[string]map[string]callerInfo `json:"callers"`
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
	callers   map[string]map[string]callerInfo
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
		callers:           make(map[string]map[string]callerInfo),
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
	content, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
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
	content, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
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

	targetUID, err := r.service.GetConnUID(uniqueName)
	if err != nil {
		return fmt.Errorf("get target caller %q UID failed: %w", uniqueName, err)
	}
	targetPID, err := r.service.GetConnPID(uniqueName)
	if err != nil {
		return fmt.Errorf("get target caller %q PID failed: %w", uniqueName, err)
	}
	info := callerInfo{UID: targetUID, PID: targetPID}
	if err := r.authorizeRegistrar(sender, uniqueName, info); err != nil {
		return err
	}

	r.persistMu.Lock()
	defer r.persistMu.Unlock()

	r.mu.Lock()
	callers := r.callers[scope]
	if callers == nil {
		callers = make(map[string]callerInfo)
		r.callers[scope] = callers
	}
	previous, existed := callers[uniqueName]
	if existed && previous == info {
		r.mu.Unlock()
		return nil
	}
	callers[uniqueName] = info
	if err := r.saveLocked(); err != nil {
		if existed {
			callers[uniqueName] = previous
		} else {
			delete(callers, uniqueName)
			if len(callers) == 0 {
				delete(r.callers, scope)
			}
		}
		r.mu.Unlock()
		return err
	}
	r.mu.Unlock()

	logger.Infof("registered security-loader caller %q for %q", uniqueName, scope)
	return nil
}

func (r *AllowCallerRegistry) authorizeRegistrar(sender dbus.Sender, uniqueName string, target callerInfo) error {
	senderUID, err := r.service.GetConnUID(string(sender))
	if err != nil {
		return fmt.Errorf("get SetAllowCaller sender %q UID failed: %w", sender, err)
	}
	// Root is trusted to register a process running under another account.
	if senderUID == 0 {
		return nil
	}
	if r.privilegedGroupID == invalidGroupID {
		return fmt.Errorf("privileged group %q is unavailable", privilegedGroup)
	}

	groups, err := r.service.GetConnGroups(string(sender))
	if err != nil {
		return fmt.Errorf("get SetAllowCaller sender %q groups failed: %w", sender, err)
	}
	if !containsGroup(groups, r.privilegedGroupID) {
		return fmt.Errorf("D-Bus caller %q is not in privileged group %q", sender, privilegedGroup)
	}
	if target.UID != senderUID {
		return fmt.Errorf("SetAllowCaller sender UID %d does not own target %q with UID %d", senderUID, uniqueName, target.UID)
	}

	senderPID, err := r.service.GetConnPID(string(sender))
	if err != nil {
		return fmt.Errorf("get SetAllowCaller sender %q PID failed: %w", sender, err)
	}
	if r.processParent == nil {
		return errors.New("process ancestry resolver is unavailable")
	}
	isDescendant, err := isProcessDescendant(target.PID, senderPID, r.processParent)
	if err != nil {
		return fmt.Errorf("verify target caller %q process ancestry failed: %w", uniqueName, err)
	}
	if !isDescendant {
		return fmt.Errorf(
			"target caller %q PID %d is not a descendant of SetAllowCaller sender %q PID %d",
			uniqueName, target.PID, sender, senderPID,
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

// Authorize checks whether the caller identified by its D-Bus sender is
// authorized for the given scope. Registered UID and PID are both rechecked
// to prevent a recycled D-Bus name from inheriting authorization.
func (r *AllowCallerRegistry) Authorize(scope string, sender dbus.Sender) (AuthResult, error) {
	if r == nil {
		return AuthError, errors.New("security-loader caller registry is nil")
	}
	if sender == "" {
		return AuthError, errors.New("D-Bus sender is empty")
	}

	uid, err := r.service.GetConnUID(string(sender))
	if err != nil {
		return AuthError, fmt.Errorf("get caller %q UID failed: %w", sender, err)
	}
	if uid == 0 {
		return AuthOK, nil
	}

	r.mu.RLock()
	registered, callerExists := r.callers[scope][string(sender)]
	r.mu.RUnlock()
	if !callerExists {
		return AuthPolkit, nil
	}

	pid, err := r.service.GetConnPID(string(sender))
	if err != nil {
		return AuthError, fmt.Errorf("get caller %q PID failed: %w", sender, err)
	}
	if registered.UID != uid || registered.PID != pid {
		logger.Warningf(
			"security-loader credentials changed for caller %q in scope %q; falling back to Polkit",
			sender, scope,
		)
		r.removeCallerIfMatches(scope, string(sender), registered)
		return AuthPolkit, nil
	}

	return AuthOK, nil
}

// removeCallerIfMatches removes only the registration observed by Authorize.
// A concurrent AddCaller may replace it before cleanup starts; comparing the
// recorded credentials prevents that newer registration from being deleted.
// Locks follow the registry-wide persistMu -> mu order to avoid deadlocks.
func (r *AllowCallerRegistry) removeCallerIfMatches(scope, uniqueName string, expected callerInfo) bool {
	r.persistMu.Lock()
	defer r.persistMu.Unlock()

	r.mu.Lock()
	callers := r.callers[scope]
	current, exists := callers[uniqueName]
	if !exists || current != expected {
		r.mu.Unlock()
		return false
	}
	delete(callers, uniqueName)
	if len(callers) == 0 {
		delete(r.callers, scope)
	}
	saveErr := r.saveLocked()
	r.mu.Unlock()

	if saveErr != nil {
		logger.Warningf("failed to persist removal of security-loader caller %q: %q", uniqueName, saveErr.Error())
	}
	return true
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
		logger.Warningf("failed to persist removal of security-loader caller %q: %q", uniqueName, err.Error())
	}
	r.mu.Unlock()
}

func (r *AllowCallerRegistry) saveLocked() error {
	state := make(map[string]map[string]callerInfo, len(r.callers))
	for scope, callers := range r.callers {
		state[scope] = make(map[string]callerInfo, len(callers))
		for uniqueName, info := range callers {
			state[scope][uniqueName] = info
		}
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

func createPrivateTempFile(dir string) (*os.File, error) {
	const maxAttempts = 10
	var random [16]byte
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if _, err := rand.Read(random[:]); err != nil {
			return nil, fmt.Errorf("generate security-loader state file name failed: %w", err)
		}
		name := filepath.Join(dir, ".allow-callers-"+hex.EncodeToString(random[:]))
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			return file, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
	}
	return nil, errors.New("create unique security-loader state file failed")
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
	tmp, err := createPrivateTempFile(dir)
	if err != nil {
		return fmt.Errorf("create security-loader state file failed: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
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
	content, err := os.ReadFile(r.stateFile)
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
		for uniqueName, registered := range callers {
			if !strings.HasPrefix(uniqueName, ":") {
				continue
			}
			hasOwner, err := r.service.NameHasOwner(uniqueName)
			if err != nil || !hasOwner {
				continue
			}
			uid, err := r.service.GetConnUID(uniqueName)
			if err != nil || uid != registered.UID {
				continue
			}
			pid, err := r.service.GetConnPID(uniqueName)
			if err != nil || pid != registered.PID {
				continue
			}
			if r.callers[scope] == nil {
				r.callers[scope] = make(map[string]callerInfo)
			}
			r.callers[scope][uniqueName] = registered
		}
	}
	return nil
}
