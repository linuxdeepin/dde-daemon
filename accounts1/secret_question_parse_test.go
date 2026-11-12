// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectQuestions(t *testing.T, content string) (ids []int, count int, err error) {
	t.Helper()
	count, err = parseQuestions(bufio.NewReader(strings.NewReader(content)), func(id int, salt string) error {
		ids = append(ids, id)
		return nil
	})
	return ids, count, err
}

func TestParseQuestionsDoesNotDropLastLine(t *testing.T) {
	ids, count, err := collectQuestions(t, "1:salt1\n2:salt2")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []int{1, 2}, ids)
}

func TestParseQuestionsDoesNotStopAtBlankLine(t *testing.T) {
	ids, count, err := collectQuestions(t, "1:salt1\n\n2:salt2\n")
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []int{1, 2}, ids)
}

func TestParseQuestionsRejectsMalformedLine(t *testing.T) {
	ids, count, err := collectQuestions(t, "1:salt1\nbroken\n2:salt2\n")
	assert.Error(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, []int{1}, ids)
}

func TestParseQuestionsStopsOnVisitorError(t *testing.T) {
	count, err := parseQuestions(bufio.NewReader(strings.NewReader("1:salt1\n2:salt2\n")),
		func(id int, salt string) error {
			return assert.AnError
		})
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, 0, count)
}

func TestParseQuestionsEmptyStore(t *testing.T) {
	ids, count, err := collectQuestions(t, "")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Empty(t, ids)
}
