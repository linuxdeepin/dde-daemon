package network

// #cgo pkg-config: libudev
// #cgo CFLAGS: -fstack-protector-strong -D_FORTITY_SOURCE=1 -fPIC
// #include <stdlib.h>
// #include "utils_udev.h"
import "C"
import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/linuxdeepin/go-lib/utils"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-daemon/network/nm"
	networkmanager "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.networkmanager"
	"github.com/linuxdeepin/go-lib/gettext"
	. "github.com/linuxdeepin/go-lib/gettext"
)

func getSettingConnectionTimestamp(settings map[string]map[string]dbus.Variant) uint64 {
	a := settings["connection"]["timestamp"].Value()
	if a == nil {
		return 0
	}
	val, ok := a.(uint64)
	if !ok {
		logger.Warning("type of setting connection.timestamp is not uint64")
		return 0
	}
	return val
}

func getSettingConnectionAutoconnect(settings map[string]map[string]dbus.Variant) bool {
	a := settings["connection"]["autoconnect"].Value()
	if a == nil {
		return true
	}
	val, ok := a.(bool)
	if !ok {
		logger.Warning("type of setting connection.autoconnect is not bool")
		return false
	}
	return val
}

func getSettingConnectionType(settings map[string]map[string]dbus.Variant) string {
	return getSettingString(settings, "connection", "type")
}

func getSettingConnectionUuid(settings map[string]map[string]dbus.Variant) string {
	return getSettingString(settings, "connection", "uuid")
}
func getSettingConnectionId(settings map[string]map[string]dbus.Variant) string {
	return getSettingString(settings, "connection", "id")
}

func getSettingString(settings map[string]map[string]dbus.Variant, key1, key2 string) string {
	a := settings[key1][key2].Value()
	if a == nil {
		return ""
	}
	val, ok := a.(string)
	if !ok {
		logger.Warning("type of setting connection.autoconnect is not string")
		return ""
	}
	return val
}

func setDeviceAutoConnect(d networkmanager.Device, val bool) error {
	autoConnect, err := d.Device().Autoconnect().Get(0)
	if err != nil {
		return err
	}

	if autoConnect != val {
		err = d.Device().Autoconnect().Set(0, val)
		if err != nil {
			return err
		}
	}
	return nil
}

func setDeviceManaged(d networkmanager.Device, val bool) error {
	managed, err := d.Device().Managed().Get(0)
	if err != nil {
		return err
	}

	if managed != val {
		err = d.Device().Managed().Set(0, val)
		if err != nil {
			return err
		}
	}
	return nil
}

func restartIPWatchD() {
	go func() {
		err := exec.Command("systemctl", "restart", "ipwatchd.service").Run()
		if err != nil {
			logger.Warning(err)
		}
	}()
}

// ensure wired connection exist
func ensureWiredConnectionExist(devPath dbus.ObjectPath) (dbus.ObjectPath, error) {
	// get exist saved path
	connPath, err := devExistSavedPath(devPath)
	if err != nil {
		logger.Warningf("get dev exist saved connection failed, err: %v", err)
		return "", err
	}
	logger.Infof("make sure wired connection already exist path: %v", connPath)
	// create system bus
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warningf("get system bus failed, err: %v", err)
		return "", err
	}
	//// create new device
	nmDev, err := networkmanager.NewDevice(sysBus, devPath)
	if err != nil {
		logger.Warningf("create new device failed, err: %v", err)
		return "", err
	}
	// get udi
	udi, err := nmDev.Device().Udi().Get(0)
	if err != nil {
		logger.Warningf("get udi failed, err: %v", err)
	}
	// if connection not found. just create one
	if connPath == "" {
		// create connection id is device interface name
		var id string
		// check if device is usb
		if udevIsUsbDevice(udi) {
			// usb use device interface name as id
			ifc, err := nmDev.Device().Interface().Get(0)
			if err != nil {
				logger.Warningf("get interface failed, err: %v", err)
			}
			id = ifc
		} else {
			// or create connections
			id = getCreateConnectionName()
		}
		// get hardware address
		addr := "00:00:00:00:00:00"
		addr, _ = nmDev.Wired().PermHwAddress().Get(0)
		if addr == "" {
			addr, _ = nmDev.Wired().HwAddress().Get(0)
		}
		addr = strings.ToUpper(addr)
		uuid := strToUuid(addr)
		connData := newWiredConnectionData(id, uuid, devPath)
		// create connection settings
		conn := networkmanager.NewSettings(sysBus)
		path, err := conn.AddConnection(0, connData)
		if err != nil {
			logger.Warningf("add connection failed, err: %v", err)
			return "", err
		}
		connPath = path
	}
	logger.Infof("make sure wired connection result path: %v", connPath)
	return connPath, nil
}

// check dev exist saved path, find best connections
func devExistSavedPath(devPath dbus.ObjectPath) (dbus.ObjectPath, error) {
	// create system bus
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warningf("get system bus failed, err: %v", err)
		return "", err
	}
	// create device
	nmDev, err := networkmanager.NewDevice(sysBus, devPath)
	if err != nil {
		logger.Warningf("create new device failed, err: %v", err)
		return "", err
	}
	// check type, only wired device should make sure connection exist
	typ, err := nmDev.Device().DeviceType().Get(0)
	if err != nil {
		logger.Warningf("get device type failed, err: %v", err)
		return "", err
	}
	// check if type is ethernet
	if typ != nm.NM_DEVICE_TYPE_ETHERNET {
		return "", errors.New("device is not ethernet type")
	}
	// get all available device connections
	conSl, err := nmDev.Device().AvailableConnections().Get(0)
	if err != nil {
		logger.Warningf("get available connection failed, err: %v", err)
		return "", err
	}
	logger.Infof("device %v available con: %v", devPath, conSl)
	var path dbus.ObjectPath
	var timestamp uint64
	for _, conPath := range conSl {
		conn, err := networkmanager.NewConnectionSettings(sysBus, conPath)
		if err != nil {
			logger.Warningf("create connection failed, err: %v", err)
			continue
		}
		unsaved, err := conn.Unsaved().Get(0)
		if err != nil {
			logger.Warningf("get unsaved state failed, err: %v", err)
			continue
		}
		// find saved path
		if unsaved {
			continue
		}
		// get settings
		connData, err := conn.GetSettings(0)
		if err != nil {
			logger.Warningf("get settings failed, err: %v", err)
			continue
		}
		// get timestamp
		tmpTime := getSettingConnectionTimestamp(connData)
		if tmpTime > timestamp {
			path = conPath
			timestamp = tmpTime
		}
	}
	return path, nil
}

func getCreateConnectionName() string {
	sysBus, err := dbus.SystemBus()
	if err != nil {
		logger.Warningf("get sys bus failed, err: %v", err)
		return ""
	}
	settings := networkmanager.NewSettings(sysBus)
	connSl, err := settings.Connections().Get(0)
	if err != nil {
		logger.Warningf("get all connection failed, err: %v", err)
		return ""
	}
	var numSl []int
	for _, conPath := range connSl {
		conn, err := networkmanager.NewConnectionSettings(sysBus, conPath)
		if err != nil {
			logger.Warningf("create connection failed, err: %v", err)
			continue
		}
		connData, err := conn.GetSettings(0)
		if err != nil {
			logger.Warningf("get settings failed, err: %v", err)
			continue
		}
		typ := getSettingConnectionType(connData)
		if typ != "802-3-ethernet" {
			continue
		}
		id := getSettingConnectionId(connData)
		if id == "" {
			continue
		}
		if !strings.Contains(id, Tr("Wired Connection")) {
			continue
		}
		// trim left, " 1"
		numStr := strings.TrimLeft(id, Tr("Wired Connection"))
		// id is Wired Connection, add 0 to sl
		if numStr == "" {
			numSl = append(numSl, 0)
			continue
		}
		// trim " "
		numStr = strings.Trim(numStr, " ")
		// convert num string to int
		num, err := strconv.Atoi(numStr)
		// could fail here, such as id is "Wired Connection A/B"
		if err != nil {
			logger.Debugf("connection index convert unexpected happens %v", err)
			continue
		}
		// append
		numSl = append(numSl, num)
	}

	sort.Ints(numSl)
	logger.Debugf("num Sl is %v", numSl)
	var num int
	for index := 0; index < len(numSl); index++ {
		// already in the last one, such as numSl is [0,0], num is  0+1
		if index == len(numSl)-1 {
			num = numSl[index] + 1
			break
		}
		// check if equal next or larger
		if numSl[index] == numSl[index+1] || numSl[index]+1 == numSl[index+1] {
			continue
		}
		num = numSl[index] + 1
		break
	}
	var name string
	var locale string
	buf, err := ioutil.ReadFile("/etc/default/locale")
	if err != nil {
		logger.Warningf("read locale failed, err: %v", err)
	} else {
		logger.Infof("read local message is %v", string(buf))
		// create reader
		buffer := bytes.NewBuffer(buf)
		reader := bufio.NewReader(buffer)
		for {
			// read line, dont care about error or file end
			buf, _, err = reader.ReadLine()
			if err != nil {
				logger.Infof("read file failed, err: %v", err)
				break
			}
			// split string
			sl := strings.Split(string(buf), "=")
			logger.Infof("locale split is %v", sl)
			if len(sl) < 2 {
				continue
			}
			logger.Info("locale match 2 slice")
			// find lang
			if sl[0] == "LANG" {
				logger.Infof("locale is %v", sl[1])
				locale = sl[1]
			}
		}
	}
	logger.Infof("read lang from file success, lang: %v", locale)
	// make sure lang env exist
	if locale == "" {
		locale = os.Getenv("LANG")
	}
	SetLocale(gettext.LcAll, locale)
	if num == 0 {
		name = fmt.Sprintf(Tr("Wired Connection"))
	} else {
		name = fmt.Sprintf(Tr("Wired Connection %v"), num)
	}
	logger.Infof("create name is %v", name)
	return name
}

func udevIsUsbDevice(syspath string) bool {
	cSyspath := C.CString(syspath)
	defer C.free(unsafe.Pointer(cSyspath))

	ret := C.is_usb_device(cSyspath)
	return ret == 0
}

func strToUuid(str string) (uuid string) {
	md5, _ := utils.SumStrMd5(str)
	return doStrToUuid(md5)
}
func doStrToUuid(str string) (uuid string) {
	str = strings.ToLower(str)
	for i := 0; i < len(str); i++ {
		if (str[i] >= '0' && str[i] <= '9') ||
			(str[i] >= 'a' && str[i] <= 'f') {
			uuid = uuid + string(str[i])
		}
	}
	if len(uuid) < 32 {
		misslen := 32 - len(uuid)
		uuid = strings.Repeat("0", misslen) + uuid
	}
	uuid = fmt.Sprintf("%s-%s-%s-%s-%s", uuid[0:8], uuid[8:12], uuid[12:16], uuid[16:20], uuid[20:32])
	return
}
