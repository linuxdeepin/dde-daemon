/*
 * Copyright (C) 2014 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     jouyouyun <jouyouwen717@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package bluetooth

import (
	"encoding/json"
	"sync"

	"github.com/godbus/dbus"
)

type AdapterInfo struct {
	Address             string
	Path                dbus.ObjectPath
	Name                string
	Alias               string
	Powered             bool
	Discovering         bool
	Discoverable        bool
	DiscoverableTimeout uint32
}

func unmarshalAdapterInfo(data string) (*AdapterInfo, error) {
	var adapter AdapterInfo
	err := json.Unmarshal([]byte(data), &adapter)
	if err != nil {
		return nil, err
	}
	return &adapter, nil
}

type AdapterInfos struct {
	mu    sync.Mutex
	infos []AdapterInfo
}

func (a *AdapterInfos) getAdapter(path dbus.ObjectPath) (int, *AdapterInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.getAdapterNoLock(path)
}

func (a *AdapterInfos) getAdapterNoLock(path dbus.ObjectPath) (int, *AdapterInfo) {
	for idx, info := range a.infos {
		if info.Path == path {
			return idx, &info
		}
	}
	return -1, nil
}

func (a *AdapterInfos) addOrUpdateAdapter(adapterInfo *AdapterInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()

	idx, _ := a.getAdapterNoLock(adapterInfo.Path)
	if idx != -1 {
		// 更新
		a.infos[idx] = *adapterInfo
		return
	}
	a.infos = append(a.infos, *adapterInfo)
}

func (a *AdapterInfos) removeAdapter(path dbus.ObjectPath) {
	a.mu.Lock()
	defer a.mu.Unlock()

	idx, _ := a.getAdapterNoLock(path)
	if idx == -1 {
		return
	}
	a.infos = append(a.infos[:idx], a.infos[idx+1:]...)
}

func (a *AdapterInfos) toJSON() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return marshalJSON(a.infos)
}

func (a *AdapterInfos) clear() {
	a.mu.Lock()
	a.infos = nil
	a.mu.Unlock()
}
