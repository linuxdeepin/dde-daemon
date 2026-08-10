// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package securityloader

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

type Destination struct {
	DBusName      string `json:"DbusName"`
	DBusPath      string `json:"DbusPath"`
	DBusInterface string `json:"DbusInterface"`
}

type loaderInfo struct {
	fd1    int
	fd2    int
	loaded bool
}

type handshakeRequest struct {
	UniqueName string        `json:"UniqueName"`
	DestList   []Destination `json:"DestList"`
}

const maxSecurityLoaderResponseSize = int64(5 << 20)

// Handshake consumes deepin-security-loader's injected arguments and registers
// the process system-bus unique name for the requested destinations.
func Handshake(args []string, destinations []Destination) ([]string, bool, error) {
	info, cleanedArgs, err := parseLoaderArgs(args)
	if err != nil || !info.loaded {
		return cleanedArgs, info.loaded, err
	}
	if len(destinations) == 0 {
		return cleanedArgs, true, fmt.Errorf("security-loader destination list is empty")
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return cleanedArgs, true, fmt.Errorf("connect to system bus failed: %w", err)
	}
	uniqueName := ""
	for _, name := range conn.Names() {
		if strings.HasPrefix(name, ":") {
			uniqueName = name
			break
		}
	}
	if uniqueName == "" {
		return cleanedArgs, true, fmt.Errorf("system bus unique name is unavailable")
	}

	request := handshakeRequest{UniqueName: uniqueName, DestList: destinations}
	requestData, err := json.Marshal(request)
	if err != nil {
		return cleanedArgs, true, fmt.Errorf("marshal security-loader request failed: %w", err)
	}

	requestFile := os.NewFile(uintptr(info.fd1), "security-loader-request")
	if requestFile == nil {
		return cleanedArgs, true, fmt.Errorf("open security-loader request fd failed")
	}
	defer requestFile.Close()

	responseFile := os.NewFile(uintptr(info.fd2), "security-loader-response")
	if responseFile == nil {
		return cleanedArgs, true, fmt.Errorf("open security-loader response fd failed")
	}
	defer responseFile.Close()

	if err := validateSecurityLoaderFile(requestFile, syscall.O_WRONLY); err != nil {
		return cleanedArgs, true, fmt.Errorf("invalid security-loader request fd: %w", err)
	}
	if err := validateSecurityLoaderFile(responseFile, syscall.O_RDONLY); err != nil {
		return cleanedArgs, true, fmt.Errorf("invalid security-loader response fd: %w", err)
	}

	if _, err := requestFile.Write(requestData); err != nil {
		return cleanedArgs, true, fmt.Errorf("write security-loader request failed: %w", err)
	}
	if err := requestFile.Close(); err != nil {
		return cleanedArgs, true, fmt.Errorf("close security-loader request fd failed: %w", err)
	}

	type readResult struct {
		data []byte
		err  error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		data, err := readSecurityLoaderResponse(responseFile)
		resultCh <- readResult{data: data, err: err}
	}()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	var responseData []byte
	select {
	case result := <-resultCh:
		responseFile.Close()
		if result.err != nil {
			return cleanedArgs, true, fmt.Errorf("read security-loader response failed: %w", result.err)
		}
		responseData = result.data
	case <-timer.C:
		responseFile.Close()
		return cleanedArgs, true, fmt.Errorf("timeout waiting for security-loader response")
	}

	var response struct {
		Result  bool   `json:"Result"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(responseData, &response); err != nil {
		return cleanedArgs, true, fmt.Errorf("decode security-loader response failed: %w", err)
	}
	if !response.Result {
		return cleanedArgs, true, fmt.Errorf("security-loader authorization failed: %s", response.Message)
	}
	return cleanedArgs, true, nil
}

func parseLoaderArgs(args []string) (loaderInfo, []string, error) {
	info := loaderInfo{fd1: -1, fd2: -1}
	if len(args) == 0 {
		return info, nil, nil
	}

	cleaned := []string{args[0]}
	seenFD1 := false
	seenFD2 := false
	for i := 1; i < len(args); i++ {
		if args[i] != "--fd1" && args[i] != "--fd2" {
			cleaned = append(cleaned, args[i])
			continue
		}
		info.loaded = true
		if i+1 >= len(args) {
			return info, cleaned, fmt.Errorf("%s requires a file descriptor", args[i])
		}
		value, err := strconv.Atoi(args[i+1])
		if err != nil || value < 0 {
			return info, cleaned, fmt.Errorf("invalid %s value %q", args[i], args[i+1])
		}
		if args[i] == "--fd1" {
			if seenFD1 {
				return info, cleaned, fmt.Errorf("duplicate --fd1 argument")
			}
			seenFD1 = true
			info.fd1 = value
		} else {
			if seenFD2 {
				return info, cleaned, fmt.Errorf("duplicate --fd2 argument")
			}
			seenFD2 = true
			info.fd2 = value
		}
		i++
	}
	if info.loaded && (info.fd1 < 0 || info.fd2 < 0) {
		return info, cleaned, fmt.Errorf("security-loader requires both --fd1 and --fd2")
	}
	if info.loaded && info.fd1 == info.fd2 {
		return info, cleaned, fmt.Errorf("security-loader requires distinct --fd1 and --fd2")
	}
	return info, cleaned, nil
}

func validateSecurityLoaderFile(file *os.File, expectedAccessMode int) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		return fmt.Errorf("descriptor is not a pipe")
	}

	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, file.Fd(), uintptr(syscall.F_GETFL), 0)
	if errno != 0 {
		return errno
	}
	if int(flags)&syscall.O_ACCMODE != expectedAccessMode {
		return fmt.Errorf("unexpected descriptor access mode")
	}
	return nil
}

func readSecurityLoaderResponse(reader io.Reader) ([]byte, error) {
	data, err := ioutil.ReadAll(io.LimitReader(reader, maxSecurityLoaderResponseSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSecurityLoaderResponseSize {
		return nil, fmt.Errorf("response exceeds %d bytes", maxSecurityLoaderResponseSize)
	}
	return data, nil
}