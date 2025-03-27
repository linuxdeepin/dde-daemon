// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
	hostname1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.hostname1"
	"github.com/linuxdeepin/go-gir/gudev-1.0"
	"github.com/linuxdeepin/go-lib/utils"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

func getRotations(origin uint16) []uint16 {
	var ret []uint16

	if origin&randr.RotationRotate0 == randr.RotationRotate0 {
		ret = append(ret, randr.RotationRotate0)
	}
	if origin&randr.RotationRotate90 == randr.RotationRotate90 {
		ret = append(ret, randr.RotationRotate90)
	}
	if origin&randr.RotationRotate180 == randr.RotationRotate180 {
		ret = append(ret, randr.RotationRotate180)
	}
	if origin&randr.RotationRotate270 == randr.RotationRotate270 {
		ret = append(ret, randr.RotationRotate270)
	}
	return ret
}

func getReflects(origin uint16) []uint16 {
	var ret = []uint16{0}

	if origin&randr.RotationReflectX == randr.RotationReflectX {
		ret = append(ret, randr.RotationReflectX)
	}
	if origin&randr.RotationReflectY == randr.RotationReflectY {
		ret = append(ret, randr.RotationReflectY)
	}
	if len(ret) == 3 {
		ret = append(ret, randr.RotationReflectX|randr.RotationReflectY)
	}
	return ret
}

func parseCrtcRotation(origin uint16) (rotation, reflect uint16) {
	rotation = origin & 0xf
	reflect = origin & 0xf0

	switch rotation {
	case 1, 2, 4, 8:
		break
	default:
		//Invalid rotation value
		rotation = 1
	}

	switch reflect {
	case 0, 16, 32, 48:
		break
	default:
		// Invalid reflect value
		reflect = 0
	}

	return
}

func formatRate(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

func uint16SliceEqual(v1, v2 []uint16) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i, e1 := range v1 {
		if e1 != v2[i] {
			return false
		}
	}
	return true
}

func objPathsEqual(v1, v2 []dbus.ObjectPath) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i, e1 := range v1 {
		if e1 != v2[i] {
			return false
		}
	}
	return true
}

func outputSliceEqual(v1, v2 []randr.Output) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i, e1 := range v1 {
		if e1 != v2[i] {
			return false
		}
	}
	return true
}

func outputSliceContains(outputs []randr.Output, output randr.Output) bool {
	for _, o := range outputs {
		if o == output {
			return true
		}
	}
	return false
}

func getMonitorsCommonSizes(monitors []*Monitor) []Size {
	var notUseBestMode bool
	count := make(map[Size]int)
	bestMode := monitors[0].BestMode
	for _, monitor := range monitors {
		if bestMode != monitor.BestMode {
			notUseBestMode = true
		}
		smm := getSizeModeMap(monitor.Modes)
		for size := range smm {
			count[size]++
		}
	}

	var commonSizes []Size
	if !notUseBestMode {
		return []Size{{bestMode.Width, bestMode.Height}}
	}

	for size, value := range count {
		if value == len(monitors) {
			commonSizes = append(commonSizes, size)
		}
	}
	return commonSizes
}

func getMaxAreaSize(sizes []Size) Size {
	if len(sizes) == 0 {
		return Size{}
	}
	maxS := sizes[0]
	for _, s := range sizes[1:] {
		if (int(maxS.width) * int(maxS.height)) < (int(s.width) * int(s.height)) {
			maxS = s
		}
	}
	return maxS
}

func parseEdid(edid []byte) (string, string) {
	if len(edid) < 16 {
		return "DEFAULT", ""
	}
	var brandInf = edid[8:12]
	var bInf = uint64(brandInf[0])<<8 + uint64(brandInf[1])
	var maInf []byte
	var k uint
	for k = 1; k <= 3; k++ {
		m := byte(((bInf >> (15 - 5*k)) & 31) + 'A' - 1)
		if m >= 'A' && m <= 'Z' {
			maInf = append(maInf, m)
		} else {
			return "DEFAULT", ""
		}
	}

	// 截取显示器型号信息
	if len(edid) < 128 {
		return string(maInf), ""
	}
	modelInf := edid[88:112]
	var moInf []byte
	var isHaveInf = false
	for _, m := range modelInf {
		if m >= '!' && m <= '~' {
			moInf = append(moInf, m)
			isHaveInf = true
		}
		if isHaveInf && (m < '!' || m > '~') { // 截取型号信息终止
			break
		}
	}
	if !isHaveInf { // 如果没有型号信息，则解析厂家内部小版本号作为类型信息
		for i := 2; i <= 3; i++ {
			strInf := []byte(strconv.Itoa(int(brandInf[i])))
			for j := 0; j < len(strInf); j++ {
				moInf = append(moInf, strInf[j])
			}
		}
	}
	return string(maInf), string(moInf)
}

func getOutputUuid(name, stdName string, edid []byte) string {
	if len(edid) < 128 {
		return name + "||v1"
	}

	id, _ := utils.SumStrMd5(string(edid[:128]))
	if id == "" {
		return name + "||v1"
	}

	if stdName != "" {
		name = "@" + stdName
	}

	return name + "|" + id + "|v1"
}

func getOutputUuidV0(name string, edid []byte) string {
	if len(edid) < 128 {
		return name
	}

	id, _ := utils.SumStrMd5(string(edid[:128]))
	if id == "" {
		return name
	}

	return name + id
}

func sortMonitorsByPrimaryAndId(monitors []*Monitor, primary *Monitor) {
	sort.Slice(monitors, func(i, j int) bool {
		if primary == nil {
			return monitors[i].ID < monitors[j].ID
		}
		if monitors[i].Name == primary.Name {
			return true
		} else if monitors[j].Name == primary.Name {
			return false
		}
		return monitors[i].ID < monitors[j].ID
	})
}

func getMinIdMonitor(monitors []*Monitor) *Monitor {
	if len(monitors) == 0 {
		return nil
	}

	minMonitor := monitors[0]
	for _, monitor := range monitors[1:] {
		if minMonitor.ID > monitor.ID {
			minMonitor = monitor
		}
	}
	return minMonitor
}

func jsonMarshal(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func jsonUnmarshal(data string, ret interface{}) error {
	return json.Unmarshal([]byte(data), ret)
}

func needSwapWidthHeight(rotation uint16) bool {
	return rotation&randr.RotationRotate90 != 0 ||
		rotation&randr.RotationRotate270 != 0
}

func swapWidthHeightWithRotation(rotation uint16, pWidth, pHeight *uint16) {
	if needSwapWidthHeight(rotation) {
		*pWidth, *pHeight = *pHeight, *pWidth
	}
}

func swapWidthHeightWithRotationInt32(rotation uint16, pWidth, pHeight *int32) {
	if needSwapWidthHeight(rotation) {
		*pWidth, *pHeight = *pHeight, *pWidth
	}
}

func getConfigVersion(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(content)), nil
}

func getComputeChassis() (string, error) {
	const chassisTypeFilePath = "/sys/class/dmi/id/chassis_type"
	systemBus, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}
	hostnameObj := hostname1.NewHostname(systemBus)
	chassis, err := hostnameObj.Chassis().Get(0)
	if err != nil {
		return "", err
	}
	if chassis == "" || chassis == "desktop" {
		chassisNum, err := os.ReadFile(chassisTypeFilePath)
		if err != nil {
			logger.Warning(err)
			return "", err
		}
		switch string(bytes.TrimSpace(chassisNum)) {
		case "13":
			chassis = "all-in-one"
		}
	}
	return chassis, nil
}

func getGraphicsCardPciId() string {
	var pciId string
	subsystems := []string{"drm"}
	gudevClient := gudev.NewClient(subsystems)
	if gudevClient == nil {
		return ""
	}
	defer gudevClient.Unref()

	devices := gudevClient.QueryBySubsystem("drm")
	defer func() {
		for _, dev := range devices {
			dev.Unref()
		}
	}()

	for _, dev := range devices {
		name := dev.GetName()
		if strings.HasPrefix(name, "card") && strings.Contains(name, "-") {
			if dev.GetSysfsAttr("status") == "connected" {
				cardDevice := dev.GetParent()
				parentDevice := cardDevice.GetParent()
				pciId = parentDevice.GetProperty("PCI_ID")
				cardDevice.Unref()
				parentDevice.Unref()
				break
			}
		}
	}

	return pciId
}

func hasRate(rates []float64, rate float64) bool {
	for _, r := range rates {
		if math.Abs(r-rate) < 0.005 {
			return true
		}
	}

	return false
}

var regCardOutput = regexp.MustCompile(`^card\d+-.+`)

func getStdMonitorName(edid []byte) (string, error) {
	// /sys/class/drm/card0-HDMI-A-1
	fileInfos, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return "", err
	}
	for _, info := range fileInfos {
		name := info.Name()
		if regCardOutput.MatchString(name) {
			nameParts := strings.SplitN(name, "-", 2)
			if len(nameParts) == 2 {
				outputName := nameParts[1]
				sysEdid, _ := readSysDrmEdid(name)
				if outputName != "" && len(sysEdid) > 0 && edidEqual(edid, sysEdid) {
					return outputName, nil
				}
			}
		}
	}
	return "", errors.New("can not get std name")
}

func edidEqual(edid1, edid2 []byte) bool {
	// 比如 edid1  是从 x or wayland 获取的，
	// 而 edid2 是从 /sys/class/drm/XXX/edid 获取的。
	if len(edid1) == len(edid2) {
		return bytes.Equal(edid1, edid2)
	}
	if len(edid2) > 128 && len(edid1) == 128 {
		return bytes.Equal(edid1, edid2[:128])
	}
	return false
}

func readSysDrmEdid(name string) ([]byte, error) {
	content, err := os.ReadFile(filepath.Join("/sys/class/drm", name, "edid"))
	return content, err
}
