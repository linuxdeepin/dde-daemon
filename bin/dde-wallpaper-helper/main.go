// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// maxSize matches the limit enforced by dde-system-daemon's SaveCustomWallPaper.
const maxSize = 32 * 1024 * 1024

func main() {
	if len(os.Args) != 2 {
		_, _ = fmt.Fprintln(os.Stderr, "usage: dde-wallpaper-helper <file>")
		os.Exit(1)
	}

	file := os.Args[1]

	// Open with O_NOFOLLOW to atomically reject symlinks — this is the
	// core TOCTOU defence. All subsequent operations use the returned fd.
	fd, err := unix.Open(file, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	f := os.NewFile(uintptr(fd), file)
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !info.Mode().IsRegular() {
		_, _ = fmt.Fprintf(os.Stderr, "file %s is not a regular file\n", file)
		os.Exit(1)
	}
	if info.Size() > maxSize {
		_, _ = fmt.Fprintf(os.Stderr, "file size %d exceeds limit %d\n", info.Size(), maxSize)
		os.Exit(1)
	}

	_, err = io.Copy(os.Stdout, f)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
