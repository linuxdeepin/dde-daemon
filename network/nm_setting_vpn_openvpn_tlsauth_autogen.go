// This file is automatically generated, please don't edit manully.
package main

import (
	"fmt"
)

// Get key type
func getSettingVpnOpenvpnTlsauthKeyType(key string) (t ktype) {
	switch key {
	default:
		t = ktypeUnknown
	case NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE:
		t = ktypeString
	case NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS:
		t = ktypeString
	case NM_SETTING_VPN_OPENVPN_KEY_TA:
		t = ktypeString
	case NM_SETTING_VPN_OPENVPN_KEY_TA_DIR:
		t = ktypeUint32
	}
	return
}

// Check is key in current setting field
func isKeyInSettingVpnOpenvpnTlsauth(key string) bool {
	switch key {
	case NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE:
		return true
	case NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS:
		return true
	case NM_SETTING_VPN_OPENVPN_KEY_TA:
		return true
	case NM_SETTING_VPN_OPENVPN_KEY_TA_DIR:
		return true
	}
	return false
}

// Get key's default value
func getSettingVpnOpenvpnTlsauthDefaultValue(key string) (value interface{}) {
	switch key {
	default:
		logger.Error("invalid key:", key)
	case NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE:
		value = ""
	case NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS:
		value = ""
	case NM_SETTING_VPN_OPENVPN_KEY_TA:
		value = ""
	case NM_SETTING_VPN_OPENVPN_KEY_TA_DIR:
		value = 0
	}
	return
}

// Get JSON value generally
func generalGetSettingVpnOpenvpnTlsauthKeyJSON(data connectionData, key string) (value string) {
	switch key {
	default:
		logger.Error("generalGetSettingVpnOpenvpnTlsauthKeyJSON: invalide key", key)
	case NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE:
		value = getSettingVpnOpenvpnKeyTlsRemoteJSON(data)
	case NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS:
		value = getSettingVpnOpenvpnKeyRemoteCertTlsJSON(data)
	case NM_SETTING_VPN_OPENVPN_KEY_TA:
		value = getSettingVpnOpenvpnKeyTaJSON(data)
	case NM_SETTING_VPN_OPENVPN_KEY_TA_DIR:
		value = getSettingVpnOpenvpnKeyTaDirJSON(data)
	}
	return
}

// Set JSON value generally
func generalSetSettingVpnOpenvpnTlsauthKeyJSON(data connectionData, key, valueJSON string) (err error) {
	switch key {
	default:
		logger.Error("generalSetSettingVpnOpenvpnTlsauthKeyJSON: invalide key", key)
	case NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE:
		err = setSettingVpnOpenvpnKeyTlsRemoteJSON(data, valueJSON)
	case NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS:
		err = setSettingVpnOpenvpnKeyRemoteCertTlsJSON(data, valueJSON)
	case NM_SETTING_VPN_OPENVPN_KEY_TA:
		err = setSettingVpnOpenvpnKeyTaJSON(data, valueJSON)
	case NM_SETTING_VPN_OPENVPN_KEY_TA_DIR:
		err = setSettingVpnOpenvpnKeyTaDirJSON(data, valueJSON)
	}
	return
}

// Check if key exists
func isSettingVpnOpenvpnKeyTlsRemoteExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE)
}
func isSettingVpnOpenvpnKeyRemoteCertTlsExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS)
}
func isSettingVpnOpenvpnKeyTaExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA)
}
func isSettingVpnOpenvpnKeyTaDirExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA_DIR)
}

// Ensure field and key exists and not empty
func ensureFieldSettingVpnOpenvpnTlsauthExists(data connectionData, errs fieldErrors, relatedKey string) {
	if !isSettingFieldExists(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME) {
		rememberError(errs, relatedKey, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, fmt.Sprintf(NM_KEY_ERROR_MISSING_SECTION, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME))
	}
	fieldData, _ := data[NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME]
	if len(fieldData) == 0 {
		rememberError(errs, relatedKey, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, fmt.Sprintf(NM_KEY_ERROR_EMPTY_SECTION, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME))
	}
}
func ensureSettingVpnOpenvpnKeyTlsRemoteNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingVpnOpenvpnKeyTlsRemoteExists(data) {
		rememberError(errs, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE, NM_KEY_ERROR_MISSING_VALUE)
	}
	value := getSettingVpnOpenvpnKeyTlsRemote(data)
	if len(value) == 0 {
		rememberError(errs, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE, NM_KEY_ERROR_EMPTY_VALUE)
	}
}
func ensureSettingVpnOpenvpnKeyRemoteCertTlsNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingVpnOpenvpnKeyRemoteCertTlsExists(data) {
		rememberError(errs, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS, NM_KEY_ERROR_MISSING_VALUE)
	}
	value := getSettingVpnOpenvpnKeyRemoteCertTls(data)
	if len(value) == 0 {
		rememberError(errs, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS, NM_KEY_ERROR_EMPTY_VALUE)
	}
}
func ensureSettingVpnOpenvpnKeyTaNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingVpnOpenvpnKeyTaExists(data) {
		rememberError(errs, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA, NM_KEY_ERROR_MISSING_VALUE)
	}
	value := getSettingVpnOpenvpnKeyTa(data)
	if len(value) == 0 {
		rememberError(errs, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA, NM_KEY_ERROR_EMPTY_VALUE)
	}
}
func ensureSettingVpnOpenvpnKeyTaDirNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingVpnOpenvpnKeyTaDirExists(data) {
		rememberError(errs, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA_DIR, NM_KEY_ERROR_MISSING_VALUE)
	}
}

// Getter
func getSettingVpnOpenvpnKeyTlsRemote(data connectionData) (value string) {
	ivalue := getSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(string)
		if !ok {
			logger.Warningf("getSettingVpnOpenvpnKeyTlsRemote: value type is invalid, should be string instead of %#v", ivalue)
		}
	}
	return
}
func getSettingVpnOpenvpnKeyRemoteCertTls(data connectionData) (value string) {
	ivalue := getSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(string)
		if !ok {
			logger.Warningf("getSettingVpnOpenvpnKeyRemoteCertTls: value type is invalid, should be string instead of %#v", ivalue)
		}
	}
	return
}
func getSettingVpnOpenvpnKeyTa(data connectionData) (value string) {
	ivalue := getSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(string)
		if !ok {
			logger.Warningf("getSettingVpnOpenvpnKeyTa: value type is invalid, should be string instead of %#v", ivalue)
		}
	}
	return
}
func getSettingVpnOpenvpnKeyTaDir(data connectionData) (value uint32) {
	ivalue := getSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA_DIR)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(uint32)
		if !ok {
			logger.Warningf("getSettingVpnOpenvpnKeyTaDir: value type is invalid, should be uint32 instead of %#v", ivalue)
		}
	}
	return
}

// Setter
func setSettingVpnOpenvpnKeyTlsRemote(data connectionData, value string) {
	setSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE, value)
}
func setSettingVpnOpenvpnKeyRemoteCertTls(data connectionData, value string) {
	setSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS, value)
}
func setSettingVpnOpenvpnKeyTa(data connectionData, value string) {
	setSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA, value)
}
func setSettingVpnOpenvpnKeyTaDir(data connectionData, value uint32) {
	setSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA_DIR, value)
}

// JSON Getter
func getSettingVpnOpenvpnKeyTlsRemoteJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE, getSettingVpnOpenvpnTlsauthKeyType(NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE))
	return
}
func getSettingVpnOpenvpnKeyRemoteCertTlsJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS, getSettingVpnOpenvpnTlsauthKeyType(NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS))
	return
}
func getSettingVpnOpenvpnKeyTaJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA, getSettingVpnOpenvpnTlsauthKeyType(NM_SETTING_VPN_OPENVPN_KEY_TA))
	return
}
func getSettingVpnOpenvpnKeyTaDirJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA_DIR, getSettingVpnOpenvpnTlsauthKeyType(NM_SETTING_VPN_OPENVPN_KEY_TA_DIR))
	return
}

// JSON Setter
func setSettingVpnOpenvpnKeyTlsRemoteJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE, valueJSON, getSettingVpnOpenvpnTlsauthKeyType(NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE))
}
func setSettingVpnOpenvpnKeyRemoteCertTlsJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS, valueJSON, getSettingVpnOpenvpnTlsauthKeyType(NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS))
}
func setSettingVpnOpenvpnKeyTaJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA, valueJSON, getSettingVpnOpenvpnTlsauthKeyType(NM_SETTING_VPN_OPENVPN_KEY_TA))
}
func setSettingVpnOpenvpnKeyTaDirJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA_DIR, valueJSON, getSettingVpnOpenvpnTlsauthKeyType(NM_SETTING_VPN_OPENVPN_KEY_TA_DIR))
}

// Logic JSON Setter

// Remover
func removeSettingVpnOpenvpnKeyTlsRemote(data connectionData) {
	removeSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TLS_REMOTE)
}
func removeSettingVpnOpenvpnKeyRemoteCertTls(data connectionData) {
	removeSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_REMOTE_CERT_TLS)
}
func removeSettingVpnOpenvpnKeyTa(data connectionData) {
	removeSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA)
}
func removeSettingVpnOpenvpnKeyTaDir(data connectionData) {
	removeSettingKey(data, NM_SETTING_VF_VPN_OPENVPN_TLSAUTH_SETTING_NAME, NM_SETTING_VPN_OPENVPN_KEY_TA_DIR)
}
