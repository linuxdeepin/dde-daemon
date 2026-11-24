// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const maxOwnershipDepth = 128

// homeOwnershipSnapshot holds the account properties used throughout one recovery.
type homeOwnershipSnapshot struct {
	userName string
	uid      string
	gid      string
	path     string
}

// Automatic recovery is limited to the account's direct /home child.
func validateAutomaticRecoveryHome(account homeOwnershipSnapshot) error {
	name := account.userName
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '/') || strings.IndexByte(name, 0) >= 0 {
		return errors.New("automatic recovery requires a single-component account name")
	}
	expected := filepath.Join("/home", name)
	if strings.TrimRight(account.path, "/") != expected {
		return fmt.Errorf("automatic recovery for %q is limited to %q", name, expected)
	}
	return nil
}

func recoveryIDs(account homeOwnershipSnapshot) (int, int, error) {
	parse := func(value, kind string) (int, error) {
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil || id == uint64(^uint32(0)) {
			return 0, fmt.Errorf("invalid recovery %s %q", kind, value)
		}
		return int(id), nil
	}
	uid, err := parse(account.uid, "uid")
	if err != nil {
		return 0, 0, err
	}
	gid, err := parse(account.gid, "gid")
	return uid, gid, err
}

func openRecoveryHome(home string) (*os.File, error) {
	if !filepath.IsAbs(home) || filepath.Clean(home) == "/" {
		return nil, fmt.Errorf("invalid recovery home %q", home)
	}
	fd, err := unix.Open("/", unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Split(strings.TrimPrefix(filepath.Clean(home), "/"), "/") {
		next, openErr := unix.Openat(fd, name, unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if openErr != nil {
			return nil, fmt.Errorf("open recovery home %q: %w", home, openErr)
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), home), nil
}

// mnt_id is available on the supported 4.19 kernel. Device numbers alone miss
// same-filesystem bind mounts. Failure to identify a mount is not a reason to
// silently allow traversing it.
func recoveryMountID(fd int) (uint64, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("/proc/self/fdinfo/%d", fd))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "mnt_id:" {
			id, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil && id != 0 {
				return id, nil
			}
			return 0, fmt.Errorf("invalid mount identity for fd %d", fd)
		}
	}
	return 0, fmt.Errorf("mount identity unavailable for fd %d", fd)
}

type ownershipRecovery struct {
	uid, gid int
	mount    uint64
	chown    func(int, int, int) error
}

func chownRecoveryFD(fd, uid, gid int) error {
	for {
		err := unix.Fchownat(fd, "", uid, gid, unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW)
		if err != unix.EINTR {
			return err
		}
	}
}

func (r *ownershipRecovery) visit(fd int, depth int) error {
	if depth > maxOwnershipDepth {
		return errors.New("ownership recovery depth limit exceeded")
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return err
	}
	mount, err := recoveryMountID(fd)
	if err != nil {
		return err
	}
	if mount != r.mount {
		return errors.New("refusing ownership recovery across a mount boundary")
	}
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		if err := r.visitChildren(fd, depth); err != nil {
			return err
		}
	case unix.S_IFREG, unix.S_IFLNK:
		// An O_PATH|O_NOFOLLOW descriptor pins the link itself. Never recurse
		// into its target or reopen any object by its original pathname.
	default:
		return nil
	}
	// Refresh metadata after processing children and before changing ownership.
	if err := unix.Fstat(fd, &st); err != nil {
		return err
	}
	if st.Uid == uint32(r.uid) && st.Gid == uint32(r.gid) {
		return nil
	}
	if st.Mode&unix.S_IFMT == unix.S_IFREG && st.Nlink > 1 {
		return errors.New("refusing to change ownership of a multiply-linked regular file")
	}
	return r.chown(fd, r.uid, r.gid)
}

func (r *ownershipRecovery) visitChildren(fd, depth int) error {
	// '.' resolves inside the pinned directory, even if its original name has
	// been moved or replaced. O_PATH itself cannot be used for enumeration.
	dirFD, err := unix.Openat(fd, ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	dir := os.NewFile(uintptr(dirFD), "ownership-recovery")
	defer dir.Close()
	for {
		names, readErr := dir.Readdirnames(128)
		for _, name := range names {
			child, err := unix.Openat(dirFD, name, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			if err == unix.ENOENT {
				continue
			}
			if err != nil {
				return fmt.Errorf("open recovery entry %q: %w", name, err)
			}
			err = r.visit(child, depth+1)
			unix.Close(child)
			if err != nil {
				return fmt.Errorf("recover entry %q: %w", name, err)
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func recoverOwnershipAt(home *os.File, uid, gid int) error {
	mount, err := recoveryMountID(int(home.Fd()))
	if err != nil {
		return err
	}
	r := ownershipRecovery{uid: uid, gid: gid, mount: mount, chown: chownRecoveryFD}
	return r.visit(int(home.Fd()), 0)
}

// prepareRecoveryHome skips optional recovery for an already-owned home. Its
// caller decides whether an error is fatal to an explicit takeover or merely
// diagnostic for ordinary publication.
func prepareRecoveryHome(account homeOwnershipSnapshot, force bool) error {
	if err := validateAutomaticRecoveryHome(account); err != nil {
		if force {
			return err
		}
		logger.Debugf("skip automatic home recovery for %s: %v", account.userName, err)
		return nil
	}
	uid, gid, err := recoveryIDs(account)
	if err != nil {
		return err
	}
	home, err := openRecoveryHome(account.path)
	if err != nil {
		return err
	}
	defer home.Close()
	var st unix.Stat_t
	if err := unix.Fstat(int(home.Fd()), &st); err != nil {
		return err
	}
	if !force && st.Uid == uint32(uid) && st.Gid == uint32(gid) {
		return nil
	}
	return recoverOwnershipAt(home, uid, gid)
}
