// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub_gfx

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/linuxdeepin/dde-daemon/grub_common"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

var detectCacheFile = filepath.Join(basedir.GetUserCacheDir(),
	"deepin/dde-daemon/grub_gfx/detect.json")

// DetectCache stores the cached result of gfxmode detection,
// including the EDID hash and the maximum supported gfxmode.
type DetectCache struct {
	EdidsHash  string `json:"edidsHash"`
	MaxGfxmode string `json:"maxGfxmode"`
}

// equal reports whether the cache matches the given EDID hash and maximum gfxmode.
func (c DetectCache) equal(edidsHash string, maxGfxmode grub_common.Gfxmode) bool {
	return c.EdidsHash == edidsHash && c.MaxGfxmode == maxGfxmode.String()
}

// loadDetectCache reads and deserializes the detect cache from disk.
func loadDetectCache() (DetectCache, error) {
	var cache DetectCache
	data, err := os.ReadFile(detectCacheFile)
	if err != nil {
		return cache, err
	}
	err = json.Unmarshal(data, &cache)
	return cache, err
}

// saveDetectCache serializes and writes the detect cache to disk.
func saveDetectCache(cache DetectCache) error {
	if cache.EdidsHash == "" || cache.MaxGfxmode == "" {
		return errors.New("invalid detect cache")
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(detectCacheFile), 0755)
	if err != nil {
		return err
	}
	return os.WriteFile(detectCacheFile, data, 0644)
}
