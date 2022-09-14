// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Result score && Sorted by it
const (
	POOR          uint32 = 50000 //nolint
	BELOW_AVERAGE        = 60000
	AVERAGE              = 70000
	ABOVE_AVERAGE        = 75000
	GOOD                 = 80000
	VERY_GOOD            = 85000
	EXCELLENT            = 90000
	HIGHEST              = 100000
)

func addMatcher(template, key string,
	score uint32, m map[*regexp.Regexp]uint32) error {
	regStr := fmt.Sprintf(template, key)
	logger.Debugf("addMatcher score: %d, regexp: %s", score, regStr)
	reg, err := regexp.Compile(regStr)
	if err != nil {
		logger.Warningf("bad regex %s : %v", regStr, err)
		return err
	}

	m[reg] = score
	return nil
}

func splitKey(key string) []string {
	var chars []string
	var isPrevCharEscape bool
	for _, r := range key {
		if isPrevCharEscape {
			chars = append(chars, "\\"+string(r))
			isPrevCharEscape = false
		} else {
			if r == '\\' {
				isPrevCharEscape = true
			} else {
				chars = append(chars, string(r))
				isPrevCharEscape = false
			}
		}
	}
	return chars
}

// learnt from synapse
func getMatchers(key string) map[*regexp.Regexp]uint32 {
	logger.Debugf("getMatchers key %s", key)
	// * create a couple of regexes and try to help with matching
	// * match with these regular expressions (with descending score):
	// * 1) ^query$
	// * 2) ^query
	// * 3) \bquery
	// * 4) split to words and search \bword1.+\bword2 (if there are 2+ words)
	// * 5) query
	// * 6) split to characters and search \bq.+\bu.+\be.+\br.+\by
	// * 7) split to characters and search \bq.*u.*e.*r.*y
	m := make(map[*regexp.Regexp]uint32)

	_ = addMatcher(`(?i)^%s$`, key, HIGHEST, m)
	_ = addMatcher(`(?i)^%s`, key, EXCELLENT, m)
	_ = addMatcher(`(?i)\b%s`, key, VERY_GOOD, m)

	words := strings.Fields(key)
	if len(words) > 1 {
		_ = addMatcher(`(?i)\b%s`, strings.Join(words, `.+\b`),
			GOOD, m)
	}

	_ = addMatcher(`(?i)%s`, key, BELOW_AVERAGE, m)

	chars := splitKey(key)
	logger.Debugf("chars %#v", chars)
	if len(words) == 1 && len(chars) <= 5 {
		_ = addMatcher(`(?i)\b%s`, strings.Join(chars, `.+\b`),
			ABOVE_AVERAGE, m)
	}

	_ = addMatcher(`(?i)\b%s`, strings.Join(chars, `.*`), BELOW_AVERAGE, m)
	_ = addMatcher(`(?i)%s`, strings.Join(chars, `.*`), POOR, m)

	return m
}
