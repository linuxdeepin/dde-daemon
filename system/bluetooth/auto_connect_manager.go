package bluetooth

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/godbus/dbus"
)

// 每个适配器最大 worker 数量，最大同时连接设备数。
const maxNumWorkerPerAdapter = 3

// 自动连接管理器
type autoConnectManager struct {
	mu                sync.Mutex
	devices           map[dbus.ObjectPath]*autoDeviceInfo // key 是 device path
	adapterWorkersMap map[dbus.ObjectPath]map[int]bool    // key 是 adapter path
	connectCb         func(adapter, device dbus.ObjectPath, wId int) error
}

func newAutoConnectManager() *autoConnectManager {
	return &autoConnectManager{
		devices:           make(map[dbus.ObjectPath]*autoDeviceInfo),
		adapterWorkersMap: make(map[dbus.ObjectPath]map[int]bool),
	}
}

// 自动连接设备信息
type autoDeviceInfo struct {
	alias         string
	retryDeadline time.Time // 重试截至时间
	adapter       dbus.ObjectPath
	device        dbus.ObjectPath
	wId           int
	priority      int // 优先级，越小越高
}

func (adi *autoDeviceInfo) String() string {
	return fmt.Sprintf("autoDeviceInfo{retryDeadline: %v, adapter: %q, alias: %q, device: %q, wId: %d, p: %d}",
		adi.retryDeadline, adi.adapter, adi.alias, adi.device, adi.wId, adi.priority)
}

func (adi *autoDeviceInfo) canRetry(now time.Time) bool {
	if now.Before(adi.retryDeadline) {
		return true
	}
	return false
}

// isTaken 设备是否正在被 worker 拿走，正在被自动连接。
func (adi *autoDeviceInfo) isTaken() bool {
	return adi.wId != 0
}

// getDevice 获取设备用于自动连接
func (acm *autoConnectManager) getDevice(workerId int, now time.Time) (result autoDeviceInfo) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	defer func() {
		if result.device != "" {
			logger.Debugf("[%d] getDevice success, adi: %v", workerId, &result)
		} else {
			logger.Debugf("[%d] getDevice failed", workerId)
		}
	}()

	var tmpDeviceInfos []*autoDeviceInfo
	for _, info := range acm.devices {
		if info.canRetry(now) && !info.isTaken() {
			tmpDeviceInfos = append(tmpDeviceInfos, info)
		}
	}

	if len(tmpDeviceInfos) == 0 {
		return autoDeviceInfo{}
	}

	sort.SliceStable(tmpDeviceInfos, func(i, j int) bool {
		return tmpDeviceInfos[i].priority < tmpDeviceInfos[j].priority
	})

	info := tmpDeviceInfos[0]
	info.wId = workerId
	if !info.canRetry(now) {
		delete(acm.devices, info.device)
	}

	return *info
}

// putDevice 归还设备
func (acm *autoConnectManager) putDevice(d autoDeviceInfo) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	v, ok := acm.devices[d.device]
	if !ok {
		// 设备已被删除
		return
	}
	if v.wId != d.wId {
		return
	}
	v.wId = 0
}

// removeDevice 移除设备，比如设备连接成功。
func (acm *autoConnectManager) removeDevice(d autoDeviceInfo) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	delete(acm.devices, d.device)
}

// addDevices 添加自动连接设备，devices 中所有 adapter 都要是 adapterPath。
func (acm *autoConnectManager) addDevices(adapterPath dbus.ObjectPath, devices []autoDeviceInfo, now time.Time) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	if _, ok := acm.adapterWorkersMap[adapterPath]; !ok {
		return
	}

	for _, d := range devices {
		if d.adapter != adapterPath {
			continue
		}
		currentD, ok := acm.devices[d.device]
		if ok {
			// 只保留 currentD 的 wId 字段，其余字段都被更新。
			d.wId = currentD.wId
		}
		dCopy := d
		acm.devices[d.device] = &dCopy
	}

	num := acm.evalNumWorkers(adapterPath, now)
	acm.startWorkers(adapterPath, num)
}

// evalNumWorkers 评估期待新启动的 worker 数量
func (acm *autoConnectManager) evalNumWorkers(adapterPath dbus.ObjectPath, now time.Time) int {
	// NOTE: 不要加锁
	num := 0
	for _, d := range acm.devices {
		if d.adapter == adapterPath && !d.isTaken() && d.canRetry(now) {
			num++
			if num == maxNumWorkerPerAdapter {
				break
			}
		}
	}
	idMap := acm.adapterWorkersMap[adapterPath]
	if idMap != nil {
		count := 0
		for _, exist := range idMap {
			if exist {
				count++
			}
		}
		limit := maxNumWorkerPerAdapter - count
		if num > limit {
			num = limit
		}
	}
	return num
}

// getId 分配 worker 的 id，并可以限制 worker 的数量。
// 返回 0 表示分配失败，大于 0 的表示成功。
func (acm *autoConnectManager) getId(adapterPath dbus.ObjectPath) int {
	// NOTE: 不要加锁
	idMap := acm.adapterWorkersMap[adapterPath]
	if idMap == nil {
		return 0
	}
	for i := 1; i <= maxNumWorkerPerAdapter; i++ {
		if !idMap[i] {
			idMap[i] = true
			return i
		}
	}
	return 0
}

// putId 归还 worker 使用的 id
func (acm *autoConnectManager) putId(adapterPath dbus.ObjectPath, id int) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	idMap := acm.adapterWorkersMap[adapterPath]
	if idMap == nil {
		return
	}
	idMap[id] = false
}

// startWorkers 为适配器启动 worker 们
func (acm *autoConnectManager) startWorkers(adapterPath dbus.ObjectPath, num int) {
	// NOTE: 不要加锁
	for i := 0; i < num; i++ {
		id := acm.getId(adapterPath)
		if id != 0 {
			w := newAutoConnectWorker(adapterPath, id, acm)
			logger.Debug("new worker", id)
			w.start()
		}
	}
}

// addAdapter 响应适配器被添加事件
func (acm *autoConnectManager) addAdapter(adapterPath dbus.ObjectPath) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	if _, ok := acm.adapterWorkersMap[adapterPath]; ok {
		return
	}
	acm.adapterWorkersMap[adapterPath] = make(map[int]bool)
}

// removeAdapter 响应适配器被删除事件
func (acm *autoConnectManager) removeAdapter(adapterPath dbus.ObjectPath) {
	acm.mu.Lock()
	defer acm.mu.Unlock()

	delete(acm.adapterWorkersMap, adapterPath)
	for devPath, info := range acm.devices {
		if info.adapter == adapterPath {
			delete(acm.devices, devPath)
		}
	}
}

// 自动连接的 worker，代表一个 go routine。
type autoConnectWorker struct {
	id      int
	adapter dbus.ObjectPath
	m       *autoConnectManager
}

func newAutoConnectWorker(adapterPath dbus.ObjectPath, id int, m *autoConnectManager) *autoConnectWorker {
	w := &autoConnectWorker{
		id:      id,
		adapter: adapterPath,
		m:       m,
	}
	return w
}

// start 开始工作
func (w *autoConnectWorker) start() {
	go func() {
		d := w.m.getDevice(w.id, time.Now())
		for {
			if d.device != "" {
				logger.Debugf("worker %d before connect", w.id)
				err := w.m.connectCb(d.adapter, d.device, w.id)
				if err != nil {
					logger.Warning(err)
					// 连接失败
					if err == errInvalidDevicePath {
						// 设备被移除，b.getDevice 失败
						w.m.removeDevice(d)
					} else {
						// 归还设备，以便再次尝试
						w.m.putDevice(d)
					}
				} else {
					// 连接成功，或被忽略
					w.m.removeDevice(d)
				}
				logger.Debugf("worker %d after connect", w.id)

				d = w.m.getDevice(w.id, time.Now())
				if d.device != "" {
					continue
				}
			}
			// 无工作可做，退出，回收资源。
			logger.Debugf("[%d] quit", w.id)
			w.m.putId(w.adapter, w.id)
			return
		}
	}()
}
