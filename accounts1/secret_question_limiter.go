// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	"sync"
	"time"
)

const (
	// secretQuestionMaxAttempts is the number of consecutive failed
	// verification rounds tolerated before the first lock.
	secretQuestionMaxAttempts = 5
	// secretQuestionLockDuration is the first lock duration. Every further
	// lock doubles it up to secretQuestionMaxLockDuration.
	secretQuestionLockDuration    = time.Minute
	secretQuestionMaxLockDuration = 10 * time.Minute
)

type secretQuestionAttempt struct {
	failures    int
	lockedUntil time.Time
	lockCount   int
}

// secretQuestionLimiter throttles secret question verification per user so
// that answers cannot be guessed at an unbounded rate.
type secretQuestionLimiter struct {
	mu sync.Mutex
	// verifyMu serialises verification rounds so the lock check and the
	// failure accounting cannot interleave.
	verifyMu sync.Mutex
	attempts map[string]*secretQuestionAttempt
	now      func() time.Time
}

func newSecretQuestionLimiter() *secretQuestionLimiter {
	return &secretQuestionLimiter{
		attempts: make(map[string]*secretQuestionAttempt),
		now:      time.Now,
	}
}

var secretQuestionVerifyLimiter = newSecretQuestionLimiter()

// locked reports whether verification for the given user is throttled and how
// much time is left before the next attempt is accepted.
func (l *secretQuestionLimiter) locked(username string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, ok := l.attempts[username]
	if !ok {
		return false, 0
	}

	remain := attempt.lockedUntil.Sub(l.now())
	if remain <= 0 {
		return false, 0
	}
	return true, remain
}

// recordFailure counts one failed verification round and locks the user once
// secretQuestionMaxAttempts consecutive rounds have failed.
func (l *secretQuestionLimiter) recordFailure(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, ok := l.attempts[username]
	if !ok {
		attempt = &secretQuestionAttempt{}
		l.attempts[username] = attempt
	}

	attempt.failures++
	if attempt.failures < secretQuestionMaxAttempts {
		return
	}

	attempt.failures = 0
	attempt.lockCount++

	duration := secretQuestionLockDuration
	for i := 1; i < attempt.lockCount && duration < secretQuestionMaxLockDuration; i++ {
		duration *= 2
	}
	if duration > secretQuestionMaxLockDuration {
		duration = secretQuestionMaxLockDuration
	}
	attempt.lockedUntil = l.now().Add(duration)
}

// reset clears the failure state after a successful verification round.
func (l *secretQuestionLimiter) reset(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.attempts, username)
}
