// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package apps1

import (
	"github.com/adrg/xdg"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func intSliceContains(slice []int, a int) bool {
	for _, elem := range slice {
		if elem == a {
			return true
		}
	}
	return false
}

func intSliceRemove(slice []int, a int) (ret []int) {
	for _, elem := range slice {
		if elem != a {
			ret = append(ret, elem)
		}
	}
	return
}

func readDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	return names, nil
}

func Walk(root string, walkFn WalkFunc) {
	info, err := os.Lstat(root)
	if err != nil {
		return
	}
	walk(root, ".", info, walkFn)
}

func walk(root, name0 string, info os.FileInfo, walkFn WalkFunc) {
	walkFn(name0, info)
	if !info.IsDir() {
		return
	}
	path := filepath.Join(root, name0)
	names, err := readDirNames(path)
	if err != nil {
		return
	}
	for _, name := range names {
		filename := filepath.Join(path, name)
		fileInfo, err := os.Lstat(filename)
		if err != nil {
			continue
		}
		walk(root, filepath.Join(name0, name), fileInfo, walkFn)
	}
}

type WalkFunc func(name string, info os.FileInfo)

func getDirsAndApps(root string) (dirs, apps []string) {
	Walk(root, func(name string, info os.FileInfo) {
		if info.IsDir() {
			dirs = append(dirs, name)
		} else if filepath.Ext(name) == desktopExt {
			apps = append(apps, strings.TrimSuffix(name, desktopExt))
		}
	})
	return
}

const desktopExt = ".desktop"

func isDesktopFile(path string) bool {
	return filepath.Ext(path) == desktopExt
}

func removeDesktopExt(name string) string {
	return strings.TrimSuffix(name, desktopExt)
}

func getSystemDataDirs() []string {
	return xdg.DataDirs
}

// get user home
func getHomeByUid(uid int) (string, error) {
	user, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return "", err
	}
	return user.HomeDir, nil
}

// copy from go source src/os/path.go
func MkdirAll(path string, uid int, perm os.FileMode) error {
	logger.Debug("MkdirAll", path, uid, perm)
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := os.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent
		err = MkdirAll(path[0:j-1], uid, perm)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = os.Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := os.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}

	err = os.Chown(path, uid, uid)
	return err
}
