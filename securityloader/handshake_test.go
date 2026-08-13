// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package securityloader

import (
	"bytes"
	"io/ioutil"
	"os"
	"reflect"
	"syscall"
	"testing"
)

func TestParseLoaderArgs(t *testing.T) {
	info, args, err := parseLoaderArgs([]string{
		"dde-session-daemon", "--verbose", "--fd1", "7", "--fd2", "8", "--force",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !info.loaded || info.fd1 != 7 || info.fd2 != 8 {
		t.Fatalf("unexpected loader info: %+v", info)
	}
	want := []string{"dde-session-daemon", "--verbose", "--force"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected cleaned args: got %v, want %v", args, want)
	}
}

func TestParseLoaderArgsRequiresBothFDs(t *testing.T) {
	_, _, err := parseLoaderArgs([]string{"startdde", "--fd1", "7"})
	if err == nil {
		t.Fatal("expected incomplete loader arguments to fail")
	}
}

func TestParseLoaderArgsRejectsDuplicateFDs(t *testing.T) {
	tests := [][]string{
		{"dde-session-daemon", "--fd1", "7", "--fd1", "9", "--fd2", "8"},
		{"dde-session-daemon", "--fd1", "7", "--fd2", "8", "--fd2", "9"},
	}
	for _, args := range tests {
		if _, _, err := parseLoaderArgs(args); err == nil {
			t.Fatalf("expected duplicate loader arguments to fail: %v", args)
		}
	}
}

func TestParseLoaderArgsRequiresDistinctFDs(t *testing.T) {
	_, _, err := parseLoaderArgs([]string{
		"dde-session-daemon", "--fd1", "7", "--fd2", "7",
	})
	if err == nil {
		t.Fatal("expected identical loader file descriptors to fail")
	}
}

func TestReadSecurityLoaderResponseLimit(t *testing.T) {
	valid := bytes.Repeat([]byte{'a'}, int(maxSecurityLoaderResponseSize))
	data, err := readSecurityLoaderResponse(bytes.NewReader(valid))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(valid) {
		t.Fatalf("unexpected response length: got %d, want %d", len(data), len(valid))
	}

	oversized := bytes.Repeat([]byte{'a'}, int(maxSecurityLoaderResponseSize)+1)
	if _, err := readSecurityLoaderResponse(bytes.NewReader(oversized)); err == nil {
		t.Fatal("expected oversized loader response to fail")
	}
}

func TestValidateSecurityLoaderFile(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	if err := validateSecurityLoaderFile(writer, syscall.O_WRONLY); err != nil {
		t.Fatalf("expected pipe writer to be valid: %v", err)
	}
	if err := validateSecurityLoaderFile(reader, syscall.O_RDONLY); err != nil {
		t.Fatalf("expected pipe reader to be valid: %v", err)
	}
	if err := validateSecurityLoaderFile(reader, syscall.O_WRONLY); err == nil {
		t.Fatal("expected pipe access mode mismatch to fail")
	}

	regularFile, err := ioutil.TempFile(t.TempDir(), "security-loader")
	if err != nil {
		t.Fatal(err)
	}
	defer regularFile.Close()
	if err := validateSecurityLoaderFile(regularFile, syscall.O_RDWR); err == nil {
		t.Fatal("expected regular file descriptor to fail")
	}
}

func TestHandshakeReportsLoaderState(t *testing.T) {
	args, loaded, err := Handshake([]string{"dde-session-daemon", "--verbose"}, []Destination{{}})
	if err != nil {
		t.Fatal(err)
	}
	if loaded {
		t.Fatal("direct invocation was reported as security-loader invocation")
	}
	want := []string{"dde-session-daemon", "--verbose"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: got %v, want %v", args, want)
	}

	_, loaded, err = Handshake([]string{"dde-session-daemon", "--fd1", "7"}, []Destination{{}})
	if err == nil {
		t.Fatal("expected incomplete loader arguments to fail")
	}
	if !loaded {
		t.Fatal("invalid injected arguments did not report security-loader invocation")
	}
}
