package ipwatchd

import (
	"net"
	"strings"
	"sync"
	"time"

	dbus "github.com/godbus/dbus"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/linuxdeepin/dde-daemon/loader"
	nmdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.networkmanager"
	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-lib/dbusutil/proxy"
	"github.com/linuxdeepin/go-lib/log"
)

type Module struct {
	*loader.ModuleBase
	ipwatchd *IPWatchD
}

func (m *Module) GetDependencies() []string {
	return nil
}

func (m *Module) Start() error {
	if m.ipwatchd != nil {
		return nil
	}
	logger.Debug("start ipwatchd")
	m.ipwatchd = newIPWatchD()

	service := loader.GetService()
	m.ipwatchd.service = service

	err := m.ipwatchd.init()
	if err != nil {
		return err
	}

	serverObj, err := service.NewServerObject(dbusPath, m.ipwatchd)
	if err != nil {
		return err
	}

	err = serverObj.Export()
	if err != nil {
		return err
	}

	err = service.RequestName(dbusServiceName)
	if err != nil {
		return err
	}

	return nil
}

func (m *Module) Stop() error {
	m.ipwatchd.stop()
	return nil
}

var logger = log.NewLogger("daemon/ipwatchd")

func newModule(logger *log.Logger) *Module {
	m := &Module{}
	m.ModuleBase = loader.NewModuleBase("ipwatchd", m, logger)
	return m
}

func init() {
	loader.Register(newModule(logger))
}

//go:generate dbusutil-gen em -type IPWatchD

type IPWatchD struct {
	service    *dbusutil.Service
	devices    map[dbus.ObjectPath]*device
	devicesMu  sync.Mutex
	nmManager  nmdbus.Manager
	nmSettings nmdbus.Settings
	sigLoop    *dbusutil.SignalLoop
	taskMu     sync.Mutex
	taskMap    map[string]*task
	quit       chan bool

	// nolint
	signals *struct {
		IPConflict struct {
			ip   string
			smac string
			dmac string
		}
	}
}

func (i *IPWatchD) init() error {
	sysBus := i.getSysBus()
	i.sigLoop = dbusutil.NewSignalLoop(sysBus, 10)
	i.sigLoop.Start()
	i.nmManager = nmdbus.NewManager(sysBus)
	i.nmSettings = nmdbus.NewSettings(sysBus)
	i.connectSignal()
	i.addDevicesWithRetry()
	go i.loop()

	return nil
}

type device struct {
	iface    string
	nmDevice nmdbus.Device
	type0    uint32
}

func (i *IPWatchD) getSysBus() *dbus.Conn {
	return i.service.Conn()
}

func (i *IPWatchD) stop() {
	i.quit <- true
}

func (n *IPWatchD) connectSignal() {
	var err error
	n.nmManager.InitSignalExt(n.sigLoop, true)
	_, err = n.nmManager.ConnectDeviceAdded(func(devPath dbus.ObjectPath) {
		logger.Debug("device added", devPath)
		err := n.addDevice(devPath)
		if err != nil {
			logger.Warning(err)
		}

	})
	if err != nil {
		logger.Warning(err)
	}

	_, err = n.nmManager.ConnectDeviceRemoved(func(devPath dbus.ObjectPath) {
		logger.Debug("device removed", devPath)
		n.removeDevice(devPath)
	})

	if err != nil {
		logger.Warning(err)
	}
}

// get devices may failed because dde-system-daemon and NetworkManager are started very nearly,
// need call method 5 times.
func (i *IPWatchD) addDevicesWithRetry() {
	// try get all devices 5 times
	for n := 0; n < 5; n++ {
		devicePaths, err := i.nmManager.GetDevices(0)
		if err != nil {
			logger.Warning(err)
			// sleep for 1 seconds, and retry get devices
			time.Sleep(1 * time.Second)
		} else {
			i.addAndCheckDevices(devicePaths)
			// if success, break
			break
		}
	}
}

// add and check devices
func (i *IPWatchD) addAndCheckDevices(devicePaths []dbus.ObjectPath) {
	// add device
	for _, devPath := range devicePaths {
		err := i.addDevice(devPath)
		if err != nil {
			logger.Warning(err)
			continue
		}
	}
}

func (i *IPWatchD) addDevice(devPath dbus.ObjectPath) error {
	i.devicesMu.Lock()
	_, ok := i.devices[devPath]
	if ok {
		i.devicesMu.Unlock()
		return nil
	}

	dev, err := nmdbus.NewDevice(i.getSysBus(), devPath)
	if err != nil {
		i.devicesMu.Unlock()
		return err
	}
	d := dev.Device()
	iface, err := d.Interface().Get(0)
	if err != nil {
		i.devicesMu.Unlock()
		return err
	}

	deviceType, err := d.DeviceType().Get(0)
	if err != nil {
		logger.Warningf("get device %s type failed: %v", dev.Path_(), err)
	}

	newDev := &device{
		iface:    iface,
		nmDevice: dev,
		type0:    deviceType,
	}
	i.devices[devPath] = newDev

	i.devicesMu.Unlock()

	dev.InitSignalExt(i.sigLoop, true)
	err = d.Interface().ConnectChanged(func(hasValue bool, value string) {
		if !hasValue {
			return
		}
		logger.Debugf("recv interface changed signal, old iface: %s, new iface: %s", iface, value)
		newDev.iface = value
	})
	if err != nil {
		logger.Warning(err)
	}

	return nil
}

func (i *IPWatchD) removeDevice(devPath dbus.ObjectPath) {
	i.devicesMu.Lock()
	defer i.devicesMu.Unlock()

	d, ok := i.devices[devPath]
	if !ok {
		return
	}
	d.nmDevice.RemoveHandler(proxy.RemoveAllHandlers)
	delete(i.devices, devPath)
}

func newIPWatchD() *IPWatchD {
	return &IPWatchD{
		quit:    make(chan bool, 1),
		devices: make(map[dbus.ObjectPath]*device),
		taskMap: make(map[string]*task),
	}
}

func (i *IPWatchD) getDeviceByIface(iface string) *device {
	for _, value := range i.devices {
		if value.iface == iface {
			return value
		}
	}
	return nil
}

func (i *IPWatchD) findIPConflictMac(ip net.IP, hw net.HardwareAddr) string {
	i.devicesMu.Lock()
	defer i.devicesMu.Unlock()

	mac := ""
	for _, v := range i.devices {
		hwAddr, err := getDeviceHwAddr(v.nmDevice, false)
		if err != nil {
			// 虚拟设备无法获取mac, 屏蔽掉错误打印
			// logger.Debugf("can't get device(%s) mac addr: %v", v.iface, err)
			continue
		}
		// local mac
		if hwAddr.String() == hw.String() {
			return ""
		}
		// 已经存在同一个ip, 不同的mac
		if len(mac) != 0 {
			continue
		}

		ip4Path, err := v.nmDevice.Device().Ip4Config().Get(0)
		if err != nil {
			logger.Warning(err)
			continue
		}
		if !IsNmObjectPathValid(ip4Path) {
			continue
		}
		nmIp4Config, err := nmdbus.NewIP4Config(i.getSysBus(), ip4Path)
		if err != nil {
			logger.Warning(err)
			continue
		}
		addrs, err := nmIp4Config.Addresses().Get(0)
		if err != nil {
			logger.Warning(err)
			continue
		}
		ip4list := GetIP4NetList(addrs)
		for _, v := range ip4list {
			if v.IP != nil {
				if v.IP.Equal(ip) { // ip conflict ?
					mac = hwAddr.String()
				}
			}
		}
	}

	return mac
}

func (i *IPWatchD) findDevice(pathOrIface string) *device {
	i.devicesMu.Lock()
	defer i.devicesMu.Unlock()

	if strings.HasPrefix(pathOrIface, "/org/freedesktop/NetworkManager") {
		return i.devices[dbus.ObjectPath(pathOrIface)]
	}
	return i.getDeviceByIface(pathOrIface)
}

// addTask 增加一个检测任务
func (i *IPWatchD) addTask(ip net.IP, dev *device) error {
	i.taskMu.Lock()
	defer i.taskMu.Unlock()

	if v, ok := i.taskMap[ip.String()]; ok && v.dev.iface == dev.iface { // 已经存在
		v.count = defaultCheckCount
	} else {
		task := newTask(ip, dev)
		err := task.buildARPPackage()
		if err != nil {
			return err
		}
		i.taskMap[ip.String()] = task
	}

	logger.Debugf("add a task: %s <--> %s", ip.String(), dev.iface)

	return nil
}

// removeTask 移除一个检测任务
func (i *IPWatchD) removeTask(ip net.IP) {
	i.taskMu.Lock()
	defer i.taskMu.Unlock()

	delete(i.taskMap, ip.String())
}

// findTask 找到一个检测任务
func (i *IPWatchD) findTask(ip net.IP) *task {
	i.taskMu.Lock()
	defer i.taskMu.Unlock()

	return i.taskMap[ip.String()]
}

// work 检测任务
func (i *IPWatchD) work() {
	i.taskMu.Lock()
	defer i.taskMu.Unlock()

	for _, v := range i.taskMap {
		if v.count > 0 {
			v.count--
			v.sendPacket()
		} else {
			logger.Debugf("task over: %s <--> %s conflict: no", v.ip.String(), v.dev.iface)
			delete(i.taskMap, v.ip.String())
			i.service.Emit(i, "IPConflict", v.ip.String(), v.hwAddr.String(), "")
		}
	}
}

// handle 处理arp包
func (i *IPWatchD) handle(packet gopacket.Packet) {
	arp, ok := packet.Layer(layers.LayerTypeARP).(*layers.ARP)
	if !ok {
		return
	}

	hwAddr := net.HardwareAddr(arp.SourceHwAddress)
	ip := net.IP(arp.SourceProtAddress).To4()
	mac := i.findIPConflictMac(ip, hwAddr)
	if len(mac) != 0 { // 本地ip冲突
		logger.Debugf("found local ip conflict: %s - %s - %s",
			ip.String(), mac, hwAddr.String())
		i.removeTask(ip)
		err := i.service.Emit(i, "IPConflict", ip.String(), mac, hwAddr.String())
		if err != nil {
			logger.Warning(err)
		}
	} else { // 非本地ip冲突
		task := i.findTask(ip)
		if task != nil {
			logger.Debugf("found global ip conflict: %s - %s - %s",
				ip.String(), task.hwAddr.String(), hwAddr.String())
			i.removeTask(ip)
			err := i.service.Emit(i, "IPConflict",
				ip.String(), task.hwAddr.String(), hwAddr.String())
			if err != nil {
				logger.Warning(err)
			}
		}
	}
}

// loop 监听网卡arp包
func (i *IPWatchD) loop() {
	handle, err := pcap.OpenLive("any", 128, false, 30*time.Millisecond)
	if err != nil {
		logger.Warning("pcap open failed:", err)
		return
	}
	defer handle.Close()
	handle.SetBPFFilter("arp")
	ps := gopacket.NewPacketSource(handle, handle.LinkType())

	ticker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case packet := <-ps.Packets():
			i.handle(packet)

		case _, ok := <-ticker.C:
			if !ok {
				logger.Error("Invalid ticker event")
				return
			}
			i.work()

		case <-i.quit:
			ticker.Stop()
			return
		}
	}
}

// findActiveConnectionByIP 找出与所给ip最接近的激活连接
func (i *IPWatchD) findActiveConnectionByIP(ip net.IP) nmdbus.ActiveConnection {
	conns, err := i.nmManager.ActiveConnections().Get(0)
	if err != nil {
		logger.Warning(err)
		return nil
	}

	var ret nmdbus.ActiveConnection
	sysBus := i.getSysBus()
	for _, conn := range conns {
		nmAConn, err := nmdbus.NewActiveConnection(sysBus, conn)
		if err != nil {
			logger.Warning(err)
			continue
		}
		ip4Path, err := nmAConn.Ip4Config().Get(0)
		if err != nil {
			logger.Warning(err)
			continue
		}
		if !IsNmObjectPathValid(ip4Path) {
			continue
		}
		nmIp4Config, err := nmdbus.NewIP4Config(sysBus, ip4Path)
		if err != nil {
			logger.Warning(err)
			continue
		}
		addrs, err := nmIp4Config.Addresses().Get(0)
		if err != nil {
			logger.Warning(err)
			continue
		}
		ip4list := GetIP4NetList(addrs)
		for _, v := range ip4list {
			if v.IP != nil {
				if v.IP.Equal(ip) { // 完美匹配
					return nmAConn
				}
				a := v.IP.Mask(v.Mask)
				b := ip.Mask(v.Mask)
				if a.Equal(b) { // Mask后一致
					ret = nmAConn
				}
			}
		}
	}

	return ret
}

// findDeviceByIP 根据ip找到对应的网络接口设备
func (i *IPWatchD) findDeviceByIP(ip net.IP) *device {
	conn := i.findActiveConnectionByIP(ip)
	if conn == nil {
		logger.Warning("unable to find a suitable connection")
		return nil
	}

	devPaths, err := conn.Devices().Get(0)
	if err != nil {
		logger.Warning(err)
		return nil
	}

	for _, path := range devPaths {
		if dev, ok := i.devices[path]; ok {
			return dev
		}
	}

	return nil
}
