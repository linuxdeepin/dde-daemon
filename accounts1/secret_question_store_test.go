// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestWriteSecretQuestionsCreatesPrivateFile(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, writeSecretQuestions(dir, "alice", []byte("1:salt1\n")))

	path := filepath.Join(dir, "alice")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "1:salt1\n", string(content))
}

func TestWriteSecretQuestionsRejectsSymlink(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, []byte("untouched"), 0600))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "alice")))

	require.NoError(t, writeSecretQuestions(dir, "alice", []byte("1:salt1\n")))

	content, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, "untouched", string(content))

	// The symlink itself must have been replaced by a regular file.
	info, err := os.Lstat(filepath.Join(dir, "alice"))
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
}

func TestLoadSecretQuestionsRejectsSymlink(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, []byte("1:salt1\n"), 0600))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "alice")))

	_, err := loadSecretQuestions(dir, "alice")
	require.Error(t, err)
}

func TestLoadSecretQuestionsReadsFinalLineWithoutNewline(t *testing.T) {
	dir := t.TempDir()

	content := "1:salt1\n2:salt2\n3:salt3"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alice"), []byte(content), 0600))

	questions, err := loadSecretQuestions(dir, "alice")
	require.NoError(t, err)
	assert.Equal(t, []secretQuestion{
		{id: 1, salt: "salt1"},
		{id: 2, salt: "salt2"},
		{id: 3, salt: "salt3"},
	}, questions)
}

func TestLoadSecretQuestionsRejectsBlankLine(t *testing.T) {
	dir := t.TempDir()

	content := "1:salt1\n\n2:salt2\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alice"), []byte(content), 0600))

	_, err := loadSecretQuestions(dir, "alice")
	require.Error(t, err)
}

func TestLoadSecretQuestionsRejectsEmptyFile(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "alice"), nil, 0600))

	_, err := loadSecretQuestions(dir, "alice")
	require.Error(t, err)
}

func TestLoadSecretQuestionsMissingFile(t *testing.T) {
	dir := t.TempDir()

	questions, err := loadSecretQuestions(dir, "alice")
	require.NoError(t, err)
	assert.Empty(t, questions)
}

func TestLoadSecretQuestionsRejectsDuplicateID(t *testing.T) {
	dir := t.TempDir()

	content := "1:salt1\n1:salt2\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alice"), []byte(content), 0600))

	_, err := loadSecretQuestions(dir, "alice")
	require.Error(t, err)
}

func TestWriteSecretQuestionsKeepsOldContentOnFailure(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, writeSecretQuestions(dir, "alice", []byte("1:salt1\n")))

	// A write that cannot be completed must not truncate the live store.
	require.NoError(t, os.Chmod(dir, 0500))
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0700)
	})

	err := writeSecretQuestions(dir, "alice", []byte("2:salty\n"))
	require.Error(t, err)

	content, readErr := os.ReadFile(filepath.Join(dir, "alice"))
	require.NoError(t, readErr)
	assert.Equal(t, "1:salt1\n", string(content))
}

func TestLoadSecretQuestionsRejectsFifo(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, unix.Mkfifo(filepath.Join(dir, "alice"), 0600))

	_, err := loadSecretQuestions(dir, "alice")
	require.Error(t, err)
}

func TestLoadSecretQuestionsRejectsHardLink(t *testing.T) {
	dir := t.TempDir()

	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, []byte("1:salt1\n"), 0600))
	require.NoError(t, os.Link(target, filepath.Join(dir, "alice")))

	_, err := loadSecretQuestions(dir, "alice")
	require.Error(t, err)
}

func TestVerifySecretAnswersRejectsIncompleteStore(t *testing.T) {
	tests := []struct {
		name      string
		questions []secretQuestion
	}{
		{name: "empty"},
		{
			name: "too few",
			questions: []secretQuestion{
				{id: 1, salt: "salt1"},
				{id: 2, salt: "salt2"},
			},
		},
		{
			name: "too many",
			questions: []secretQuestion{
				{id: 1, salt: "salt1"},
				{id: 2, salt: "salt2"},
				{id: 3, salt: "salt3"},
				{id: 4, salt: "salt4"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failed, err := verifySecretAnswers(tt.questions, map[int]string{})
			require.Error(t, err)
			assert.Nil(t, failed)
		})
	}
}

func TestVerifySecretAnswersTreatsWrongAndMissingAsFailures(t *testing.T) {
	questions := []secretQuestion{
		{id: 1, salt: "salt1"},
		{id: 2, salt: "salt2"},
		{id: 3, salt: "salt3"},
	}

	failed, err := verifySecretAnswers(questions, map[int]string{1: "answer1"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []int{1, 2, 3}, failed)
}

func TestValidateSecretQuestionList(t *testing.T) {
	valid := map[int][]byte{
		1: []byte("salt1"),
		2: []byte("salt2"),
		3: []byte("salt3"),
	}

	tests := []struct {
		name    string
		list    map[int][]byte
		wantErr bool
	}{
		{name: "valid", list: valid},
		{name: "wrong count", list: map[int][]byte{1: []byte("salt1")}, wantErr: true},
		{name: "non-positive id", list: map[int][]byte{
			0: []byte("salt1"), 2: []byte("salt2"), 3: []byte("salt3"),
		}, wantErr: true},
		{name: "empty answer", list: map[int][]byte{
			1: nil, 2: []byte("salt2"), 3: []byte("salt3"),
		}, wantErr: true},
		{name: "line break", list: map[int][]byte{
			1: []byte("salt1\n"), 2: []byte("salt2"), 3: []byte("salt3"),
		}, wantErr: true},
		{name: "oversized answer", list: map[int][]byte{
			1: []byte(strings.Repeat("a", maxSecretQuestionAnswerSize+1)),
			2: []byte("salt2"),
			3: []byte("salt3"),
		}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecretQuestionList(tt.list)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
