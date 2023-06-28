// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package fonts

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path"
)

var (
	familyHashCacheFile = path.Join(home, ".cache", "deepin", "dde-daemon", "fonts", "family_hash")
)

func (table FamilyHashTable) saveToFile() error {
	return doSaveObject(familyHashCacheFile, &table)
}

func loadCacheFromFile(file string, obj interface{}) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	var r = bytes.NewBuffer(data)
	decoder := gob.NewDecoder(r)
	err = decoder.Decode(obj)
	if err != nil {
		return err
	}
	return nil
}

func doSaveObject(file string, obj interface{}) error {
	var w bytes.Buffer
	encoder := gob.NewEncoder(&w)
	err := encoder.Encode(obj)
	if err != nil {
		return err
	}

	err = os.MkdirAll(path.Dir(file), 0755)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, w.Bytes(), 0644)
}

func writeFontConfig(content, file string) error {
	err := os.MkdirAll(path.Dir(file), 0755)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, []byte(content), 0644)
}

func removeAll(path string) error {
	return os.RemoveAll(path)
}

// If set pixelsize, wps-office-wps will not show some text.
//
//func configContent(standard, mono string, pixel float64) string {
func configContent(standard, mono string) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "fonts.dtd">
<fontconfig>
    <match target="pattern">
        <test qual="any" name="family">
            <string>serif</string>
        </test>
        <edit name="family" mode="assign" binding="strong">
            <string>%s</string>
            <string>%s</string>
        </edit>
    </match>

    <match target="pattern">
        <test qual="any" name="family">
            <string>sans-serif</string>
        </test>
        <edit name="family" mode="assign" binding="strong">
            <string>%s</string>
            <string>%s</string>
        </edit>
    </match>

    <match target="pattern">
        <test qual="any" name="family">
            <string>monospace</string>
        </test>
        <edit name="family" mode="assign" binding="strong">
            <string>%s</string>
            <string>%s</string>
			<string>%s</string>
        </edit>
    </match>

    <match target="font">
        <edit name="rgba"><const>rgb</const></edit>
    </match>
</fontconfig>`, standard, fallbackStandard,
		standard, fallbackStandard,
		mono, fallbackMonospace, standard)
}
