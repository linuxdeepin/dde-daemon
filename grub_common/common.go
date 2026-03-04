// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package grub_common

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/linuxdeepin/go-lib/encoding/kv"
	"github.com/linuxdeepin/go-lib/log"
)

var logger = log.NewLogger("grub_common")

const (
	GrubParamsFile            = "/etc/default/grub"
	DDEGrubParamsFile         = "/etc/default/grub.d/11_dde.cfg"
	GfxmodeDetectReadyPath    = "/tmp/deepin-gfxmode-detect-ready"
	DeepinGfxmodeDetect       = "DEEPIN_GFXMODE_DETECT"
	DeepinGfxmodeAdjusted     = "DEEPIN_GFXMODE_ADJUSTED"
	DeepinGfxmodeNotSupported = "DEEPIN_GFXMODE_NOT_SUPPORTED"
	sysClassDrm               = "/sys/class/drm"
	deepinGfxmodeMod          = "deepin_gfxmode.mod"
	GrubDistributor           = "GRUB_DISTRIBUTOR"
	GrubCmdlineLinuxDefault   = "GRUB_CMDLINE_LINUX_DEFAULT"
)

func LoadGrubParamsFile(file string) (map[string]string, error) {
	params := make(map[string]string)
	f, err := os.Open(file)
	if err != nil {
		return params, err
	}
	defer f.Close()

	r := kv.NewReader(f)
	r.TrimSpace = kv.TrimLeadingTailingSpace
	r.Comment = '#'
	for {
		pair, err := r.Read()
		if err != nil {
			break
		}
		if pair.Key == "" {
			continue
		}
		params[pair.Key] = pair.Value
	}

	return params, nil
}

func LoadGrubParams() map[string]string {
	params := make(map[string]string)

	// First read the main configuration file
	if err := readGrubParamsFile(GrubParamsFile, params); err != nil {
		fmt.Fprintf(os.Stderr, "failed to load %s: %v\n", GrubParamsFile, err)
	}
	// Then read the DDE configuration file
	if err := readGrubParamsFile(DDEGrubParamsFile, params); err != nil {
		fmt.Fprintf(os.Stderr, "failed to load %s: %v\n", DDEGrubParamsFile, err)
	}

	return params
}

func readGrubParamsFile(filePath string, params map[string]string) error {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comment lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Find the position of the first equal sign
		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			// Not in key=value format, skip
			continue
		}
		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])
		if key == "" {
			continue
		}
		params[key] = value
	}
	return scanner.Err()
}

func DecodeShellValue(in string) string {
	output, err := exec.Command("/bin/sh", "-c", "echo -n "+in).Output()
	if err != nil {
		// fallback
		return strings.Trim(in, "\"")
	}
	return string(output)
}

type Gfxmode struct {
	Width  int
	Height int
}

func (v Gfxmode) String() string {
	return fmt.Sprintf("%dx%d", v.Width, v.Height)
}

func getBootArgDeepinGfxmode() (string, error) {
	filename := "/proc/cmdline"
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	fields := bytes.Split(content, []byte(" "))
	const prefix = "DEEPIN_GFXMODE="
	var result string
	for _, field := range fields {
		if bytes.HasPrefix(field, []byte(prefix)) {
			result = string(bytes.TrimSpace(field[len(prefix):]))
			break
		}
	}

	return result, nil
}

func GetBootArgDeepinGfxmode() (cur Gfxmode, all Gfxmodes, err error) {
	deepinGfxmode, err := getBootArgDeepinGfxmode()
	if err != nil {
		return
	}

	cur, all, err = parseBootArgDeepinGfxmode(deepinGfxmode)
	return
}

func parseBootArgDeepinGfxmode(str string) (cur Gfxmode, all Gfxmodes, err error) {
	fields := strings.Split(str, ",")
	if len(fields) < 2 {
		err = errors.New("length of fields < 2")
		return
	}
	var curIdx int
	curIdx, err = strconv.Atoi(string(fields[0]))
	if err != nil {
		return
	}

	for _, field := range fields[1:] {
		var m Gfxmode
		m, err = ParseGfxmode(field)
		if err != nil {
			return
		}

		all = append(all, m)
	}

	if curIdx < 0 || curIdx >= len(all) {
		err = fmt.Errorf("curIdx %d out of range [0,%d]", curIdx, len(all))
		return
	}

	cur = all[curIdx]
	return cur, all, nil
}

var gfxmodeReg = regexp.MustCompile(`^\d+x\d+$`)

func ParseGfxmode(str string) (Gfxmode, error) {
	if !gfxmodeReg.MatchString(str) {
		return Gfxmode{}, fmt.Errorf("invalid gfxmode %q", str)
	}

	var v Gfxmode
	_, err := fmt.Sscanf(str, "%dx%d", &v.Width, &v.Height)
	if err != nil {
		return Gfxmode{}, err
	}

	return v, nil
}

type Gfxmodes []Gfxmode

func (v Gfxmodes) Len() int {
	return len(v)
}

func (v Gfxmodes) Less(i, j int) bool {
	a := v[i]
	b := v[j]

	return a.Width*a.Height < b.Width*b.Height
}

func (v Gfxmodes) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

func (v Gfxmodes) Add(m Gfxmode) Gfxmodes {
	var found bool
	for _, r0 := range v {
		if r0 == m {
			found = true
			break
		}
	}
	if !found {
		return append(v, m)
	}
	return v
}

func (v Gfxmodes) Max() (max Gfxmode) {
	for _, m := range v {
		if m.Width*m.Height > max.Width*max.Height {
			max = m
		}
	}
	return
}

func (v Gfxmodes) Intersection(v1 Gfxmodes) (result Gfxmodes) {
	dict := make(map[Gfxmode]struct{})
	for _, m := range v {
		dict[m] = struct{}{}
	}

	for _, m := range v1 {
		if _, ok := dict[m]; ok {
			result = append(result, m)
		}
	}
	return
}

func (v Gfxmodes) SortDesc() {
	sort.Sort(sort.Reverse(v))
}

func ShouldFinishGfxmodeDetect(params map[string]string) bool {
	if params[DeepinGfxmodeDetect] == "1" {
		_, err := os.Stat(GfxmodeDetectReadyPath)
		if os.IsNotExist(err) {
			return true
		}
	}
	return false
}

func InGfxmodeDetectionMode(params map[string]string) bool {
	return params[DeepinGfxmodeDetect] == "1"
}

func IsGfxmodeDetectFailed(params map[string]string) bool {
	return params[DeepinGfxmodeDetect] == "2"
}

func HasDeepinGfxmodeMod() bool {
	const dir = "/usr/lib/grub"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	hasI386PC := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Prefer -efi platforms, check immediately
		if strings.Contains(name, "-efi") {
			if _, err := os.Stat(filepath.Join(dir, name, deepinGfxmodeMod)); err == nil {
				return true
			}
		} else if name == "i386-pc" {
			hasI386PC = true
		}
	}

	// Fallback to i386-pc
	if hasI386PC {
		if _, err := os.Stat(filepath.Join(dir, "i386-pc", deepinGfxmodeMod)); err == nil {
			return true
		}
	}

	return false
}

var regCardOutput = regexp.MustCompile(`^card\d+-.+`)

// GetGfxmodesFromSysDrm reads graphics modes from /sys/class/drm/
// This function does not require X11 and works in headless environments.
func GetGfxmodesFromSysDrm() (Gfxmodes, error) {
	fileInfos, err := os.ReadDir(sysClassDrm)
	if err != nil {
		return nil, err
	}

	var connectedOutputs []string
	for _, info := range fileInfos {
		name := info.Name()
		if regCardOutput.MatchString(name) {
			status, err := os.ReadFile(filepath.Join(sysClassDrm, name, "status"))
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(status)) == "connected" {
				connectedOutputs = append(connectedOutputs, name)
			}
		}
	}

	if len(connectedOutputs) == 0 {
		return Gfxmodes{Gfxmode{Width: 1024, Height: 768}}, nil
	}

	// Collect modes from all connected outputs
	gfxmodeMap := make(map[Gfxmode]int)
	for _, output := range connectedOutputs {
		modesPath := filepath.Join(sysClassDrm, output, "modes")
		modesFile, err := os.Open(modesPath)
		if err != nil {
			continue
		}
		defer modesFile.Close()

		// Use a map to deduplicate modes for this output
		outputModes := make(map[Gfxmode]struct{})
		scanner := bufio.NewScanner(modesFile)
		for scanner.Scan() {
			modeStr := scanner.Text()
			mode, err := ParseGfxmode(modeStr)
			if err != nil {
				continue
			}
			if mode.Width < 1024 || mode.Height < 720 {
				continue
			}
			outputModes[mode] = struct{}{}
		}

		// Add unique modes to the global map
		for mode := range outputModes {
			gfxmodeMap[mode]++
		}
	}

	// Find modes supported by all connected outputs
	var result Gfxmodes
	for mode, count := range gfxmodeMap {
		if count == len(connectedOutputs) {
			result = result.Add(mode)
		}
	}

	if len(result) == 0 {
		result = Gfxmodes{Gfxmode{Width: 1024, Height: 768}}
	}

	return result, nil
}

// GetConnectedEdidsHash reads the EDID of every connected DRM output,
// sorts the collected EDID blobs lexicographically, feeds them in order
// into an MD5 hasher, and returns the resulting digest as a lowercase
// hex string.
func GetConnectedEdidsHash() (string, error) {
	fileInfos, err := os.ReadDir(sysClassDrm)
	if err != nil {
		return "", err
	}

	var edids [][]byte
	for _, info := range fileInfos {
		name := info.Name()
		if !regCardOutput.MatchString(name) {
			continue
		}

		statusPath := filepath.Join(sysClassDrm, name, "status")
		status, err := os.ReadFile(statusPath)
		if err != nil {
			logger.Warningf("GetConnectedEdidsHash: failed to read %s: %v", statusPath, err)
			continue
		}
		statusStr := strings.TrimSpace(string(status))
		logger.Debugf("GetConnectedEdidsHash: %s status=%q", statusPath, statusStr)
		if statusStr != "connected" {
			continue
		}

		edidPath := filepath.Join(sysClassDrm, name, "edid")
		edid, err := os.ReadFile(edidPath)
		if err != nil {
			logger.Warningf("GetConnectedEdidsHash: failed to read %s: %v", edidPath, err)
			continue
		}
		if len(edid) == 0 {
			logger.Debugf("GetConnectedEdidsHash: %s is empty, skipped", edidPath)
			continue
		}
		logger.Debugf("GetConnectedEdidsHash: %s (%d bytes):\n%s",
			edidPath, len(edid), hex.Dump(edid))
		edids = append(edids, edid)
	}

	sort.Slice(edids, func(i, j int) bool {
		return bytes.Compare(edids[i], edids[j]) < 0
	})

	h := md5.New()
	for _, edid := range edids {
		h.Write(edid)
	}
	result := fmt.Sprintf("%x", h.Sum(nil))
	logger.Debugf("GetConnectedEdidsHash: total %d EDIDs, hash=%s", len(edids), result)
	return result, nil
}
