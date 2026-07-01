// SPDX-FileCopyrightText: 2022 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package ddcci

//#cgo CFLAGS: -W -Wall -fstack-protector-all -fPIC
//#cgo pkg-config: ddcutil
//#include <ddcutil_c_api.h>
//#include <ddcutil_types.h>
//#include <stdlib.h>
import "C"
import (
	"bytes"
	"encoding/base64"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"unsafe"

	"github.com/linuxdeepin/go-lib/utils"
)

type ddcci struct {
	listPointer *C.DDCA_Display_Info_List
	listMu      sync.Mutex

	displayHandleMap map[string]*displayHandle //index -> handle
	//displayMap       map[string]int         //edidBase64 --> ddcutil monitor index
}

type displayHandle struct {
	ddcci  *ddcci
	idx    int
	edid   [128]byte
	info   *C.DDCA_Display_Info
	handle C.DDCA_Display_Handle
	val    int
	state  int
}

func (d *ddcci) newDisplayHandle(idx int) *displayHandle {
	dh := &displayHandle{
		idx: idx,
	}
	dh.info = d.getDisplayInfoByIdx(idx)
	return dh
}

func (d *displayHandle) Open() error {
	if d.state > 0 {
		return nil
	}
	status := C.ddca_open_display2(d.info.dref, C.bool(true), &d.handle)
	if status != C.int(0) {
		logger.Info("doing ddca_open_display2 error")
		return fmt.Errorf("brightness,init: failed to open monitor=%d", status)
	}
	d.state = 1
	return nil
}

func (d *displayHandle) Close() error {
	if d.state == 0 {
		return nil
	}
	C.ddca_close_display(d.handle)
	d.state = 0
	return nil
}

func (d *displayHandle) initEdid() {
	edid := C.GoBytes(unsafe.Pointer(&d.info.edid_bytes), 128)
	copy(d.edid[:], edid)
}

func (d *displayHandle) getEdidBase64() string {
	return base64.StdEncoding.EncodeToString(d.edid[:])
}

func (d *displayHandle) setBrightness(percent int) error {
	var val C.DDCA_Non_Table_Vcp_Value
	status := C.ddca_get_non_table_vcp_value(d.handle, brightnessVCP, &val)
	if status != C.int(0) {
		return fmt.Errorf("brightness: failed to get brightness: %d", status)
	}

	if int(val.sl) == percent {
		return nil
	}
	status = C.ddca_set_non_table_vcp_value(d.handle, brightnessVCP, 0, C.uchar(percent))
	if status != C.int(0) {
		return fmt.Errorf("brightness: failed to set brightness via DDC/CI: %d", status)
	}
	return nil
}

const (
	brightnessVCP = 0x10
)

func newDDCCI() (*ddcci, error) {
	ddc := &ddcci{
		displayHandleMap: make(map[string]*displayHandle),
	}

	status := C.ddca_init2((*C.char)(unsafe.Pointer(nil)), C.DDCA_SYSLOG_NOTICE, C.DDCA_INIT_OPTIONS_CLIENT_OPENED_SYSLOG, (***C.char)(unsafe.Pointer(nil)))
	if status < C.int(0) {
		return nil, fmt.Errorf("brightness: Error ddcci init: %d", status)
	}

	err := ddc.RefreshDisplays()
	if err != nil {
		return nil, err
	}

	return ddc, nil
}

func (d *ddcci) freeList() {
	// handle 不再常驻打开，open/close 生命周期严格局限于 Get/Set 调用内部。
	// 注意：ddcutil 的显示锁是线程亲和的，ddca_open_display2 和 ddca_close_display
	// 必须在同一个 OS 线程内成对调用，否则跨线程解锁会触发 DDCRC_LOCKED。
	// 因此此处不能也不需要 Close 任何 handle——若曾有线程在 Get/Set 中 open 了
	// handle 却未 close，那已是对约束的违反；正确做法是保证 Get/Set 内部即用即关，
	// 让 freeList 只负责释放列表内存并清空 map。
	d.displayHandleMap = make(map[string]*displayHandle)

	if d.listPointer != nil {
		C.ddca_free_display_info_list(d.listPointer)
		d.listPointer = nil
	}
}

func (d *ddcci) RefreshDisplays() error {
	d.listMu.Lock()
	defer d.listMu.Unlock()

	d.freeList()

	// ddcutil 的显示器探测是「探测一次永久缓存」（ddc_ensure_displays_detected
	// 仅在 all_display_refs 为 NULL 时探测），ddca_get_display_info_list2 只是
	// 从缓存重建列表、不重新探测总线。因此热插拔后若不先丢弃缓存，新接入的显示器
	// 不会被识别（列表不变）。ddca_redetect_displays 会丢弃缓存并重新探测 I2C 总线。
	// 仅在 libddcutil 编译时启用 --enable-watch-displays 时生效，否则返回非 0，
	// 此时退化为使用缓存的 get_display_info_list2。
	if rc := C.ddca_redetect_displays(); rc != C.int(0) {
		logger.Warningf("brightness: ddca_redetect_displays failed: %d", int(rc))
	}

	status := C.ddca_get_display_info_list2(C.bool(true), &d.listPointer)
	if status != C.int(0) {
		return fmt.Errorf("brightness: failed to get display info list: %d", status)
	}
	logger.Debug("brightness: display-number=", int(d.listPointer.ct))
	for i := 0; i < int(d.listPointer.ct); i++ {
		err := d.initDisplay(i)
		if err != nil {
			logger.Warning(err)
		}
	}
	return nil
}

func (d *ddcci) initDisplay(idx int) error {
	logger.Debug("brightness: initDisplay enter, index=", idx)
	dh := d.newDisplayHandle(idx)
	// 方案A：不在初始化期 open，仅缓存 dref/edid；Get/Set 时即用即开，
	// 避免 ddcutil 显示锁跨 goroutine 长生命周期持有导致 DDCRC_LOCKED。
	dh.initEdid()
	d.displayHandleMap[dh.getEdidBase64()] = dh
	return nil
}

func (d *ddcci) SupportBrightness(edidBase64 string) bool {
	d.listMu.Lock()
	defer d.listMu.Unlock()

	index, find := d.findMonitorIndex(edidBase64)
	logger.Debug("brightness: SupportBrightness", index, find, edidBase64)
	return find
}

func (d *ddcci) GetBrightness(edidBase64 string) (brightness int, err error) {
	d.listMu.Lock()
	defer d.listMu.Unlock()

	// 固定到当前 OS 线程，保证 ddca_open_display2 与 ddca_close_display 同线程，
	// ddcutil 显示锁不再跨越 goroutine 边界 → 消除 DDCRC_LOCKED。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	handle, ok := d.displayHandleMap[edidBase64]
	if !ok || handle == nil {
		//ignore any bytes
		idx, find := d.findMonitorIndex(edidBase64)
		if find {
			handle = d.getDisplayHandleByIdx(idx)
			logger.Info("get display handle by index:", idx)
		}

		if !find || handle == nil {
			err = fmt.Errorf("brightness: failed to find monitor")
			return
		}
	}

	// 即用即开：本次调用内 open，defer close，锁生命周期局限于此线程上的这一次调用。
	if err = handle.Open(); err != nil {
		return
	}
	defer handle.Close()

	var val C.DDCA_Non_Table_Vcp_Value
	status := C.ddca_get_non_table_vcp_value(handle.handle, brightnessVCP, &val)
	if status != C.int(0) {
		err = fmt.Errorf("brightness: failed to get brightness: %d", status)
		return
	}

	brightness = int(val.sl)
	return
}

func (d *ddcci) SetBrightness(edidBase64 string, percent int) error {
	d.listMu.Lock()
	defer d.listMu.Unlock()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 开启结果验证，防止返回设置成功，但实际上没有生效的情况
	// 此方法仅对当前线程生效
	C.ddca_enable_verify(false)
	dh, ok := d.displayHandleMap[edidBase64]
	if !ok || dh == nil {
		//ignore any bytes
		idx, find := d.findMonitorIndex(edidBase64)
		if find {
			dh = d.getDisplayHandleByIdx(idx)
			logger.Info("get display handle by index---------", idx)
		} else {
			return fmt.Errorf("brightness: failed to find monitor")
		}
	}

	// 即用即开：同线程内 open/close，避免跨线程解锁 DDCRC_LOCKED。
	if err := dh.Open(); err != nil {
		return err
	}
	defer dh.Close()
	return dh.setBrightness(percent)
}

func (d *ddcci) getDisplayHandleByIdx(idx int) *displayHandle {
	for _, handle := range d.displayHandleMap {
		if handle.idx == idx {
			logger.Debugf("getDisplayHandleByIdx:=== %d\n", idx)
			return handle
		}
	}
	return nil
}

func zeroEdidMonitorName(byBuf []byte) bool {
	if len(byBuf) < 128 {
		logger.Infof("zeroEdidMonitorName: len error, %d\n", len(byBuf))
		return false
	}
	hasModify := false
	//解释几个数字: edid长度128,头部长54，扩展部分18字节一段，54 + 18 * 4 = 126
	for i := 0; i < 4; i++ {
		offset := 0x36 + i*18
		//fmt.Printf("zeroEdidMonitorName: idx=%d, %x\n", i, byBuf[offset: offset + 5])
		if reflect.DeepEqual(byBuf[offset:offset+3], []byte{0, 0, 0}) {
			if byBuf[offset+3] == 0xfc {
				//logger.Debugf("brightness: zeroEdidMonitorName, block index=%d\n", i)
				hasModify = true
				zeroStart := offset + 5
				zeroLen := 13
				for j := 0; j < zeroLen; j++ {
					byBuf[zeroStart+j] = 0
				}
			}
		}
	}
	return hasModify
}

func (d *ddcci) findMonitorIndex(edidBase64 string) (int, bool) {
	logger.Debug("edidBase64:", edidBase64)
	dh, find := d.displayHandleMap[edidBase64]
	if find {
		logger.Info("find df for edidBase64:", edidBase64)
		return dh.idx, find
	}

	idx := 0
	bufInput, _ := base64.StdEncoding.DecodeString(edidBase64)
	zeroEdidMonitorName(bufInput)
	for edidIter, idxIter := range d.displayHandleMap {
		//logger.Debugf("edid:", edidIter)
		bufIter, _ := base64.StdEncoding.DecodeString(edidIter)
		zeroEdidMonitorName(bufIter)
		// ddcci只存储128个字节，所以之比较前128个字节
		if bytes.HasPrefix(bufInput, bufIter) {
			idx = idxIter.idx
			find = true
			//logger.Debugf("brightness: DeepEqual\n")
		}
	}
	return idx, find
}

func (d *ddcci) getDisplayInfoByIdx(idx int) *C.DDCA_Display_Info {
	start := unsafe.Pointer(uintptr(unsafe.Pointer(d.listPointer)) + uintptr(C.sizeof_DDCA_Display_Info_List))
	size := uintptr(C.sizeof_DDCA_Display_Info)

	return (*C.DDCA_Display_Info)(unsafe.Pointer(uintptr(start) + size*uintptr(idx)))
}

func getEDIDChecksum(edid []byte) string {
	if len(edid) < 128 {
		return ""
	}

	id, _ := utils.SumStrMd5(string(edid[:128]))
	return id
}
