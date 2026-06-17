// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fileutil

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// SafeReadFile 安全读取文件，拒绝符号链接
func SafeReadFile(filename string) ([]byte, error) {
	fi, err := os.Lstat(filename)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("file %q is a symlink, refusing to follow", filename)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("file %q is not a regular file", filename)
	}

	f, err := os.OpenFile(filename, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// SafeWriteFile 安全写入文件，拒绝符号链接
func SafeWriteFile(filename string, content []byte, perm os.FileMode) error {
	fi, err := os.Lstat(filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		// 文件已存在，必须是普通文件
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("file %q is a symlink, refusing to write", filename)
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("file %q is not a regular file", filename)
		}
		f, err := os.OpenFile(filename, unix.O_WRONLY|unix.O_TRUNC|unix.O_NOFOLLOW, perm)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(content)
		return err
	}
	// 文件不存在，安全创建
	f, err := os.OpenFile(filename, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(content)
	return err
}
