// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"github.com/linuxdeepin/go-lib/locale"
)

// key is locale code
type DefaultFontConfig map[string]FontConfigItem

type FontConfigItem struct {
	Standard  string
	Monospace string `json:"Mono"`
}

func (cfg DefaultFontConfig) Get() (standard, monospace string) {
	languages := locale.GetLanguageNames()
	for _, lang := range languages {
		if item, ok := cfg[lang]; ok {
			return item.Standard, item.Monospace
		}
	}

	defaultItem := cfg["en_US"]
	return defaultItem.Standard, defaultItem.Monospace
}
