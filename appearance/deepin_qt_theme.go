// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package appearance

import (
	"fmt"
	"path"
	"sync"

	"github.com/linuxdeepin/go-lib/keyfile"
	"github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/go-lib/xdg/basedir"
)

const (
	dQtSectionTheme = "Theme"
	dQtKeyIcon      = "IconThemeName"
	dQtKeyFont      = "Font"
	dQtKeyMonoFont  = "MonoFont"
	dQtKeyFontSize  = "FontSize"
)

var dQtFile = path.Join(basedir.GetUserConfigDir(), "deepin", "qt-theme.ini")

func setDQtTheme(file, section string, keys, values []string) error {
	_dQtLocker.Lock()
	defer _dQtLocker.Unlock()

	keyLen := len(keys)
	if keyLen != len(values) {
		return fmt.Errorf("keys - values not match: %d - %d", keyLen, len(values))
	}

	_, err := getDQtHandler(file)
	if err != nil {
		return err
	}

	for i := 0; i < keyLen; i++ {
		v, _ := _dQtHandler.GetString(section, keys[i])
		if v == values[i] {
			continue
		}
		_dQtHandler.SetString(section, keys[i], values[i])
		_needSave = true
	}
	return nil
}

func saveDQtTheme(file string) error {
	_dQtLocker.Lock()
	defer _dQtLocker.Unlock()

	if !_needSave {
		_dQtHandler = nil
		return nil
	}
	_needSave = false

	_, err := getDQtHandler(file)
	if err != nil {
		return err
	}

	err = _dQtHandler.SaveToFile(file)
	_dQtHandler = nil
	return err
}

var (
	_dQtHandler *keyfile.KeyFile
	_dQtLocker  sync.Mutex
	_needSave   bool = false
)

func getDQtHandler(file string) (*keyfile.KeyFile, error) {
	if _dQtHandler != nil {
		return _dQtHandler, nil
	}

	if !utils.IsFileExist(file) {
		err := utils.CreateFile(file)
		if err != nil {
			logger.Debug("Failed to create qt theme file:", file, err)
			return nil, err
		}
	}

	kf := keyfile.NewKeyFile()
	err := kf.LoadFromFile(file)
	if err != nil {
		logger.Debug("Failed to load qt theme file:", file, err)
		return nil, err
	}
	_dQtHandler = kf
	return _dQtHandler, nil
}
