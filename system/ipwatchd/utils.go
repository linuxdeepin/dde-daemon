package ipwatchd

import (
	"fmt"
	"net"

	dbus "github.com/godbus/dbus"
	"github.com/linuxdeepin/dde-daemon/network/nm"
	nmdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.networkmanager"
)

func getDeviceHwAddr(dev nmdbus.Device, perm bool) (net.HardwareAddr, error) {
	mac := "00:00:00:00:00:00"

	devType, _ := dev.Device().DeviceType().Get(0)
	switch devType {
	case nm.NM_DEVICE_TYPE_ETHERNET:
		devWired := dev.Wired()
		mac = ""
		if perm {
			mac, _ = devWired.PermHwAddress().Get(0)
		}
		if mac == "" {
			// may get PermHwAddress failed under NetworkManager 1.4.1
			mac, _ = devWired.HwAddress().Get(0)
		}
	case nm.NM_DEVICE_TYPE_WIFI:
		devWireless := dev.Wireless()
		mac = ""
		if perm {
			mac, _ = devWireless.PermHwAddress().Get(0)
		}
		if len(mac) == 0 {
			mac, _ = devWireless.HwAddress().Get(0)
		}
	case nm.NM_DEVICE_TYPE_BT:
		devBluetooth := dev.Bluetooth()
		mac, _ = devBluetooth.HwAddress().Get(0)
	case nm.NM_DEVICE_TYPE_OLPC_MESH:
		devOlpcMesh := dev.OlpcMesh()
		mac, _ = devOlpcMesh.HwAddress().Get(0)
	case nm.NM_DEVICE_TYPE_WIMAX:
		devWiMax := dev.WiMax()
		mac, _ = devWiMax.HwAddress().Get(0)
	case nm.NM_DEVICE_TYPE_INFINIBAND:
		devInfiniband := dev.Infiniband()
		mac, _ = devInfiniband.HwAddress().Get(0)
	case nm.NM_DEVICE_TYPE_BOND:
		devBond := dev.Bond()
		mac, _ = devBond.HwAddress().Get(0)
	case nm.NM_DEVICE_TYPE_BRIDGE:
		devBridge := dev.Bridge()
		mac, _ = devBridge.HwAddress().Get(0)
	case nm.NM_DEVICE_TYPE_VLAN:
		devVlan := dev.Vlan()
		mac, _ = devVlan.HwAddress().Get(0)
	case nm.NM_DEVICE_TYPE_GENERIC:
		devGeneric := dev.Generic()
		mac, _ = devGeneric.HwAddress().Get(0)
	case nm.NM_DEVICE_TYPE_TEAM:
		devTeam := dev.Team()
		mac, _ = devTeam.HwAddress().Get(0)
	case nm.NM_DEVICE_TYPE_MODEM, nm.NM_DEVICE_TYPE_ADSL, nm.NM_DEVICE_TYPE_TUN, nm.NM_DEVICE_TYPE_IP_TUNNEL, nm.NM_DEVICE_TYPE_MACVLAN, nm.NM_DEVICE_TYPE_VXLAN, nm.NM_DEVICE_TYPE_VETH:
		// there is no hardware address for such devices
		return nil, fmt.Errorf("there is no hardware address for device modem, adsl, tun")
	default:
		return nil, fmt.Errorf("unknown device type %d", devType)
	}

	return net.ParseMAC(mac)
}

func IsNmObjectPathValid(path dbus.ObjectPath) bool {
	str := string(path)
	return len(str) > 0 && str != "/"
}

func GetIP4NetList(data [][]uint32) []*net.IPNet {
	ret := []*net.IPNet{}
	for _, d := range data {
		if len(d) != 3 {
			continue
		}

		v := &net.IPNet{
			IP:   net.IPv4(byte(d[0]), byte(d[0]>>8), byte(d[0]>>16), byte(d[0]>>24)),
			Mask: net.CIDRMask(int(d[1]), 32),
		}

		ret = append(ret, v)
	}

	return ret
}
