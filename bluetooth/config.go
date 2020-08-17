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
	"sort"
	"strings"
	"time"

	"pkg.deepin.io/lib/utils"
)

type config struct {
	core utils.Config

	Adapters map[string]*adapterConfig // use adapter hardware address as key
	Devices  map[string]*deviceConfig  // use adapter address/device address as key

	Discoverable bool `json:"discoverable"`
}

type adapterConfig struct {
	Powered bool
}

type deviceConfig struct {
	// add icon info to mark device type
	Icon string
	// device connect status
	Connected bool
	// record latest time to do compare with other devices
	LatestTime int64
}

// add address message
type deviceConfigWithAddress struct {
	Icon       string
	Connected  bool
	LatestTime int64
	Address    string
}

func newConfig() (c *config) {
	c = &config{}
	c.core.SetConfigName("bluetooth")
	logger.Info("load bluetooth config file:", c.core.GetConfigFile())
	c.Adapters = make(map[string]*adapterConfig)
	c.Devices = make(map[string]*deviceConfig)
	c.Discoverable = true
	c.load()
	return
}

func (c *config) load() {
	c.core.Load(c)
}

func (c *config) save() {
	c.core.Save(c)
}

func newAdapterConfig() (ac *adapterConfig) {
	ac = &adapterConfig{Powered: true}
	return
}

func (c *config) clearSpareConfig(b *Bluetooth) {
	var adapterAddresses []string
	// key is adapter address
	var adapterDevicesMap = make(map[string][]*device)

	b.adaptersLock.Lock()
	for _, adapter := range b.adapters {
		adapterAddresses = append(adapterAddresses, adapter.address)
	}
	b.adaptersLock.Unlock()

	for _, adapterAddr := range adapterAddresses {
		adapterDevicesMap[adapterAddr] = b.getAdapterDevices(adapterAddr)
	}

	c.core.Lock()
	// remove spare adapters
	for addr := range c.Adapters {
		if !isStringInArray(addr, adapterAddresses) {
			delete(c.Adapters, addr)
		}
	}

	// remove spare devices
	for addr := range c.Devices {
		addrParts := strings.SplitN(addr, "/", 2)
		if len(addrParts) != 2 {
			delete(c.Devices, addr)
			continue
		}
		adapterAddr := addrParts[0]
		deviceAddr := addrParts[1]

		devices := adapterDevicesMap[adapterAddr]
		var foundDevice bool
		for _, device := range devices {
			if device.Address == deviceAddr {
				foundDevice = true
				break
			}
		}

		if !foundDevice {
			delete(c.Devices, addr)
			continue
		}
	}

	c.core.Unlock()
}

func (c *config) addAdapterConfig(address string) {
	if c.isAdapterConfigExists(address) {
		return
	}

	c.core.Lock()
	c.Adapters[address] = newAdapterConfig()
	c.core.Unlock()
	c.save()
}

func (c *config) removeAdapterConfig(address string) {
	c.core.Lock()
	delete(c.Adapters, address)
	c.core.Unlock()
	c.save()
}

func (c *config) getAdapterConfig(address string) (ac *adapterConfig, ok bool) {
	c.core.Lock()
	defer c.core.Unlock()
	ac, ok = c.Adapters[address]
	return
}

func (c *config) isAdapterConfigExists(address string) (ok bool) {
	c.core.Lock()
	defer c.core.Unlock()
	_, ok = c.Adapters[address]
	return
}

func (c *config) getAdapterConfigPowered(address string) (powered bool) {
	c.core.Lock()
	defer c.core.Unlock()
	if ac, ok := c.Adapters[address]; ok {
		return ac.Powered
	}
	return false
}

func (c *config) setAdapterConfigPowered(address string, powered bool) {
	c.core.Lock()
	if ac, ok := c.Adapters[address]; ok {
		ac.Powered = powered
	}
	c.core.Unlock()
	c.save()
	return
}

func newDeviceConfig() (ac *deviceConfig) {
	ac = &deviceConfig{Connected: false}
	return
}

func (c *config) isDeviceConfigExist(address string) (ok bool) {
	c.core.Lock()
	defer c.core.Unlock()
	_, ok = c.Devices[address]
	return
}

func (c *config) addDeviceConfig(address string) {
	if c.isDeviceConfigExist(address) {
		return
	}
	c.core.Lock()
	c.Devices[address] = newDeviceConfig()
	c.core.Unlock()
	c.save()
}

func (c *config) getDeviceConfig(address string) (dc *deviceConfig, ok bool) {
	c.core.Lock()
	defer c.core.Unlock()
	dc, ok = c.Devices[address]
	return
}

func (c *config) removeDeviceConfig(address string) {
	c.core.Lock()
	delete(c.Devices, address)
	c.core.Unlock()
	c.save()
}

func (c *config) getDeviceConfigConnected(address string) (connected bool) {
	dc, ok := c.getDeviceConfig(address)
	if !ok {
		return
	}

	c.core.Lock()
	defer c.core.Unlock()
	return dc.Connected
}

func (c *config) setDeviceConfigConnected(device *device, connected bool) {
	dc, ok := c.getDeviceConfig(device.getAddress())
	if !ok {
		return
	}

	c.core.Lock()
	dc.Connected = connected
	dc.Icon = device.Icon
	if connected {
		dc.LatestTime = time.Now().Unix()
	}
	c.core.Unlock()
	c.save()
	return
}

// select latest devices from devAddressMap, each type only contain one device
func (c *config) filterDemandedTypeDevices(devAddressMap map[string]*device) map[string][]*device {
	// prepare map to contain different type device, each device is distributed one nil element
	// to fill suitable device
	typeDeviceConfigMap := make(map[string][]*deviceConfigWithAddress, len(DeviceTypes))
	for _, value := range DeviceTypes {
		typeDeviceConfigMap[value] = nil
	}

	// find latest devices to fill ordered type device
	for _, deviceUnit := range devAddressMap {
		// if device's address is empty, ignore this device
		if deviceUnit.getAddress() == "" {
			continue
		}

		// check if device type is included in required types
		if _, ok := typeDeviceConfigMap[deviceUnit.Icon]; !ok {
			continue
		}

		// get device info from config devices according to address
		devConfig := c.Devices[deviceUnit.getAddress()]
		if devConfig == nil {
			continue
		}

		// only paired but not connected devices allowed to auto connect
		if !deviceUnit.Paired || deviceUnit.connected {
			continue
		}

		
		typeDeviceConfigMap[deviceUnit.Icon] = append(typeDeviceConfigMap[deviceUnit.Icon], &deviceConfigWithAddress{
			Icon:       devConfig.Icon,
			Connected:  devConfig.Connected,
			LatestTime: devConfig.LatestTime,
			Address:    deviceUnit.getAddress(),
		})
	}
	//range typeDeviceConfigMap to sort DeviceConfig by LatestTime from lastest to farthest
	for _, DeviceConfigList := range typeDeviceConfigMap {
		sort.Sort(deviceConfigWithAddressSlice(DeviceConfigList))
	}

	// add all filtered devices to device list
	var deviceList []*device
	typeDeviceListMap := make(map[string][]*device, len(DeviceTypes))
	for _, value := range DeviceTypes {
		typeDeviceListMap[value] = nil
	}

	for deviceType, deviceConfigs := range typeDeviceConfigMap {
		//empty deviceList for new deviceType
		deviceList = nil
		// check if type devices is nil
		if deviceConfigs == nil {
			continue
		}
		// select device from devAddressMap, add device to device list
		for _, devCfg := range deviceConfigs {
			if devCfg == nil {
				continue
			}
			deviceList = append(deviceList, devAddressMap[devCfg.Address])
		}
		typeDeviceListMap[deviceType] = deviceList
	}
	return typeDeviceListMap
}
