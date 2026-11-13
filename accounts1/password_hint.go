// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package accounts

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

// maxPasswordHintLength bounds what can be written into the user config file.
const maxPasswordHintLength = 512

// validatePasswordHint rejects hints that would not round-trip through the
// user config file: too long, or containing control characters.
func validatePasswordHint(hint string) error {
	if utf8.RuneCountInString(hint) > maxPasswordHintLength {
		return fmt.Errorf("password hint exceeds %d characters", maxPasswordHintLength)
	}
	for _, r := range hint {
		if r < 0x20 || r == 0x7f {
			return errors.New("password hint contains control characters")
		}
	}
	return nil
}
