package main

import (
	"encoding/json"
	"fmt"
)

const (
	jsonNull        = `null`
	jsonEmptyString = `""`
	jsonEmptyArray  = `[]`
)

// dbus.Variant.Value() -> realdata -> wrapped data(if need) -> json string
func keyValueToJSON(v interface{}, t ktype) (jsonStr string, err error) {
	// dispatch wrapper keys
	switch t {
	case ktypeWrapperString:
		tmpv, _ := interfaceToArrayByte(v)
		v = string(tmpv)
	case ktypeWrapperMacAddress:
		tmpv, _ := interfaceToArrayByte(v)
		v = convertMacAddressToString(tmpv)
	case ktypeWrapperIpv4Dns:
		tmpv, _ := interfaceToArrayUint32(v)
		v = wrapIpv4Dns(tmpv)
	case ktypeWrapperIpv4Addresses:
		tmpv, _ := interfaceToArrayArrayUint32(v)
		v = wrapIpv4Addresses(tmpv)
	case ktypeWrapperIpv4Routes:
		tmpv, _ := interfaceToArrayArrayUint32(v)
		v = wrapIpv4Routes(tmpv)
	case ktypeWrapperIpv6Dns:
		tmpv, _ := interfaceToArrayArrayByte(v)
		v = wrapIpv6Dns(tmpv)
	case ktypeWrapperIpv6Addresses:
		tmpv, _ := interfaceToIpv6Addresses(v)
		v = wrapIpv6Addresses(tmpv)
	case ktypeWrapperIpv6Routes:
		tmpv, _ := interfaceToIpv6Routes(v)
		v = wrapIpv6Routes(tmpv)
	}

	jsonStr, err = marshalJSON(v)
	return
}

// json string -> wrapped data(if need) -> realdata -> dbus.Variant.Value()
func jsonToKeyValue(jsonStr string, t ktype) (v interface{}, err error) {
	switch t {
	default:
		err = fmt.Errorf("invalid variant type, %jsonStr", jsonStr)
	case ktypeString:
		v, err = jsonToKeyValueString(jsonStr)
	case ktypeByte:
		v, err = jsonToKeyValueByte(jsonStr)
	case ktypeInt32:
		v, err = jsonToKeyValueInt32(jsonStr)
	case ktypeUint32:
		v, err = jsonToKeyValueUint32(jsonStr)
	case ktypeUint64:
		v, err = jsonToKeyValueUint64(jsonStr)
	case ktypeBoolean:
		v, err = jsonToKeyValueBoolean(jsonStr)
	case ktypeArrayString:
		v, err = jsonToKeyValueArrayString(jsonStr)
	case ktypeArrayByte:
		v, err = jsonToKeyValueArrayByte(jsonStr)
	case ktypeArrayUint32:
		v, err = jsonToKeyValueArrayUint32(jsonStr)
	case ktypeArrayArrayByte:
		v, err = jsonToKeyValueArrayArrayByte(jsonStr)
	case ktypeArrayArrayUint32:
		v, err = jsonToKeyValueArrayArrayUint32(jsonStr)
	case ktypeDictStringString:
		v, err = jsonToKeyValueDictStringString(jsonStr)
	case ktypeIpv6Addresses:
		v, err = jsonToKeyValueIpv6Addresses(jsonStr)
	case ktypeIpv6Routes:
		v, err = jsonToKeyValueIpv6Routes(jsonStr)
	case ktypeWrapperString:
		v, err = jsonToKeyValueWrapperString(jsonStr)
	case ktypeWrapperMacAddress:
		v, err = jsonToKeyValueWrapperMacAddress(jsonStr)
	case ktypeWrapperIpv4Dns:
		v, err = jsonToKeyValueWrapperIpv4Dns(jsonStr)
	case ktypeWrapperIpv4Addresses:
		v, err = jsonToKeyValueWrapperIpv4Addresses(jsonStr)
	case ktypeWrapperIpv4Routes:
		v, err = jsonToKeyValueWrapperIpv4Routes(jsonStr)
	case ktypeWrapperIpv6Dns:
		v, err = jsonToKeyValueWrapperIpv6Dns(jsonStr)
	case ktypeWrapperIpv6Addresses:
		v, err = jsonToKeyValueWrapperIpv6Addresses(jsonStr)
	case ktypeWrapperIpv6Routes:
		v, err = jsonToKeyValueWrapperIpv6Routes(jsonStr)
	}
	return
}

// Convert sepcial key type which wrapped by json to dbus variant'jsonStr value
func jsonToKeyValueString(jsonStr string) (v string, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueByte(jsonStr string) (v byte, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueInt32(jsonStr string) (v int32, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueUint32(jsonStr string) (v uint32, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueUint64(jsonStr string) (v uint64, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueBoolean(jsonStr string) (v bool, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueArrayByte(jsonStr string) (v []byte, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueArrayString(jsonStr string) (v []string, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueArrayUint32(jsonStr string) (v []uint32, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueArrayArrayByte(jsonStr string) (v [][]byte, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueArrayArrayUint32(jsonStr string) (v [][]uint32, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueDictStringString(jsonStr string) (v map[string]string, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueIpv6Addresses(jsonStr string) (v ipv6Addresses, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}
func jsonToKeyValueIpv6Routes(jsonStr string) (v ipv6Routes, err error) {
	err = json.Unmarshal([]byte(jsonStr), &v)
	return
}

// key type wrapper
func jsonToKeyValueWrapperString(jsonStr string) (v []byte, err error) {
	// wrap ktypeArrayByte to [string]
	var wrapData string
	err = json.Unmarshal([]byte(jsonStr), &wrapData)
	if err != nil {
		return
	}
	v = []byte(wrapData)
	return
}
func jsonToKeyValueWrapperMacAddress(jsonStr string) (v []byte, err error) {
	// wrap ktypeArrayByte to [string]
	var wrapData string
	err = json.Unmarshal([]byte(jsonStr), &wrapData)
	if err != nil {
		return
	}
	v, err = convertMacAddressToArrayByteCheck(wrapData)
	return
}
func jsonToKeyValueWrapperIpv4Dns(jsonStr string) (v []uint32, err error) {
	// wrap ktypeArrayUint32 to [array of string]
	var wrapData []string
	err = json.Unmarshal([]byte(jsonStr), &wrapData)
	if err != nil {
		return
	}
	v = unwrapIpv4Dns(wrapData)
	return
}
func jsonToKeyValueWrapperIpv4Addresses(jsonStr string) (v [][]uint32, err error) {
	// wrap ktypeArrayArrayUint32 to [array of (string, uint32, string)]
	var wrapData ipv4AddressesWrapper
	err = json.Unmarshal([]byte(jsonStr), &wrapData)
	if err != nil {
		return
	}
	v = unwrapIpv4Addresses(wrapData)
	return
}
func jsonToKeyValueWrapperIpv4Routes(jsonStr string) (v [][]uint32, err error) {
	// wrap ktypeArrayArrayUint32 to [array of (string, uint32, string, uint32)]
	var wrapData ipv4RoutesWrapper
	err = json.Unmarshal([]byte(jsonStr), &wrapData)
	if err != nil {
		return
	}
	v = unwrapIpv4Routes(wrapData)
	return
}
func jsonToKeyValueWrapperIpv6Dns(jsonStr string) (v [][]byte, err error) {
	// wrap ktypeArrayArrayByte to [array of string]
	var wrapData []string
	err = json.Unmarshal([]byte(jsonStr), &wrapData)
	if err != nil {
		return
	}
	v = unwrapIpv6Dns(wrapData)
	return
}
func jsonToKeyValueWrapperIpv6Addresses(jsonStr string) (v ipv6Addresses, err error) {
	// wrap ktypeIpv6Addresses to [array of (string, uint32, string)]
	var wrapData ipv6AddressesWrapper
	err = json.Unmarshal([]byte(jsonStr), &wrapData)
	if err != nil {
		return
	}
	v = unwrapIpv6Addresses(wrapData)
	return
}
func jsonToKeyValueWrapperIpv6Routes(jsonStr string) (v ipv6Routes, err error) {
	// wrap ktypeIpv6Routes to [array of (string, uint32, string, uint32)]
	var wrapData ipv6RoutesWrapper
	err = json.Unmarshal([]byte(jsonStr), &wrapData)
	if err != nil {
		return
	}
	v = unwrapIpv6Routes(wrapData)
	return
}
