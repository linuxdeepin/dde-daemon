// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSecretQuestionLimiterLocksAfterMaxAttempts(t *testing.T) {
	now := time.Unix(0, 0)
	limiter := newSecretQuestionLimiter()
	limiter.now = func() time.Time { return now }

	for i := 0; i < secretQuestionMaxAttempts-1; i++ {
		limiter.recordFailure("alice")
		locked, _ := limiter.locked("alice")
		assert.False(t, locked, "should not lock after %d failures", i+1)
	}

	limiter.recordFailure("alice")
	locked, remain := limiter.locked("alice")
	assert.True(t, locked)
	assert.Equal(t, secretQuestionLockDuration, remain)

	locked, _ = limiter.locked("bob")
	assert.False(t, locked, "lock must be per user")

	now = now.Add(secretQuestionLockDuration)
	locked, _ = limiter.locked("alice")
	assert.False(t, locked, "lock must expire")
}

func TestSecretQuestionLimiterBacksOff(t *testing.T) {
	now := time.Unix(0, 0)
	limiter := newSecretQuestionLimiter()
	limiter.now = func() time.Time { return now }

	for i := 0; i < secretQuestionMaxAttempts; i++ {
		limiter.recordFailure("alice")
	}
	now = now.Add(secretQuestionLockDuration)

	for i := 0; i < secretQuestionMaxAttempts; i++ {
		limiter.recordFailure("alice")
	}
	_, remain := limiter.locked("alice")
	assert.Equal(t, 2*secretQuestionLockDuration, remain)

	for i := 0; i < 20*secretQuestionMaxAttempts; i++ {
		limiter.recordFailure("alice")
		now = now.Add(secretQuestionMaxLockDuration)
	}
	_, remain = limiter.locked("alice")
	assert.LessOrEqual(t, remain, secretQuestionMaxLockDuration)
}

func TestSecretQuestionLimiterReset(t *testing.T) {
	limiter := newSecretQuestionLimiter()

	for i := 0; i < secretQuestionMaxAttempts-1; i++ {
		limiter.recordFailure("alice")
	}
	limiter.reset("alice")
	for i := 0; i < secretQuestionMaxAttempts-1; i++ {
		limiter.recordFailure("alice")
	}

	locked, _ := limiter.locked("alice")
	assert.False(t, locked, "successful verification must clear the failure count")
}
