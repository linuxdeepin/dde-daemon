package ipwatchd

import (
	"fmt"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

const defaultCheckCount = 3

type task struct {
	ip     net.IP
	dev    *device
	hwAddr net.HardwareAddr
	packet []byte
	count  int
}

func newTask(ip net.IP, dev *device) *task {
	return &task{
		ip:    ip,
		dev:   dev,
		count: defaultCheckCount,
	}
}

// buildARPPackage 构建arp包
func (t *task) buildARPPackage() error {
	hwAddr, err := getDeviceHwAddr(t.dev.nmDevice, false)
	if err != nil {
		return fmt.Errorf("can't get device[%s] mac addr", t.dev.iface)
	}
	t.hwAddr = hwAddr

	// 以太网首部
	// EthernetType 0x0806 ARP
	eth := &layers.Ethernet{
		SrcMAC:       hwAddr,
		DstMAC:       net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		EthernetType: layers.EthernetTypeARP,
	}

	arp := &layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         layers.ARPRequest,
		SourceHwAddress:   hwAddr,
		SourceProtAddress: net.IPv4(0, 0, 0, 0).To4(),
		DstHwAddress:      net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		DstProtAddress:    t.ip,
	}

	logger.Debugf("build arp packet SourceHwAddress %v DstProtAddress %v", hwAddr, t.ip)

	buffer := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	gopacket.SerializeLayers(buffer, opts, eth, arp)

	t.packet = buffer.Bytes()
	return nil
}

// sendPacket 对指定的网络接口发送ARP报文
func (t *task) sendPacket() {
	handle, err := pcap.OpenLive(t.dev.iface, 128, false, 30*time.Second)
	if err != nil {
		logger.Warning("pcap open failed：", err)
		return
	}
	defer handle.Close()

	err = handle.WritePacketData(t.packet)
	if err != nil {
		logger.Warning("send packet failed:", err)
	}
}
